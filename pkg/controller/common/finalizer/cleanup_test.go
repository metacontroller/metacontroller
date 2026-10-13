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
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/finalizer"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	. "metacontroller/pkg/internal/testutils/common"
	. "metacontroller/pkg/internal/testutils/dynamic/clientset"
	. "metacontroller/pkg/internal/testutils/dynamic/discovery"
)

const (
	parentFinalizer = "metacontroller.io/decoratorcontroller-test"
	otherFinalizer  = "example.com/other"
)

var testGVR = schema.GroupVersionResource{Group: TestGroup, Version: TestVersion, Resource: TestResource}

func newParent(name string, finalizers ...string) *unstructured.Unstructured {
	parent := NewUnstructured(TestAPIVersion, TestKind, TestNamespace, name)
	parent.SetFinalizers(finalizers)
	return parent
}

// newCleanupClients returns a clientset whose fake API server has the given
// parent objects.
func newCleanupClients(parents ...*unstructured.Unstructured) (*dynamicclientset.Clientset, *dynamicfake.FakeDynamicClient) {
	dynClient, fakeClient, _ := newCleanupClientsAndResources(parents...)
	return dynClient, fakeClient
}

func newCleanupClientsAndResources(parents ...*unstructured.Unstructured) (*dynamicclientset.Clientset, *dynamicfake.FakeDynamicClient, *dynamicdiscovery.ResourceMap) {
	objects := make([]runtime.Object, 0, len(parents))
	for _, parent := range parents {
		objects = append(objects, parent)
	}
	gvrToListKind := map[schema.GroupVersionResource]string{testGVR: TestResourceList}
	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objects...)
	resourceMap := NewFakeResourceMap(NewFakeClientsetWithResources(NewDefaultAPIResourceList()))
	return NewClientset(NewDefaultRestConfig(), resourceMap, fakeClient), fakeClient, resourceMap
}

func liveFinalizers(t *testing.T, fakeClient *dynamicfake.FakeDynamicClient, name string) []string {
	t.Helper()
	parent, err := fakeClient.Resource(testGVR).Namespace(TestNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return parent.GetFinalizers()
}

func countUpdates(fakeClient *dynamicfake.FakeDynamicClient) int {
	count := 0
	for _, action := range fakeClient.Actions() {
		if action.GetVerb() == "update" {
			count++
		}
	}
	return count
}

var testParents = []finalizer.ParentResource{{APIVersion: TestAPIVersion, Resource: TestResource}}

const largeBudget = time.Minute

func TestRemoveFromResources_whenParentsHaveFinalizer_removesOnlyThatFinalizer(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(
		newParent("a", otherFinalizer, parentFinalizer),
		newParent("b", parentFinalizer),
		newParent("c", otherFinalizer),
	)
	manager := finalizer.NewManager(parentFinalizer, true)

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, testParents, largeBudget)

	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, 2, removed)
	assert.Equal(t, 2, countUpdates(fakeClient))
	assert.Equal(t, []string{otherFinalizer}, liveFinalizers(t, fakeClient, "a"))
	assert.Empty(t, liveFinalizers(t, fakeClient, "b"))
	assert.Equal(t, []string{otherFinalizer}, liveFinalizers(t, fakeClient, "c"))
}

func TestRemoveFromResources_whenBudgetIsNotSufficient_removesOneFinalizerAndIsNotDone(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer), newParent("b", parentFinalizer))
	manager := finalizer.NewManager(parentFinalizer, true)

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, testParents, -time.Second)

	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, 1, removed)
	assert.Equal(t, 1, countUpdates(fakeClient))

	// The second call completes the work.
	removed, done, err = manager.RemoveFromResources(context.TODO(), dynClient, testParents, -time.Second)

	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, 1, removed)
	assert.Empty(t, liveFinalizers(t, fakeClient, "a"))
	assert.Empty(t, liveFinalizers(t, fakeClient, "b"))
}

