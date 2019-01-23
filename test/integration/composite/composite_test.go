/*
Copyright 2019 Google Inc.

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
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"metacontroller.app/controller/composite"
	"metacontroller.app/test/integration/framework"
)

func TestMain(m *testing.M) {
	framework.TestMain(m.Run)
}

// TestSyncWebhook tests that the sync webhook triggers and passes the
// resquest/response properly.
func TestSyncWebhook(t *testing.T) {
	ns := "test-sync-webhook"
	labels := map[string]string{
		"test": "test",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("Parent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("Child", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := composite.SyncHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// As a simple test of request/response content,
		// just create a child with the same name as the parent.
		child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
		child.SetLabels(labels)
		resp := composite.SyncHookResponse{
			Children: []*unstructured.Unstructured{child},
		}
		return json.Marshal(resp)
	})

	f.CreateCompositeController("cc", hook.URL, parentCRD, childCRD)

	parent := framework.UnstructuredCRD(parentCRD, "test-sync-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(parent)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err = childClient.Namespace(ns).Get("test-sync-webhook", metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Errorf("didn't find expected child: %v", err)
	}
}
