/*
 *
 * Copyright 2023. Metacontroller authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package composite_test_with_target_label_selector

import (
	"context"
	"encoding/json"
	v1 "metacontroller/pkg/controller/composite/api/v1"
	"metacontroller/test/integration/framework"
	"strings"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMain(m *testing.M) {
	framework.TestMainWithTargetLabelSelector(m.Run)
}

func TestWithMatchingController(t *testing.T) {
	ns := "test-cc-target-sync-webhook"
	labels := map[string]string{
		"test": "test-cc-target-sync-webhook",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("TestCCTargetSyncWebhookParent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestCCTargetSyncWebhookChild", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := v1.CompositeHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// As a simple test of request/response content,
		// just create a child with the same name as the parent.
		child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
		child.SetLabels(labels)
		resp := v1.CompositeHookResponse{
			Children: []*unstructured.Unstructured{child},
		}
		return json.Marshal(resp)
	})

	controllerLabels := &map[string]string{"foo": "bar"}
	f.CreateCompositeController("test-cc-target-sync-webhook", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), controllerLabels)

	parent := framework.UnstructuredCRD(parentCRD, "test-cc-target-sync-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err := childClient.Namespace(ns).Get(context.TODO(), "test-cc-target-sync-webhook", metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Errorf("didn't find expected child: %v", err)
	}
}

// TestWithNonMatchingController starts up metacontroller with a target-label-selector of "foo=bar";
// In this test we create our composite controller with labels of "baz=caz";
// This test then ensures that our timeout is triggered as this metacontroller instance should not
// find and process the created composite controller.
func TestWithNonMatchingController(t *testing.T) {
	ns := "test-cc-target-f-sync-webhook"
	labels := map[string]string{
		"test": "test-cc-target-f-sync-webhook",
	}

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("TestCCTargetFSyncWebhookParent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestCCTargetFSyncWebhookChild", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := v1.CompositeHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// As a simple test of request/response content,
		// just create a child with the same name as the parent.
		child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
		child.SetLabels(labels)
		resp := v1.CompositeHookResponse{
			Children: []*unstructured.Unstructured{child},
		}
		return json.Marshal(resp)
	})

	controllerLabels := &map[string]string{"baz": "caz"}
	f.CreateCompositeController("test-cc-target-f-sync-webhook", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), controllerLabels)

	parent := framework.UnstructuredCRD(parentCRD, "test-cc-target-f-sync-webhook")
	unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
	_, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for child object to be created...")
	err = f.Wait(func() (bool, error) {
		_, err := childClient.Namespace(ns).Get(context.TODO(), "test-cc-target-f-sync-webhook", metav1.GetOptions{})
		return err == nil, err
	})
	if err == nil {
		t.Error("didn't expected to find child, controller should not be managed due to labels mismatch")
	}
	if err != nil && !strings.Contains(err.Error(), "timed out waiting for condition") {
		t.Errorf("expected to find error: %q", "timed out waiting for condition")
	}
}
