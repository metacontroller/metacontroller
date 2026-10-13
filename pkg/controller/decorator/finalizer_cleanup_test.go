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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"

	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/common/finalizer"
	. "metacontroller/pkg/internal/testutils/common"
	"metacontroller/pkg/logging"
)

const (
	cleanupTestFinalizer = "metacontroller.io/decoratorcontroller-test"
	otherTestFinalizer   = "example.com/other"
)

var cleanupTestGVR = schema.GroupVersionResource{Group: TestGroup, Version: TestVersion, Resource: TestResource}

// newCleanupTestController returns a decoratorController whose parent informer
// cache has one parent object with the given finalizers.
func newCleanupTestController(t *testing.T, finalizers []string) (*decoratorController, *fake.FakeDynamicClient) {
	t.Helper()
	fakeClient, resources, dynClient, _, parentInformersMap := newDefaultControllerClientsAndInformers(NoOpFn, true, false)
	parentInformers := common.NewInformerMap()
	for gvr, informer := range parentInformersMap {
		parentInformers.Set(gvr, informer)
	}

	parent := NewDefaultUnstructured()
	parent.SetFinalizers(finalizers)
	_, err := fakeClient.Resource(cleanupTestGVR).Namespace(TestNamespace).Update(context.TODO(), parent, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		cached, err := parentInformers.Get(cleanupTestGVR).Lister().Namespace(TestNamespace).Get(TestName)
		return err == nil && len(cached.GetFinalizers()) == len(finalizers)
	}, 5*time.Second, 10*time.Millisecond, "the informer cache did not get the parent finalizers")
	fakeClient.ClearActions()

	c := &decoratorController{
		dc:              newDefaultDecoratorController(),
		resources:       resources,
		dynClient:       dynClient,
		parentInformers: parentInformers,
		finalizer:       finalizer.NewManager(cleanupTestFinalizer, true),
		logger:          logging.Logger,
	}
	return c, fakeClient
}

func liveParentFinalizers(t *testing.T, fakeClient *fake.FakeDynamicClient) []string {
	t.Helper()
	parent, err := fakeClient.Resource(cleanupTestGVR).Namespace(TestNamespace).Get(context.TODO(), TestName, metav1.GetOptions{})
	require.NoError(t, err)
	return parent.GetFinalizers()
}

func countUpdates(fakeClient *fake.FakeDynamicClient) int {
	count := 0
	for _, action := range fakeClient.Actions() {
		if action.GetVerb() == "update" {
			count++
		}
	}
	return count
}

func TestCleanupParentFinalizers_whenParentHasFinalizer_removesOnlyThatFinalizer(t *testing.T) {
	c, fakeClient := newCleanupTestController(t, []string{otherTestFinalizer, cleanupTestFinalizer})

	c.cleanupParentFinalizers(context.TODO())

	assert.Equal(t, []string{otherTestFinalizer}, liveParentFinalizers(t, fakeClient))
}

func TestCleanupParentFinalizers_whenParentHasNoFinalizer_doesNotUpdateParent(t *testing.T) {
	c, fakeClient := newCleanupTestController(t, []string{otherTestFinalizer})

	c.cleanupParentFinalizers(context.TODO())

	assert.Equal(t, 0, countUpdates(fakeClient))
	assert.Equal(t, []string{otherTestFinalizer}, liveParentFinalizers(t, fakeClient))
}

func TestCleanupParentFinalizers_whenUpdateFails_keepsFinalizerAndReturns(t *testing.T) {
	c, fakeClient := newCleanupTestController(t, []string{cleanupTestFinalizer})
	fakeClient.PrependReactor("update", "*", func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update refused")
	})

	c.cleanupParentFinalizers(context.TODO())

	assert.Equal(t, 1, countUpdates(fakeClient))
	assert.Equal(t, []string{cleanupTestFinalizer}, liveParentFinalizers(t, fakeClient))
}

func TestCleanupParentFinalizers_whenContextIsCancelled_keepsFinalizer(t *testing.T) {
	c, fakeClient := newCleanupTestController(t, []string{cleanupTestFinalizer})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c.cleanupParentFinalizers(ctx)

	assert.Equal(t, 0, countUpdates(fakeClient))
	assert.Equal(t, []string{cleanupTestFinalizer}, liveParentFinalizers(t, fakeClient))
}

func TestCleanupParentFinalizers_whenResourceIsNotInDiscovery_keepsFinalizer(t *testing.T) {
	c, fakeClient := newCleanupTestController(t, []string{cleanupTestFinalizer})
	informer := c.parentInformers.Get(cleanupTestGVR)
	c.parentInformers = common.NewInformerMap()
	c.parentInformers.Set(schema.GroupVersionResource{Group: TestGroup, Version: TestVersion, Resource: "unknown"}, informer)

	c.cleanupParentFinalizers(context.TODO())

	assert.Equal(t, 0, countUpdates(fakeClient))
	assert.Equal(t, []string{cleanupTestFinalizer}, liveParentFinalizers(t, fakeClient))
}