func TestRemoveFromResources_whenUpdateFails_returnsErrorAndDoesOtherParents(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer), newParent("b", parentFinalizer))
	fakeClient.PrependReactor("update", "*", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		update, ok := action.(clientgotesting.UpdateAction)
		if ok && update.GetObject().(*unstructured.Unstructured).GetName() == "a" {
			return true, nil, errors.New("update refused")
		}
		return false, nil, nil
	})
	manager := finalizer.NewManager(parentFinalizer, true)

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, testParents, largeBudget)

	require.ErrorContains(t, err, "update refused")
	assert.False(t, done)
	assert.Equal(t, 1, removed)
	assert.Equal(t, []string{parentFinalizer}, liveFinalizers(t, fakeClient, "a"))
	assert.Empty(t, liveFinalizers(t, fakeClient, "b"))
}

func TestRemoveFromResources_whenUpdateHasConflict_isNotDoneWithoutError(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer))
	fakeClient.PrependReactor("update", "*", func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewConflict(testGVR.GroupResource(), "a", errors.New("changed"))
	})
	manager := finalizer.NewManager(parentFinalizer, true)

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, testParents, largeBudget)

	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, 0, removed)
}

func TestRemoveFromResources_whenParentIsDeletedDuringCleanup_isDone(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer))
	fakeClient.PrependReactor("update", "*", func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(testGVR.GroupResource(), "a")
	})
	manager := finalizer.NewManager(parentFinalizer, true)

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, testParents, largeBudget)

	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, 0, removed)
}

func TestRemoveFromResources_whenListFails_returnsError(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer))
	fakeClient.PrependReactor("list", "*", func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list refused")
	})
	manager := finalizer.NewManager(parentFinalizer, true)

	_, done, err := manager.RemoveFromResources(context.TODO(), dynClient, testParents, largeBudget)

	require.ErrorContains(t, err, "list refused")
	assert.False(t, done)
}

func TestRemoveFromResources_whenResourceIsNotInDiscovery_returnsError(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer))
	manager := finalizer.NewManager(parentFinalizer, true)
	parents := []finalizer.ParentResource{{APIVersion: TestAPIVersion, Resource: "unknown"}}

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, parents, largeBudget)

	require.Error(t, err)
	assert.False(t, done)
	assert.Equal(t, 0, removed)
	assert.Empty(t, fakeClient.Actions())
}

func TestRemoveFromResources_whenListHasTwoPages_removesFinalizerFromAllPages(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer), newParent("b", parentFinalizer))
	// The fake client does not keep the continue token of the request, thus
	// the reactor counts the list calls.
	listCalls := 0
	fakeClient.PrependReactor("list", "*", func(clientgotesting.Action) (bool, runtime.Object, error) {
		listCalls++
		page := &unstructured.UnstructuredList{Object: map[string]interface{}{"apiVersion": TestAPIVersion, "kind": TestResourceList}}
		if listCalls == 1 {
			page.Items = []unstructured.Unstructured{*newParent("a", parentFinalizer)}
			page.SetContinue("page-2")
		} else {
			page.Items = []unstructured.Unstructured{*newParent("b", parentFinalizer)}
		}
		return true, page, nil
	})
	manager := finalizer.NewManager(parentFinalizer, true)

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, testParents, largeBudget)

	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, 2, removed)
	assert.Equal(t, 2, listCalls)
	assert.Empty(t, liveFinalizers(t, fakeClient, "a"))
	assert.Empty(t, liveFinalizers(t, fakeClient, "b"))
}

func newControllerClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	return clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestEnsureControllerFinalizer_whenFinalizerIsAbsent_addsFinalizer(t *testing.T) {
	dc := &v1alpha1.DecoratorController{ObjectMeta: metav1.ObjectMeta{Name: "dc", Finalizers: []string{otherFinalizer}}}
	k8sClient := newControllerClient(t, dc)

	require.NoError(t, finalizer.EnsureControllerFinalizer(context.TODO(), k8sClient, dc))
	// The second call must not change the object.
	require.NoError(t, finalizer.EnsureControllerFinalizer(context.TODO(), k8sClient, dc))

	live := &v1alpha1.DecoratorController{}
	require.NoError(t, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(dc), live))
	assert.Equal(t, []string{otherFinalizer, finalizer.ControllerFinalizer}, live.Finalizers)
}

