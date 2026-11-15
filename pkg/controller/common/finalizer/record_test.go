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

package finalizer_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/finalizer"
	"metacontroller/pkg/internal/testutils/cleanuprecord"
	. "metacontroller/pkg/internal/testutils/common"
	"metacontroller/pkg/internal/testutils/dynamic/clientset"
)

var (
	// testResourceA is in discovery and has objects in the fake API server.
	testResourceA = finalizer.ParentResource{APIVersion: TestAPIVersion, Resource: TestResource}
	testResourceB = finalizer.ParentResource{APIVersion: TestAPIVersion, Resource: "widgets"}
	testResourceC = finalizer.ParentResource{APIVersion: TestAPIVersion, Resource: "gadgets"}
)

func entry(p finalizer.ParentResource) cleanuprecord.Entry {
	return cleanuprecord.Entry{APIVersion: p.APIVersion, Resource: p.Resource}
}

// retiredRecord returns a record whose removeParentFinalizerFrom list has the
// given resources.
func retiredRecord(t *testing.T, retired ...finalizer.ParentResource) string {
	t.Helper()
	state := cleanuprecord.State{}
	for _, p := range retired {
		state.RemoveParentFinalizerFrom = append(state.RemoveParentFinalizerFrom, entry(p))
	}
	return cleanuprecord.JSON(t, state)
}

// newRecordController returns a controller object with ControllerFinalizer
// and, when record is not empty, the cleanup record.
func newRecordController(record string) *v1alpha1.DecoratorController {
	dc := &v1alpha1.DecoratorController{ObjectMeta: metav1.ObjectMeta{Name: "dc", Finalizers: []string{finalizer.ControllerFinalizer}}}
	if record != "" {
		dc.Annotations = map[string]string{finalizer.CleanupRecordAnnotation: record}
	}
	return dc
}

func liveController(t *testing.T, k8sClient client.Client, dc *v1alpha1.DecoratorController) *v1alpha1.DecoratorController {
	t.Helper()
	live := &v1alpha1.DecoratorController{}
	require.NoError(t, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(dc), live))
	return live
}

func liveRecord(t *testing.T, k8sClient client.Client, dc *v1alpha1.DecoratorController) cleanuprecord.State {
	t.Helper()
	return cleanuprecord.Read(t, liveController(t, k8sClient, dc))
}

func newInterceptedControllerClient(t *testing.T, funcs interceptor.Funcs, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	return clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(funcs).Build()
}

func TestRecordActiveParents(t *testing.T) {
	staleVersion := finalizer.ParentResource{APIVersion: TestGroup + "/old", Resource: TestResource}
	A, B, C := testResourceA, testResourceB, testResourceC
	for name, tc := range map[string]struct {
		running     []finalizer.ParentResource
		specs       [][]finalizer.ParentResource
		wantActive  []string
		wantRetired []string
	}{
		"first record":                     {specs: [][]finalizer.ParentResource{{A}}, wantActive: []string{TestResource}},
		"version change does not retire":   {specs: [][]finalizer.ParentResource{{staleVersion}, {A}}, wantActive: []string{TestResource}},
		"spec changes keep all retired":    {specs: [][]finalizer.ParentResource{{A}, {B}, {C}}, wantActive: []string{"gadgets"}, wantRetired: []string{TestResource, "widgets"}},
		"returned resource is not retired": {specs: [][]finalizer.ParentResource{{A}, {B}, {A, B}}, wantActive: []string{TestResource, "widgets"}},
		"no record, controller runs":       {running: []finalizer.ParentResource{A}, specs: [][]finalizer.ParentResource{{B}}, wantActive: []string{"widgets"}, wantRetired: []string{TestResource}},
	} {
		t.Run(name, func(t *testing.T) {
			dc := newRecordController("")
			k8sClient := newControllerClient(t, dc)
			cleaner := newCleaner(k8sClient, nil, nil)

			running := tc.running
			for _, spec := range tc.specs {
				require.NoError(t, cleaner.RecordActiveParents(context.TODO(), dc, running, spec))
				running = spec
			}

			state := liveRecord(t, k8sClient, dc)
			assert.Equal(t, tc.wantActive, cleanuprecord.Resources(state.Active))
			assert.Equal(t, tc.wantRetired, cleanuprecord.Resources(state.RemoveParentFinalizerFrom))
		})
	}
}

