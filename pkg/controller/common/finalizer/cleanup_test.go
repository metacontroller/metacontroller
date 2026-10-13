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
	objects := make([]runtime.Object, 0, len(parents))
	for _, parent := range parents {
		objects = append(objects, parent)
	}
	gvrToListKind := map[schema.GroupVersionResource]string{testGVR: TestResourceList}
	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objects...)
	resourceMap := NewFakeResourceMap(NewFakeClientsetWithResources(NewDefaultAPIResourceList()))
	return NewClientset(NewDefaultRestConfig(), resourceMap, fakeClient), fakeClient
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

func TestRemoveFromResources_whenResourceIsNotInDiscovery_isDone(t *testing.T) {
	dynClient, fakeClient := newCleanupClients(newParent("a", parentFinalizer))
	manager := finalizer.NewManager(parentFinalizer, true)
	parents := []finalizer.ParentResource{{APIVersion: TestAPIVersion, Resource: "unknown"}}

	removed, done, err := manager.RemoveFromResources(context.TODO(), dynClient, parents, largeBudget)

	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, 0, removed)
	assert.Empty(t, fakeClient.Actions())
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
