/*
Copyright 2026 Metacontroller authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package composite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/finalizer"
	"metacontroller/pkg/internal/testutils/cleanuprecord"
	. "metacontroller/pkg/internal/testutils/common"
	"metacontroller/pkg/internal/testutils/dynamic/clientset"
)

// The reconcile tests use two resources of TestAPIVersion: the retired
// resource TestResource and the current resource "others".
const (
	otherResource  = "others"
	otherKind      = "Other"
	controllerName = "cc"
)

var (
	retiredGVR = schema.GroupVersionResource{Group: TestGroup, Version: TestVersion, Resource: TestResource}
	parentFin  = parentFinalizerName(controllerName)
)

// stubController replaces parentController. It starts no informers.
type stubController struct {
	cc      *v1alpha1.CompositeController
	started bool
	stopped bool
}

func (s *stubController) Start() { s.started = true }
func (s *stubController) Stop()  { s.stopped = true }
func (s *stubController) object() *v1alpha1.CompositeController {
	return s.cc
}

type reconcileFixture struct {
	mc         *Metacontroller
	k8sClient  client.Client
	dynFake    *dynamicfake.FakeDynamicClient
	started    []*stubController
	factoryErr error
}

func newCompositeController(parentResource string, finalizeHook bool, record string) *v1alpha1.CompositeController {
	cc := &v1alpha1.CompositeController{
		ObjectMeta: metav1.ObjectMeta{Name: controllerName, Finalizers: []string{finalizer.ControllerFinalizer}},
		Spec: v1alpha1.CompositeControllerSpec{
			ParentResource: v1alpha1.CompositeControllerParentResourceRule{
				ResourceRule: v1alpha1.ResourceRule{APIVersion: TestAPIVersion, Resource: parentResource},
			},
		},
	}
	if finalizeHook {
		cc.Spec.Hooks = &v1alpha1.CompositeControllerHooks{Finalize: &v1alpha1.Hook{}}
	}
	if record != "" {
		cc.Annotations = map[string]string{finalizer.CleanupRecordAnnotation: record}
	}
	return cc
}

func parentCRD(resource string, statusSubresource bool) *apiextensionsv1.CustomResourceDefinition {
	version := apiextensionsv1.CustomResourceDefinitionVersion{Name: TestVersion, Served: true, Storage: true}
	if statusSubresource {
		version.Subresources = &apiextensionsv1.CustomResourceSubresources{Status: &apiextensionsv1.CustomResourceSubresourceStatus{}}
	}
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: resource + "." + TestGroup},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group:    TestGroup,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{version},
		},
	}
}

func retiredParent(name string) *unstructured.Unstructured {
	return cleanuprecord.NewParent(TestKind, name, parentFin)
}

func newReconcileFixture(t *testing.T, cc *v1alpha1.CompositeController, crd *apiextensionsv1.CustomResourceDefinition, parents ...*unstructured.Unstructured) *reconcileFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(cc, crd).Build()
	dynClient, dynFake, resources := clientset.NewFakeClients(map[string]string{TestResource: TestKind, otherResource: otherKind}, parents...)

	f := &reconcileFixture{k8sClient: k8sClient, dynFake: dynFake}
	f.mc = &Metacontroller{
		k8sClient:     k8sClient,
		resources:     resources,
		dynClient:     dynClient,
		eventRecorder: record.NewFakeRecorder(100),
		cleanupBudget: time.Minute,
		logger:        logr.Discard(),
		ctx:           context.TODO(),
	}
	f.mc.newController = func(cc *v1alpha1.CompositeController) (runningController, error) {
		if f.factoryErr != nil {
			return nil, f.factoryErr
		}
		stub := &stubController{cc: cc.DeepCopy()}
		f.started = append(f.started, stub)
		return stub, nil
	}
	return f
}

// run stores a stub controller with the given spec.
func (f *reconcileFixture) run(cc *v1alpha1.CompositeController) *stubController {
	stub := &stubController{cc: cc.DeepCopy(), started: true}
	f.mc.parentControllers.Store(cc.Name, stub)
	return stub
}

func (f *reconcileFixture) reconcile(t *testing.T) (reconcile.Result, error) {
	t.Helper()
	return f.mc.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerName}})
}

func (f *reconcileFixture) controller(t *testing.T) *v1alpha1.CompositeController {
	t.Helper()
	live := &v1alpha1.CompositeController{}
	require.NoError(t, f.k8sClient.Get(context.TODO(), client.ObjectKey{Name: controllerName}, live))
	return live
}

func (f *reconcileFixture) record(t *testing.T) cleanuprecord.State {
	t.Helper()
	return cleanuprecord.Read(t, f.controller(t))
}

func (f *reconcileFixture) retired(t *testing.T) []string {
	t.Helper()
	return cleanuprecord.Resources(f.record(t).RemoveParentFinalizerFrom)
}

// conflictOnPatch makes each patch of the controller object fail with a
// conflict.
func (f *reconcileFixture) conflictOnPatch() {
	f.mc.k8sClient = interceptor.NewClient(f.k8sClient.(client.WithWatch), interceptor.Funcs{
		Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			return apierrors.NewConflict(schema.GroupResource{Group: "metacontroller.k8s.io"}, controllerName, errors.New("changed"))
		},
	})
}

func (f *reconcileFixture) cleanCount(t *testing.T, gvr schema.GroupVersionResource, names ...string) int {
	t.Helper()
	return cleanuprecord.CleanCount(t, f.dynFake, gvr, names...)
}

func TestReconcile_whenCleanupNeedsTwoSlices_requeuesAndContinuesWithUnchangedSpec(t *testing.T) {
	cc := newCompositeController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, cc, parentCRD(otherResource, true), retiredParent("a"), retiredParent("b"))
	f.mc.cleanupBudget = -time.Second

	result, err := f.reconcile(t)

	require.NoError(t, err)
	assert.Equal(t, time.Second, result.RequeueAfter)
	assert.Equal(t, []string{TestResource}, f.retired(t))

	result, err = f.reconcile(t)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
	assert.Equal(t, 2, f.cleanCount(t, retiredGVR, "a", "b"))
	assert.Empty(t, f.retired(t))
	// The spec is unchanged, so the controller started once.
	assert.Len(t, f.started, 1)
}

func TestReconcile_whenNewControllerFails_stillCleansRetired(t *testing.T) {
	cc := newCompositeController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, cc, parentCRD(otherResource, true), retiredParent("a"))
	f.factoryErr = errors.New("no informer")

	_, err := f.reconcile(t)

	require.ErrorContains(t, err, "no informer")
	assert.Equal(t, 1, f.cleanCount(t, retiredGVR, "a"))
}

func TestReconcile_whenStartFailsDuringCleanup_requeuesWithoutError(t *testing.T) {
	cc := newCompositeController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, cc, parentCRD(otherResource, true), retiredParent("a"), retiredParent("b"))
	f.mc.cleanupBudget = -time.Second
	f.factoryErr = errors.New("no informer")

	result, err := f.reconcile(t)

	// A returned error makes the queue ignore RequeueAfter.
	require.NoError(t, err)
	assert.Positive(t, result.RequeueAfter)
	cleanuprecord.RequireEvent(t, f.mc.eventRecorder, "no informer")
}

func TestReconcile_whenParentCRDIsMissingDuringCleanup_requeuesWithoutError(t *testing.T) {
	cc := newCompositeController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, cc, parentCRD("unrelated", true), retiredParent("a"), retiredParent("b"))
	f.mc.cleanupBudget = -time.Second

	result, err := f.reconcile(t)

	require.NoError(t, err)
	assert.Positive(t, result.RequeueAfter)
	cleanuprecord.RequireEvent(t, f.mc.eventRecorder, "not found")
	assert.Empty(t, f.started)
}

func TestReconcile_whenFinalizeHookRemoved_stillCleansRetired(t *testing.T) {
	old := newCompositeController(TestResource, true, cleanuprecord.ForTestResources(t, TestResource))
	cc := newCompositeController(otherResource, false, cleanuprecord.ForTestResources(t, TestResource))
	f := newReconcileFixture(t, cc, parentCRD(otherResource, true), retiredParent("a"))
	running := f.run(old)

	_, err := f.reconcile(t)
	require.NoError(t, err)
	assert.True(t, running.stopped)
	assert.Equal(t, []string{TestResource}, f.retired(t))

	_, err = f.reconcile(t)

	require.NoError(t, err)
	assert.Equal(t, 1, f.cleanCount(t, retiredGVR, "a"))
	assert.Empty(t, f.retired(t))
}

func TestReconcile_whenSpecCrdLacksStatusSubresource_addsFinalizerAndContinuesCleanup(t *testing.T) {
	cc := newCompositeController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	cc.Finalizers = nil
	f := newReconcileFixture(t, cc, parentCRD(otherResource, false), retiredParent("a"), retiredParent("b"))
	f.mc.cleanupBudget = -time.Second

	result, err := f.reconcile(t)

	require.NoError(t, err)
	assert.Positive(t, result.RequeueAfter)
	assert.Contains(t, f.controller(t).Finalizers, finalizer.ControllerFinalizer)
	assert.Equal(t, 1, f.cleanCount(t, retiredGVR, "a", "b"))
	assert.Empty(t, f.started)
}

func TestReconcile_whenControllerFinalizerWithoutRecord_writesActive(t *testing.T) {
	f := newReconcileFixture(t, newCompositeController(otherResource, true, ""), parentCRD(otherResource, true))

	_, err := f.reconcile(t)

	require.NoError(t, err)
	assert.Equal(t, []string{otherResource}, cleanuprecord.Resources(f.record(t).Active))
	assert.Empty(t, f.retired(t))
	assert.Len(t, f.started, 1)
}

func TestReconcile_whenRecordRemovedAndSpecUnchanged_rewritesRecord(t *testing.T) {
	for name, annotations := range map[string]map[string]string{
		"absent": nil,
		"empty":  {finalizer.CleanupRecordAnnotation: ""},
	} {
		t.Run(name, func(t *testing.T) {
			cc := newCompositeController(otherResource, true, "")
			cc.Annotations = annotations
			f := newReconcileFixture(t, cc, parentCRD(otherResource, true))
			f.run(cc)

			_, err := f.reconcile(t)

			require.NoError(t, err)
			assert.Equal(t, []string{otherResource}, cleanuprecord.Resources(f.record(t).Active))
			assert.Empty(t, f.started)
		})
	}
}

func TestReconcile_whenCleanupFails_startsControllerAndRequeuesAfterErrorDelay(t *testing.T) {
	cc := newCompositeController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, cc, parentCRD(otherResource, true))
	f.dynFake.PrependReactor("list", "*", func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list refused")
	})

	result, err := f.reconcile(t)

	// The error is in an event. A returned error makes the queue use its own
	// rate limiter.
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)
	assert.Len(t, f.started, 1)
	assert.Equal(t, []string{TestResource}, f.retired(t))
}

func TestReconcile_whenRecordMalformedAndNoController_startsCurrentSpec(t *testing.T) {
	cc := newCompositeController(otherResource, true, "{not json")
	f := newReconcileFixture(t, cc, parentCRD(otherResource, true))

	_, err := f.reconcile(t)

	require.ErrorIs(t, err, finalizer.ErrInvalidCleanupRecord)
	assert.Len(t, f.started, 1)
	assert.Equal(t, "{not json", f.controller(t).Annotations[finalizer.CleanupRecordAnnotation])
}

func TestReconcile_whenRecordWriteFailsAndSpecChanged_keepsOldController(t *testing.T) {
	for name, tc := range map[string]struct {
		record   string
		conflict bool
	}{
		"malformed record": {record: "{not json"},
		"write conflict":   {record: cleanuprecord.ForTestResources(t, TestResource), conflict: true},
	} {
		t.Run(name, func(t *testing.T) {
			old := newCompositeController(TestResource, true, tc.record)
			f := newReconcileFixture(t, newCompositeController(otherResource, true, tc.record), parentCRD(otherResource, true))
			running := f.run(old)
			if tc.conflict {
				f.conflictOnPatch()
			}

			_, err := f.reconcile(t)

			require.Error(t, err)
			assert.False(t, running.stopped)
			assert.Empty(t, f.started)
			assert.Equal(t, tc.record, f.controller(t).Annotations[finalizer.CleanupRecordAnnotation])
		})
	}
}

func TestReconcile_whenRunningResourceIsRetired_doesNotCleanIt(t *testing.T) {
	// The new spec stops at a gate, so the old controller still runs on
	// TestResource.
	old := newCompositeController(TestResource, true, "")
	cc := newCompositeController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, cc, parentCRD(otherResource, false), retiredParent("a"))
	f.run(old)

	_, err := f.reconcile(t)

	require.NoError(t, err)
	assert.Equal(t, 0, f.cleanCount(t, retiredGVR, "a"))
	assert.Equal(t, []string{TestResource}, f.retired(t))
}