func TestRecordActiveParents_whenRecordOmitsRunningResource_retiresIt(t *testing.T) {
	dc := newRecordController("")
	k8sClient := newControllerClient(t, dc)
	cleaner := newCleaner(k8sClient, nil, nil)
	require.NoError(t, cleaner.RecordActiveParents(context.TODO(), dc, nil, []finalizer.ParentResource{testResourceB}))

	// The record lists B, but the running controller uses A.
	require.NoError(t, cleaner.RecordActiveParents(context.TODO(), dc, []finalizer.ParentResource{testResourceA}, []finalizer.ParentResource{testResourceC}))

	state := liveRecord(t, k8sClient, dc)
	assert.Equal(t, []string{TestResource, "widgets"}, cleanuprecord.Resources(state.RemoveParentFinalizerFrom))
}

func TestRecordActiveParents_whenRecordCannotBeWritten_returnsErrorAndKeepsValue(t *testing.T) {
	for name, tc := range map[string]struct {
		annotations map[string]string
		invalid     bool
	}{
		"malformed": {annotations: map[string]string{finalizer.CleanupRecordAnnotation: "{not json"}, invalid: true},
		"too large": {annotations: map[string]string{"example.com/large": strings.Repeat("x", 256*1024-100)}},
	} {
		t.Run(name, func(t *testing.T) {
			dc := newRecordController("")
			dc.Annotations = tc.annotations
			old := tc.annotations[finalizer.CleanupRecordAnnotation]
			k8sClient := newControllerClient(t, dc)

			err := newCleaner(k8sClient, nil, nil).RecordActiveParents(context.TODO(), dc, nil, []finalizer.ParentResource{testResourceA})

			require.Error(t, err)
			assert.Equal(t, tc.invalid, errors.Is(err, finalizer.ErrInvalidCleanupRecord))
			assert.Equal(t, old, liveController(t, k8sClient, dc).Annotations[finalizer.CleanupRecordAnnotation])
		})
	}
}

func TestRecordActiveParents_whenValueIsUnchanged_writesNothing(t *testing.T) {
	dc := newRecordController("")
	patches := 0
	k8sClient := newInterceptedControllerClient(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			patches++
			return c.Patch(ctx, obj, patch, opts...)
		},
	}, dc)
	cleaner := newCleaner(k8sClient, nil, nil)

	require.NoError(t, cleaner.RecordActiveParents(context.TODO(), dc, nil, []finalizer.ParentResource{testResourceA}))
	require.NoError(t, cleaner.RecordActiveParents(context.TODO(), dc, nil, []finalizer.ParentResource{testResourceA}))

	assert.Equal(t, 1, patches)
}

func TestCleanupRetiredParents_whenBudgetIsShort_requeuesUntilDone(t *testing.T) {
	dc := newRecordController(retiredRecord(t, testResourceA))
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer), newParent("b", parentFinalizer))
	cleaner := withBudget(newCleaner(k8sClient, dynClient, resources), -time.Second)

	result, err := cleaner.CleanupRetiredParents(context.TODO(), dc, parentFinalizer, nil)

	require.NoError(t, err)
	assert.Equal(t, time.Second, result.RequeueAfter)
	assert.Len(t, liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom, 1)

	result, err = cleaner.CleanupRetiredParents(context.TODO(), dc, parentFinalizer, nil)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
	assert.Empty(t, liveFinalizers(t, fakeClient, "a"))
	assert.Empty(t, liveFinalizers(t, fakeClient, "b"))
	assert.Empty(t, liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom)
}

func TestCleanupRetiredParents_whenOneEntryFails_cleansTheOthersAndRequeuesWithoutError(t *testing.T) {
	ghosts := finalizer.ParentResource{APIVersion: TestAPIVersion, Resource: "ghosts"}
	dc := newRecordController(retiredRecord(t, ghosts, testResourceA))
	k8sClient := newControllerClient(t, dc, newCRD("ghosts", TestGroup))
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))
	cleaner := withBudget(newCleaner(k8sClient, dynClient, resources), largeBudget)

	result, err := cleaner.CleanupRetiredParents(context.TODO(), dc, parentFinalizer, nil)

	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)
	requireEvent(t, cleaner, "discovery lists no version")
	assert.Empty(t, liveFinalizers(t, fakeClient, "a"))
	assert.Equal(t, []string{"ghosts"}, cleanuprecord.Resources(liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom))
}

func TestCleanupRetiredParents_whenOneResourceKeepsFailing_removesTheCompletedOne(t *testing.T) {
	dc := newRecordController(retiredRecord(t, testResourceA, testResourceOthers))
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := clientset.NewFakeClients(testAndOthers, newParent("a", parentFinalizer), newOther("o", parentFinalizer))
	fakeClient.PrependReactor("update", TestResource, func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update refused")
	})
	cleaner := withBudget(newCleaner(k8sClient, dynClient, resources), largeBudget)

	result, err := cleaner.CleanupRetiredParents(context.TODO(), dc, parentFinalizer, nil)

	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)
	assert.Equal(t, []string{TestResource}, cleanuprecord.Resources(liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom))
}