func TestRemoveControllerFinalizer_whenFinalizerIsPresent_removesOnlyThatFinalizer(t *testing.T) {
	cc := &v1alpha1.CompositeController{ObjectMeta: metav1.ObjectMeta{Name: "cc", Finalizers: []string{otherFinalizer, finalizer.ControllerFinalizer}}}
	k8sClient := newControllerClient(t, cc)

	require.NoError(t, finalizer.RemoveControllerFinalizer(context.TODO(), k8sClient, cc))

	live := &v1alpha1.CompositeController{}
	require.NoError(t, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(cc), live))
	assert.Equal(t, []string{otherFinalizer}, live.Finalizers)
}

func TestRemoveControllerFinalizer_whenControllerIsAlreadyDeleted_returnsNoError(t *testing.T) {
	cc := &v1alpha1.CompositeController{ObjectMeta: metav1.ObjectMeta{Name: "cc", Finalizers: []string{finalizer.ControllerFinalizer}}}
	k8sClient := newControllerClient(t)

	require.NoError(t, finalizer.RemoveControllerFinalizer(context.TODO(), k8sClient, cc))
}

func newCleaner(k8sClient client.Client, dynClient *dynamicclientset.Clientset, resources *dynamicdiscovery.ResourceMap) *finalizer.ControllerCleaner {
	return &finalizer.ControllerCleaner{
		K8sClient:     k8sClient,
		DynClient:     dynClient,
		Resources:     resources,
		EventRecorder: NewFakeRecorder(),
		Logger:        logr.Discard(),
	}
}

func TestFinalize_whenControllerHasFinalizer_stopsCleansParentsAndRemovesFinalizer(t *testing.T) {
	dc := &v1alpha1.DecoratorController{ObjectMeta: metav1.ObjectMeta{Name: "dc", Finalizers: []string{otherFinalizer, finalizer.ControllerFinalizer}}}
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))
	stopped := false

	result, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, testParents, func() bool {
		stopped = true
		return true
	})

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
	assert.True(t, stopped)
	assert.Empty(t, liveFinalizers(t, fakeClient, "a"))
	live := &v1alpha1.DecoratorController{}
	require.NoError(t, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(dc), live))
	assert.Equal(t, []string{otherFinalizer}, live.Finalizers)
}

func TestFinalize_whenControllerHasNoFinalizer_stopsAndKeepsParentFinalizers(t *testing.T) {
	dc := &v1alpha1.DecoratorController{ObjectMeta: metav1.ObjectMeta{Name: "dc", Finalizers: []string{otherFinalizer}}}
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))
	stopped := false

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, testParents, func() bool {
		stopped = true
		return false
	})

	require.NoError(t, err)
	assert.True(t, stopped)
	assert.Empty(t, fakeClient.Actions())
}

func TestFinalize_whenCleanupFails_returnsErrorAndKeepsControllerFinalizer(t *testing.T) {
	dc := &v1alpha1.DecoratorController{ObjectMeta: metav1.ObjectMeta{Name: "dc", Finalizers: []string{finalizer.ControllerFinalizer}}}
	k8sClient := newControllerClient(t, dc)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))
	fakeClient.PrependReactor("update", "*", func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update refused")
	})

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, testParents, func() bool { return false })

	require.ErrorContains(t, err, "update refused")
	live := &v1alpha1.DecoratorController{}
	require.NoError(t, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(dc), live))
	assert.Equal(t, []string{finalizer.ControllerFinalizer}, live.Finalizers)
}

// The tests below are for a parent resource that is not in discovery.

func newPendingController() *v1alpha1.DecoratorController {
	return &v1alpha1.DecoratorController{ObjectMeta: metav1.ObjectMeta{Name: "dc", Finalizers: []string{finalizer.ControllerFinalizer}}}
}

func newCRD(name, group string, versions ...map[string]interface{}) *unstructured.Unstructured {
	crd := &unstructured.Unstructured{}
	crd.SetAPIVersion("apiextensions.k8s.io/v1")
	crd.SetKind("CustomResourceDefinition")
	crd.SetName(name + "." + group)
	list := make([]interface{}, 0, len(versions))
	for _, version := range versions {
		list = append(list, version)
	}
	_ = unstructured.SetNestedSlice(crd.Object, list, "spec", "versions")
	return crd
}

