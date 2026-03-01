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
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	v2 "metacontroller/pkg/controller/decorator/api/v2"
	"metacontroller/test/integration/framework"
)

// TestSyncWebhookV2 tests that the sync webhook with version v2 triggers and passes the
// request/response properly using UniformObjectMap.
func TestSyncWebhookV2(t *testing.T) {
	ns := "test-sync-webhook-v2"
	labels := map[string]string{
		"test": "test-sync-webhook-v2",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("DecoratorParentV2", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("DecoratorChildV2", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := v2.DecoratorHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}

		// As a simple test of request/response content,
		// just create a child with the same name as the parent.
		child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
		child.SetLabels(labels)
		resp := v2.DecoratorHookResponse{
			Attachments: []*unstructured.Unstructured{child},
		}
		return json.Marshal(resp)
	})

	v2Version := v1alpha1.HookVersionV2
	dc := f.CreateDecoratorController("test-sync-webhook-v2", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), nil)
	// Update to use v2
	dc.Spec.Hooks.Sync.Version = &v2Version
	if err := f.MetacontrollerClient().Update(context.TODO(), dc); err != nil {
		t.Fatal(err)
	}

	parent := framework.UnstructuredCRD(parentCRD, "test-sync-webhook-v2")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	var child *unstructured.Unstructured
	err = f.Wait(func() (bool, error) {
		var err error
		child, err = childClient.Namespace(ns).Get(context.TODO(), "test-sync-webhook-v2", metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Fatalf("didn't find expected child: %v", err)
	}

	// Verify child content
	if child.GetLabels()["test"] != labels["test"] {
		t.Errorf("child label mismatch: got %v, want %v", child.GetLabels()["test"], labels["test"])
	}
}
