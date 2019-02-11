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

	batchv1 "k8s.io/api/batch/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"metacontroller.app/apis/metacontroller/v1alpha1"
	"metacontroller.app/controller/composite"
	"metacontroller.app/test/integration/framework"
)

func TestMain(m *testing.M) {
	framework.TestMain(m.Run)
}

// TestSyncWebhook tests that the sync webhook triggers and passes the
// request/response properly.
func TestSyncWebhook(t *testing.T) {
	ns := "test-sync-webhook"
	labels := map[string]string{
		"test": "test-sync-webhook",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("TestSyncWebhookParent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestSyncWebhookChild", apiextensions.NamespaceScoped)

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

	f.CreateCompositeController("test-sync-webhook", hook.URL, framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD))

	parent := framework.UnstructuredCRD(parentCRD, "test-sync-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(parent)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err := childClient.Namespace(ns).Get("test-sync-webhook", metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Errorf("didn't find expected child: %v", err)
	}
}

// TestCascadingDelete tests that we request cascading deletion of children,
// even if the server-side default for that child type is non-cascading.
func TestCacadingDelete(t *testing.T) {
	ns := "test-cascading-delete"
	labels := map[string]string{
		"test": "test-cascading-delete",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("TestCascadingDeleteParent", apiextensions.NamespaceScoped)
	childClient := f.Clientset().BatchV1().Jobs(ns)

	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := composite.SyncHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		resp := composite.SyncHookResponse{}
		if replicas, _, _ := unstructured.NestedInt64(req.Parent.Object, "spec", "replicas"); replicas > 0 {
			// Create a child batch/v1 Job if requested.
			// For backward compatibility, the server-side default on that API is
			// non-cascading deletion (don't delete Pods).
			// So we can use this as a test case for whether we are correctly requesting
			// cascading deletion.
			child := framework.UnstructuredJSON("batch/v1", "Job", "test-cascading-delete", `{
				"spec": {
					"template": {
						"spec": {
							"restartPolicy": "Never",
							"containers": [
								{
									"name": "pi",
									"image": "perl"
								}
							]
						}
					}
				}
			}`)
			child.SetLabels(labels)
			resp.Children = append(resp.Children, child)
		}
		return json.Marshal(resp)
	})

	f.CreateCompositeController("test-cascading-delete", hook.URL, framework.CRDResourceRule(parentCRD), v1alpha1.ResourceRule{APIVersion: "batch/v1", Resource: "jobs"})

	parent := framework.UnstructuredCRD(parentCRD, "test-cascading-delete")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	unstructured.SetNestedField(parent.Object, int64(1), "spec", "replicas")
	var err error
	if parent, err = parentClient.Namespace(ns).Create(parent); err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err := childClient.Get("test-cascading-delete", metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Errorf("didn't find expected child: %v", err)
	}

	// Now that child exists, tell parent to delete it.
	t.Logf("Updating parent to set replicas=0...")
	_, err = parentClient.Namespace(ns).AtomicUpdate(parent, func(obj *unstructured.Unstructured) bool {
		unstructured.SetNestedField(obj.Object, int64(0), "spec", "replicas")
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the child gets actually deleted, which means no GC finalizers got
	// added to it. Note that we don't actually run the GC in this integration
	// test env, so we don't need to worry about the GC racing us to process the
	// finalizers.
	t.Logf("Waiting for child object to be deleted...")
	var child *batchv1.Job
	err = f.Wait(func() (bool, error) {
		var getErr error
		child, getErr = childClient.Get("test-cascading-delete", metav1.GetOptions{})
		return apierrors.IsNotFound(getErr), nil
	})
	if err != nil {
		out, _ := json.Marshal(child)
		t.Errorf("timed out waiting for child object to be deleted: %v; object: %s", err, out)
	}
}
