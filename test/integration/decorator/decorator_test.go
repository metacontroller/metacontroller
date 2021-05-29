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

package decorator

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/customize"
	"metacontroller/pkg/controller/decorator"
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
		"test": "test",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("Parent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("Child", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := decorator.SyncHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// As a simple test of request/response content,
		// just create a child with the same name as the parent.
		child := framework.UnstructuredCRD(childCRD, req.Object.GetName())
		child.SetLabels(labels)
		resp := decorator.SyncHookResponse{
			Attachments: []*unstructured.Unstructured{child},
		}
		return json.Marshal(resp)
	})

	f.CreateDecoratorController("dc", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD))

	parent := framework.UnstructuredCRD(parentCRD, "test-sync-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err = childClient.Namespace(ns).Get(context.TODO(), "test-sync-webhook", metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Errorf("didn't find expected child: %v", err)
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
		req := decorator.SyncHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		resp := decorator.SyncHookResponse{}
		if req.Object.Object["status"] == nil {
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
			resp.Status = req.Object.Object["status"].(map[string]interface{})
		}
		return json.Marshal(resp)
	})

	f.CreateDecoratorController("test-resync-after", hook.URL, "", framework.CRDResourceRule(parentCRD), nil)

	parent := framework.UnstructuredCRD(parentCRD, "test-resync-after")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for elapsed time to be reported...")
	var elapsedSeconds float64
	err = f.Wait(func() (bool, error) {
		parent, err := parentClient.Namespace(ns).Get(context.TODO(), "test-resync-after", metav1.GetOptions{})
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
	parentCRD, parentClient := f.CreateCRD("Parent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("Child", apiextensions.NamespaceScoped)
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

	_, err := relatedClient.Create(context.TODO(), &relatedConfigMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	customizeHook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		type compositeCustomizeRequest struct {
			Controller v1alpha1.DecoratorController `json:"controller"`
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
		req := decorator.SyncHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// As a simple test of request/response content,
		// just create a child with name composes from parent name and related ConfigMap name.
		var attachments []*unstructured.Unstructured
		if len(req.Related.List()) == 0 {
			attachments = make([]*unstructured.Unstructured, 0)
		} else {
			related := req.Related.List()[0]
			child := framework.UnstructuredCRD(childCRD, req.Object.GetName()+"-"+related.GetName())
			child.SetLabels(labels)
			attachments = []*unstructured.Unstructured{child}
		}

		resp := decorator.SyncHookResponse{
			Attachments: attachments,
		}
		return json.Marshal(resp)
	})

	f.CreateDecoratorController("dc", syncHook.URL, customizeHook.URL, framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD))

	parent := framework.UnstructuredCRD(parentCRD, "test-customize-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err = parentClient.Namespace(namespace).Create(context.TODO(), parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err = childClient.Namespace(namespace).Get(context.TODO(), "test-customize-webhook-"+relatedResourceName, metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Errorf("didn't find expected child: %v", err)
	}
}
