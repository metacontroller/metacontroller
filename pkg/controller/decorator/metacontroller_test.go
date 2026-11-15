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

package decorator

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
	controllerName = "dc"
)

var (
	retiredGVR = schema.GroupVersionResource{Group: TestGroup, Version: TestVersion, Resource: TestResource}
	parentFin  = parentFinalizerName(controllerName)
)

// stubController replaces decoratorController. It starts no informers.
type stubController struct {
	dc      *v1alpha1.DecoratorController
	started bool
	stopped bool
}

func (s *stubController) Start() { s.started = true }
func (s *stubController) Stop()  { s.stopped = true }
func (s *stubController) object() *v1alpha1.DecoratorController {
	return s.dc
}

type reconcileFixture struct {
	mc         *Metacontroller
	k8sClient  client.Client
	dynFake    *dynamicfake.FakeDynamicClient
	started    []*stubController
	factoryErr error
}

func newTestDecoratorController(resource string, finalizeHook bool, record string) *v1alpha1.DecoratorController {
	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{Name: controllerName, Finalizers: []string{finalizer.ControllerFinalizer}},
		Spec: v1alpha1.DecoratorControllerSpec{
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: v1alpha1.ResourceRule{APIVersion: TestAPIVersion, Resource: resource}},
			},
		},
	}
	if finalizeHook {
		dc.Spec.Hooks = &v1alpha1.DecoratorControllerHooks{Finalize: &v1alpha1.Hook{}}
	}
	if record != "" {
		dc.Annotations = map[string]string{finalizer.CleanupRecordAnnotation: record}
	}
	return dc
}

func retiredParent(name string) *unstructured.Unstructured {
	return cleanuprecord.NewParent(TestKind, name, parentFin)
}

func newReconcileFixture(t *testing.T, dc *v1alpha1.DecoratorController, parents ...*unstructured.Unstructured) *reconcileFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	k8sClient := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(dc).Build()
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
	f.mc.newController = func(dc *v1alpha1.DecoratorController) (runningController, error) {
		if f.factoryErr != nil {
			return nil, f.factoryErr
		}
		stub := &stubController{dc: dc.DeepCopy()}
		f.started = append(f.started, stub)
		return stub, nil
	}
	return f
}

// run stores a stub controller with the given spec.
func (f *reconcileFixture) run(dc *v1alpha1.DecoratorController) *stubController {
	stub := &stubController{dc: dc.DeepCopy(), started: true}
	f.mc.decoratorControllers.Store(dc.Name, stub)
	return stub
}

func (f *reconcileFixture) reconcile(t *testing.T) (reconcile.Result, error) {
	t.Helper()
	return f.mc.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerName}})
}

func (f *reconcileFixture) controller(t *testing.T) *v1alpha1.DecoratorController {
	t.Helper()
	live := &v1alpha1.DecoratorController{}
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
	dc := newTestDecoratorController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, dc, retiredParent("a"), retiredParent("b"))
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
	dc := newTestDecoratorController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, dc, retiredParent("a"))
	f.factoryErr = errors.New("no informer")

	_, err := f.reconcile(t)

	require.ErrorContains(t, err, "no informer")
	assert.Equal(t, 1, f.cleanCount(t, retiredGVR, "a"))
}

func TestReconcile_whenStartFailsDuringCleanup_requeuesWithoutError(t *testing.T) {
	dc := newTestDecoratorController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, dc, retiredParent("a"), retiredParent("b"))
	f.mc.cleanupBudget = -time.Second
	f.factoryErr = errors.New("no informer")

	result, err := f.reconcile(t)

	// A returned error makes the queue ignore RequeueAfter.
	require.NoError(t, err)
	assert.Positive(t, result.RequeueAfter)
	cleanuprecord.RequireEvent(t, f.mc.eventRecorder, "no informer")
}

func TestReconcile_whenFinalizeHookRemoved_stillCleansRetired(t *testing.T) {
	old := newTestDecoratorController(TestResource, true, cleanuprecord.ForTestResources(t, TestResource))
	dc := newTestDecoratorController(otherResource, false, cleanuprecord.ForTestResources(t, TestResource))
	f := newReconcileFixture(t, dc, retiredParent("a"))
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

func TestReconcile_whenControllerFinalizerWithoutRecord_writesActive(t *testing.T) {
	f := newReconcileFixture(t, newTestDecoratorController(otherResource, true, ""))

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
			dc := newTestDecoratorController(otherResource, true, "")
			dc.Annotations = annotations
			f := newReconcileFixture(t, dc)
			f.run(dc)

			_, err := f.reconcile(t)

			require.NoError(t, err)
			assert.Equal(t, []string{otherResource}, cleanuprecord.Resources(f.record(t).Active))
			assert.Empty(t, f.started)
		})
	}
}

func TestReconcile_whenCleanupFails_startsControllerAndRequeuesAfterErrorDelay(t *testing.T) {
	dc := newTestDecoratorController(otherResource, true, cleanuprecord.ForTestResources(t, otherResource, TestResource))
	f := newReconcileFixture(t, dc)
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
	dc := newTestDecoratorController(otherResource, true, "{not json")
	f := newReconcileFixture(t, dc)

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
			old := newTestDecoratorController(TestResource, true, tc.record)
			f := newReconcileFixture(t, newTestDecoratorController(otherResource, true, tc.record))
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
	// The record write fails, so the old controller still runs on
	// TestResource.
	old := newTestDecoratorController(TestResource, true, "")
	dc := newTestDecoratorController(otherResource, true, cleanuprecord.ForTestResources(t, "", TestResource))
	f := newReconcileFixture(t, dc, retiredParent("a"))
	f.run(old)
	f.conflictOnPatch()

	_, err := f.reconcile(t)

	require.Error(t, err)
	assert.Equal(t, 0, f.cleanCount(t, retiredGVR, "a"))
}