func TestCleanupRetiredParents_whenResourceIsNotInDiscoveryAndHasNoCRD_dropsEntryWithWarning(t *testing.T) {
	typo := finalizer.ParentResource{APIVersion: "exmple.com/v1", Resource: "things"}
	dc := newRecordController(retiredRecord(t, typo))
	k8sClient := newControllerClient(t, dc)
	dynClient, _, resources := newCleanupClientsAndResources()
	cleaner := newCleaner(k8sClient, dynClient, resources)

	result, err := cleaner.CleanupRetiredParents(context.TODO(), dc, parentFinalizer, nil)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
	requireEvent(t, cleaner, "remove the finalizer by hand")
	assert.Empty(t, liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom)
}

func TestCleanupRetiredParents_whenRecordWriteConflicts_keepsEntry(t *testing.T) {
	dc := newRecordController(retiredRecord(t, testResourceA))
	conflicts := 0
	k8sClient := newInterceptedControllerClient(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if conflicts == 0 {
				conflicts++
				return apierrors.NewConflict(schema.GroupResource{Group: "metacontroller.k8s.io", Resource: "decoratorcontrollers"}, "dc", errors.New("changed"))
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	}, dc)
	dynClient, _, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))
	cleaner := withBudget(newCleaner(k8sClient, dynClient, resources), largeBudget)

	result, err := cleaner.CleanupRetiredParents(context.TODO(), dc, parentFinalizer, nil)

	require.NoError(t, err)
	assert.Positive(t, result.RequeueAfter)
	assert.Equal(t, []string{TestResource}, cleanuprecord.Resources(liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom))

	// The next reconcile reads the controller object again.
	_, err = cleaner.CleanupRetiredParents(context.TODO(), liveController(t, k8sClient, dc), parentFinalizer, nil)

	require.NoError(t, err)
	assert.Empty(t, liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom)
}

func TestCleanupRetiredParents_whenEntryIsExcluded_keepsIt(t *testing.T) {
	// The entry has another apiVersion than the excluded resource. The
	// cleanup compares by GroupResource.
	staleVersion := finalizer.ParentResource{APIVersion: TestGroup + "/old", Resource: TestResource}
	dc := newRecordController(retiredRecord(t, staleVersion, testResourceOthers))
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := clientset.NewFakeClients(testAndOthers, newParent("a", parentFinalizer), newOther("o", parentFinalizer))
	cleaner := withBudget(newCleaner(k8sClient, dynClient, resources), largeBudget)

	_, err := cleaner.CleanupRetiredParents(context.TODO(), dc, parentFinalizer, []finalizer.ParentResource{testResourceA})

	require.NoError(t, err)
	assert.Equal(t, []string{parentFinalizer}, liveFinalizers(t, fakeClient, "a"))
	assert.Equal(t, []string{TestResource}, cleanuprecord.Resources(liveRecord(t, k8sClient, dc).RemoveParentFinalizerFrom))
}

func TestCleanupRetiredParents_whenRecordIsMalformed_returnsErrorAndCleansNothing(t *testing.T) {
	dc := newRecordController(`{"version":2}`)
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))

	_, err := newCleaner(k8sClient, dynClient, resources).CleanupRetiredParents(context.TODO(), dc, parentFinalizer, nil)

	require.ErrorIs(t, err, finalizer.ErrInvalidCleanupRecord)
	assert.Zero(t, countUpdates(fakeClient))
}

func TestFinalize_whenRecordHasRetiredResources_cleansThemWithEmptyParents(t *testing.T) {
	dc := newRecordController(retiredRecord(t, testResourceA))
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, nil, noStop)

	require.NoError(t, err)
	assert.Empty(t, liveFinalizers(t, fakeClient, "a"))
	live := liveController(t, k8sClient, dc)
	assert.Empty(t, live.Finalizers)
	assert.NotContains(t, live.Annotations, finalizer.CleanupRecordAnnotation)
}

func TestFinalize_whenRecordIsMalformed_failsClosedAndKeepsFinalizer(t *testing.T) {
	dc := newRecordController("{not json")
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, testParents, noStop)

	require.ErrorIs(t, err, finalizer.ErrInvalidCleanupRecord)
	assert.Equal(t, []string{finalizer.ControllerFinalizer}, controllerFinalizers(t, k8sClient, dc))
	assert.Zero(t, countUpdates(fakeClient))
}
