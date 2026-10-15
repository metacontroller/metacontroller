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
	"fmt"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	v1 "metacontroller/pkg/controller/decorator/api/v1"
	"metacontroller/test/integration/framework"
)

// TestFinalizerCleanup_whenControllerIsDeleted_removesParentFinalizers tests that Metacontroller
// removes its finalizer from the parent objects when the DecoratorController
// is deleted.
func TestFinalizerCleanup_whenControllerIsDeleted_removesParentFinalizers(t *testing.T) {
	ns := "test-finalizer-cleanup"
	name := "finalizer-cleanup"
	// Use more than one parent. With one parent, a finalizer that a worker
	// adds again during the cleanup is easy to miss.
	const parentCount = 10

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("FinalizerCleanupParent", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(_ []byte) ([]byte, error) {
		return json.Marshal(v1.DecoratorHookResponse{})
	})

	// The finalize hook makes Metacontroller add a finalizer to the parents
	// and to the controller object.
	controller := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name},
		Spec: v1alpha1.DecoratorControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: *framework.CRDResourceRule(parentCRD)},
			},
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync:     &v1alpha1.Hook{Webhook: &v1alpha1.Webhook{URL: &hook.URL}},
				Finalize: &v1alpha1.Hook{Webhook: &v1alpha1.Webhook{URL: &hook.URL}},
			},
		},
	}

	f.RunFinalizerCleanupOnDelete(framework.FinalizerCleanupCase{
		Controller:      controller,
		ParentFinalizer: "metacontroller.io/decoratorcontroller-" + name,
		Namespace:       ns,
		ParentClient:    parentClient,
		Parents:         newFinalizerCleanupParents(parentCRD, parentCount),
	})
}

// TestFinalizerCleanup_whenControllerIsNotInOperation_removesParentFinalizers tests the cleanup for
// a DecoratorController that has no controller in operation. The
// DecoratorController has no hooks, thus its controller cannot start.
func TestFinalizerCleanup_whenControllerIsNotInOperation_removesParentFinalizers(t *testing.T) {
	ns := "test-finalizer-cleanup-stopped"
	name := "finalizer-cleanup-stopped"

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("FinalizerCleanupStoppedParent", apiextensions.NamespaceScoped)

	controller := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name},
		Spec: v1alpha1.DecoratorControllerSpec{
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: *framework.CRDResourceRule(parentCRD)},
			},
		},
	}

	f.RunFinalizerCleanupNotInOperation(framework.FinalizerCleanupCase{
		Controller:      controller,
		ParentFinalizer: "metacontroller.io/decoratorcontroller-" + name,
		Namespace:       ns,
		ParentClient:    parentClient,
		Parents:         newFinalizerCleanupParents(parentCRD, 1),
	})
}

// TestFinalizerCleanup_whenParentAPIDoesNotExist_deletesController tests that a
// DecoratorController whose parent API does not exist can be deleted.
func TestFinalizerCleanup_whenParentAPIDoesNotExist_deletesController(t *testing.T) {
	name := "finalizer-cleanup-no-api"

	f := framework.NewFixture(t)
	defer f.TearDown()

	controller := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name},
		Spec: v1alpha1.DecoratorControllerSpec{
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: v1alpha1.ResourceRule{APIVersion: "gone.example.com/v1", Resource: "things"}},
			},
		},
	}

	f.RunFinalizerCleanupNotInOperation(framework.FinalizerCleanupCase{
		Controller:      controller,
		ParentFinalizer: "metacontroller.io/decoratorcontroller-" + name,
	})
}

func newFinalizerCleanupParents(parentCRD *apiextensions.CustomResourceDefinition, count int) []*unstructured.Unstructured {
	parents := make([]*unstructured.Unstructured, 0, count)
	for i := 0; i < count; i++ {
		parent := framework.UnstructuredCRD(parentCRD, fmt.Sprintf("parent-%d", i))
		parents = append(parents, parent)
	}
	return parents
}
