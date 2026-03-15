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
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	customizeV2 "metacontroller/pkg/controller/common/customize/api/v2"
	v2 "metacontroller/pkg/controller/composite/api/v2"
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
	parentCRD, parentClient := f.CreateCRD("TestSyncWebhookV2Parent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestSyncWebhookV2Child", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := v2.CompositeHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}

		// As a simple test of request/response content,
		// just create a child with the same name as the parent.
		child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
		child.SetLabels(labels)
		resp := v2.CompositeHookResponse{
			Children: []*unstructured.Unstructured{child},
		}
		return json.Marshal(resp)
	})

	v2Version := v1alpha1.HookVersionV2
	cc := f.CreateCompositeController("test-sync-webhook-v2", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), nil)
	// Update to use v2
	cc.Spec.Hooks.Sync.Version = &v2Version
	if err := f.MetacontrollerClient().Update(context.TODO(), cc); err != nil {
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

// TestCustomizeWebhookV2 tests that the sync and customize webhook with version v2 triggers and passes the
// request/response properly.
func TestCustomizeWebhookV2(t *testing.T) {
	namespace := "test-customize-webhook-v2"
	relatedResourceName := "related-config-map"
	labels := map[string]string{
		"test": "test-customize-webhook-v2",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(namespace)
	parentCRD, parentClient := f.CreateCRD("TestCustomizeV2Parent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestCustomizeV2Child", apiextensions.NamespaceScoped)
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
		req := struct {
			Controller json.RawMessage            `json:"controller"`
			Parent     *unstructured.Unstructured `json:"parent"`
		}{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		resp := customizeV2.CustomizeHookResponse{
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
		req := v2.CompositeHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// As a simple test of request/response content,
		// just create a child with name composes from parent name and related ConfigMap name.
		var children []*unstructured.Unstructured
		if len(req.Related) == 0 {
			children = make([]*unstructured.Unstructured, 0)
		} else {
			related := req.Related.List()[0]
			child := framework.UnstructuredCRD(childCRD, req.Parent.GetName()+"-"+related.GetName())
			child.SetLabels(labels)
			children = []*unstructured.Unstructured{child}
		}
		resp := v2.CompositeHookResponse{
			Children: children,
		}
		return json.Marshal(resp)
	})

	v2Version := v1alpha1.HookVersionV2
	cc := f.CreateCompositeController("test-customize-webhook-v2", syncHook.URL, customizeHook.URL, framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), nil)
	// Update to use v2 for both hooks
	cc.Spec.Hooks.Sync.Version = &v2Version
	cc.Spec.Hooks.Customize.Version = &v2Version
	if err := f.MetacontrollerClient().Update(context.TODO(), cc); err != nil {
		t.Fatal(err)
	}

	parent := framework.UnstructuredCRD(parentCRD, "test-customize-webhook-v2")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err = parentClient.Namespace(namespace).Create(context.TODO(), parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	var child *unstructured.Unstructured
	err = f.Wait(func() (bool, error) {
		var err error
		child, err = childClient.Namespace(namespace).Get(context.TODO(), "test-customize-webhook-v2-"+relatedResourceName, metav1.GetOptions{})
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