func crdVersion(name string, served bool) map[string]interface{} {
	return map[string]interface{}{"name": name, "served": served}
}

func newAPIService(name string) *unstructured.Unstructured {
	apiService := &unstructured.Unstructured{}
	apiService.SetAPIVersion("apiregistration.k8s.io/v1")
	apiService.SetKind("APIService")
	apiService.SetName(name)
	return apiService
}

func controllerFinalizers(t *testing.T, k8sClient client.Client, dc *v1alpha1.DecoratorController) []string {
	t.Helper()
	live := &v1alpha1.DecoratorController{}
	require.NoError(t, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(dc), live))
	return live.Finalizers
}

func noStop() bool { return false }

func TestFinalize_whenSpecVersionIsNotServed_cleansParentsWithServedVersion(t *testing.T) {
	dc := newPendingController()
	crd := newCRD(TestResource, TestGroup, crdVersion("old", false), crdVersion(TestVersion, true))
	k8sClient := newControllerClient(t, dc, crd)
	dynClient, fakeClient, resources := newCleanupClientsAndResources(newParent("a", parentFinalizer))
	parents := []finalizer.ParentResource{{APIVersion: TestGroup + "/old", Resource: TestResource}}

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, parents, noStop)

	require.NoError(t, err)
	assert.Empty(t, liveFinalizers(t, fakeClient, "a"))
	assert.Empty(t, controllerFinalizers(t, k8sClient, dc))
}

func TestFinalize_whenCRDExistsButNoServedVersionIsInDiscovery_returnsErrorAndKeepsControllerFinalizer(t *testing.T) {
	dc := newPendingController()
	crd := newCRD("others", TestGroup, crdVersion("v1", true))
	k8sClient := newControllerClient(t, dc, crd)
	dynClient, _, resources := newCleanupClientsAndResources()
	parents := []finalizer.ParentResource{{APIVersion: TestGroup + "/v1", Resource: "others"}}

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, parents, noStop)

	require.ErrorContains(t, err, "no served version")
	assert.Equal(t, []string{finalizer.ControllerFinalizer}, controllerFinalizers(t, k8sClient, dc))
}

func TestFinalize_whenResourceDoesNotExistInServedAPIVersion_removesControllerFinalizer(t *testing.T) {
	dc := newPendingController()
	k8sClient := newControllerClient(t, dc)
	dynClient, _, resources := newCleanupClientsAndResources()
	parents := []finalizer.ParentResource{{APIVersion: TestAPIVersion, Resource: "unknown"}}

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, parents, noStop)

	require.NoError(t, err)
	assert.Empty(t, controllerFinalizers(t, k8sClient, dc))
}

func TestFinalize_whenAPIVersionDoesNotExist_removesControllerFinalizer(t *testing.T) {
	dc := newPendingController()
	k8sClient := newControllerClient(t, dc)
	dynClient, _, resources := newCleanupClientsAndResources()
	parents := []finalizer.ParentResource{{APIVersion: "gone.example.com/v1", Resource: "things"}}

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, parents, noStop)

	require.NoError(t, err)
	assert.Empty(t, controllerFinalizers(t, k8sClient, dc))
}

func TestFinalize_whenAPIServiceExistsButAPIVersionIsNotInDiscovery_returnsErrorAndKeepsControllerFinalizer(t *testing.T) {
	dc := newPendingController()
	k8sClient := newControllerClient(t, dc, newAPIService("v1.aggregated.example.com"))
	dynClient, _, resources := newCleanupClientsAndResources()
	parents := []finalizer.ParentResource{{APIVersion: "aggregated.example.com/v1", Resource: "things"}}

	_, err := newCleaner(k8sClient, dynClient, resources).Finalize(context.TODO(), dc, parentFinalizer, parents, noStop)

	require.ErrorContains(t, err, "not in discovery")
	assert.Equal(t, []string{finalizer.ControllerFinalizer}, controllerFinalizers(t, k8sClient, dc))
}
