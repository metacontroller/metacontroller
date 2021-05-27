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
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/customize"
	"metacontroller/pkg/controller/composite"
	"metacontroller/test/integration/framework"
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

	f.CreateCompositeController("test-sync-webhook", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD))

	parent := framework.UnstructuredCRD(parentCRD, "test-sync-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(parent, metav1.CreateOptions{})
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
func TestCascadingDelete(t *testing.T) {
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

	f.CreateCompositeController("test-cascading-delete", hook.URL, "", framework.CRDResourceRule(parentCRD), &v1alpha1.ResourceRule{APIVersion: "batch/v1", Resource: "jobs"})

	parent := framework.UnstructuredCRD(parentCRD, "test-cascading-delete")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	unstructured.SetNestedField(parent.Object, int64(1), "spec", "replicas")
	var err error
	if parent, err = parentClient.Namespace(ns).Create(parent, metav1.CreateOptions{}); err != nil {
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

// TestResyncAfter tests that the resyncAfterSeconds field works.
func TestResyncAfter(t *testing.T) {
	ns := "test-resync-after"
	labels := map[string]string{
		"test": "test-sync-after",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("TestResyncAfterParent", apiextensions.NamespaceScoped)

	var lastSync time.Time
	done := false
	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := composite.SyncHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		resp := composite.SyncHookResponse{}
		if req.Parent.Object["status"] == nil {
			// If status hasn't been set yet, set it. This is the "zeroth" sync.
			// Metacontroller will set our status and then the object should quiesce.
			resp.Status = map[string]interface{}{}
		} else if lastSync.IsZero() {
			// This should be the final sync before quiescing. Do nothing except
			// request a resync. Other than our resyncAfter request, there should be
			// nothing that causes our object to get resynced.
			lastSync = time.Now()
			resp.ResyncAfterSeconds = 0.1
		} else if !done {
			done = true
			// This is the second sync. Report how much time elapsed.
			resp.Status = map[string]interface{}{
				"elapsedSeconds": time.Since(lastSync).Seconds(),
			}
		} else {
			// If we're done, just freeze the status.
			resp.Status = req.Parent.Object["status"].(map[string]interface{})
		}
		return json.Marshal(resp)
	})

	f.CreateCompositeController("test-resync-after", hook.URL, "", framework.CRDResourceRule(parentCRD), nil)

	parent := framework.UnstructuredCRD(parentCRD, "test-resync-after")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for elapsed time to be reported...")
	var elapsedSeconds float64
	err = f.Wait(func() (bool, error) {
		parent, err := parentClient.Namespace(ns).Get("test-resync-after", metav1.GetOptions{})
		val, found, err := unstructured.NestedFloat64(parent.Object, "status", "elapsedSeconds")
		if err != nil || !found {
			// The value hasn't been populated. Keep waiting.
			return false, err
		}
		elapsedSeconds = val
		return true, nil
	})
	if err != nil {
		t.Fatalf("didn't find expected status field: %v", err)
	}

	t.Logf("elapsedSeconds: %v", elapsedSeconds)
	if elapsedSeconds > 1.0 {
		t.Errorf("requested resyncAfter did not occur in time; elapsedSeconds: %v", elapsedSeconds)
	}
}

// TestCustomizeWebhook tests that the sync and customize webhook triggers and passes the
// request/response properly.
func TestCustomizeWebhook(t *testing.T) {
	namespace := "test-customize-webhook"
	relatedResourceName := "related-config-map"
	labels := map[string]string{
		"test": "test-customize-webhook",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(namespace)
	parentCRD, parentClient := f.CreateCRD("TestCustomizeWebhookParent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestCustomizeWebhookChild", apiextensions.NamespaceScoped)
	relatedClient := f.Clientset().CoreV1().ConfigMaps(namespace)
	relatedConfigMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      relatedResourceName,
			Namespace: namespace,
		},
		Data: make(map[string]string, 0),
	}

	_, err := relatedClient.Create(&relatedConfigMap)
	if err != nil {
		t.Fatal(err)
	}

	customizeHook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		type compositeCustomizeRequest struct {
			Controller v1alpha1.CompositeController `json:"controller"`
			Parent     unstructured.Unstructured    `json:"parent"`
		}
		req := compositeCustomizeRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		resp := customize.CustomizeHookResponse{
			RelatedResourceRules: []*v1alpha1.RelatedResourceRule{
				{
					ResourceRule: v1alpha1.ResourceRule{
						APIVersion: "v1",
						Resource:   "configmaps",
					},
					Namespace: req.Parent.GetNamespace(),
					Names:     []string{relatedResourceName},
				},
			},
		}
		return json.Marshal(resp)
	})

	syncHook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := composite.SyncHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// As a simple test of request/response content,
		// just create a child with name composes from parent name and related ConfigMap name.
		var children []*unstructured.Unstructured
		if len(req.Related.List()) == 0 {
			children = make([]*unstructured.Unstructured, 0)
		} else {
			related := req.Related.List()[0]
			child := framework.UnstructuredCRD(childCRD, req.Parent.GetName()+"-"+related.GetName())
			child.SetLabels(labels)
			children = []*unstructured.Unstructured{child}
		}
		resp := composite.SyncHookResponse{
			Children: children,
		}
		return json.Marshal(resp)
	})

	f.CreateCompositeController("test-customize-webhook", syncHook.URL, customizeHook.URL, framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD))

	parent := framework.UnstructuredCRD(parentCRD, "test-customize-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err = parentClient.Namespace(namespace).Create(parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err := childClient.Namespace(namespace).Get("test-customize-webhook-"+relatedResourceName, metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Errorf("didn't find expected child: %v", err)
	}
}

func TestFailIfNoStatusSubresourceInParentCRD(t *testing.T) {
	namespace := "test-sync-subresource-status"
	labels := map[string]string{
		"test": namespace,
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(namespace)
	parentCRD, parentClient := f.CreateCRDWithoutStatusSubresource("TestSyncWebhookParent", apiextensions.NamespaceScoped)
	childCRD, _ := f.CreateCRD("TestSyncWebhookChild", apiextensions.NamespaceScoped)
	eventsClient := f.Clientset().CoreV1().Events("default")
	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {

		resp := composite.SyncHookResponse{}
		return json.Marshal(resp)
	})

	f.CreateCompositeController("test-sync-webhook", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD))

	parent := framework.UnstructuredCRD(parentCRD, "test-sync-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(namespace).Create(parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for warn event to be created...")
	err = f.Wait(func() (bool, error) {
		events, err := eventsClient.List(metav1.ListOptions{})
		for _, event := range events.Items {
			t.Logf("Event: %s", event.Message)
			if strings.Contains(event.Message, "does not have subresource 'Status' enabled") {
				return true, nil
			}
		}
		return false, err
	})
	if err != nil {
		t.Errorf("didn't find expected event: %v", err)
	}
}
