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
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	v1 "metacontroller/pkg/controller/composite/api/v1"
	"metacontroller/test/integration/framework"
)

// The child handler of a controller ignores the objects that the informer
// replays at start. This test shows that the first sync of a parent still
// finds a child that exists before the controller starts.
func TestControllerStart_whenOrphanExistsBeforeController_adoptsTheOrphan(t *testing.T) {
	ns := "test-start-existing-orphan"
	matchingLabels := map[string]string{"test": ns}
	childName := "pre-existing-child"
	f := framework.NewFixture(t)
	defer f.TearDown()
	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("TestStartExistingOrphanParent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestStartExistingOrphanChild", apiextensions.NamespaceScoped)
	// The hook asks for the child, so the controller keeps it after adoption.
	hook := f.ServeWebhook(func([]byte) ([]byte, error) {
		child := framework.UnstructuredCRD(childCRD, childName)
		child.SetLabels(matchingLabels)
		return json.Marshal(v1.CompositeHookResponse{Children: []*unstructured.Unstructured{child}})
	})
	orphan := framework.UnstructuredCRD(childCRD, childName)
	orphan.SetLabels(matchingLabels)
	if _, err := childClient.Namespace(ns).Create(context.TODO(), orphan, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	parent := framework.UnstructuredCRD(parentCRD, "parent")
	if err := unstructured.SetNestedStringMap(parent.Object, matchingLabels, "spec", "selector", "matchLabels"); err != nil {
		t.Fatal(err)
	}
	if _, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	f.CreateCompositeController(ns, hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), nil)

	err := f.Wait(func() (bool, error) {
		child, err := childClient.Namespace(ns).Get(context.TODO(), childName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return metav1.GetControllerOf(child) != nil, nil
	})
	if err != nil {
		t.Fatalf("orphan child was never adopted: %v", err)
	}
}
