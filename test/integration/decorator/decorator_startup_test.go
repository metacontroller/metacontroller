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
	"sync/atomic"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"

	v1 "metacontroller/pkg/controller/decorator/api/v1"
	"metacontroller/test/integration/framework"
)

// The attachment handler of a controller ignores the objects that the
// informer replays at start. This test shows that the first sync of a parent
// still gets an attachment that exists before the controller starts.
func TestControllerStart_whenAttachmentExistsBeforeController_passesItToTheSyncHook(t *testing.T) {
	ns := "test-start-existing-attachment"
	childName := "pre-existing-attachment"
	f := framework.NewFixture(t)
	defer f.TearDown()
	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("TestStartAttachmentParent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestStartAttachmentChild", apiextensions.NamespaceScoped)
	parent, err := parentClient.Namespace(ns).Create(context.TODO(), framework.UnstructuredCRD(parentCRD, "parent"), metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	child := framework.UnstructuredCRD(childCRD, childName)
	// The controller takes only the attachments that have its name in this
	// annotation.
	child.SetAnnotations(map[string]string{"metacontroller.k8s.io/decorator-controller": ns})
	child.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion:         parent.GetAPIVersion(),
		Kind:               parent.GetKind(),
		Name:               parent.GetName(),
		UID:                parent.GetUID(),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}})
	if _, err := childClient.Namespace(ns).Create(context.TODO(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	var sawAttachment atomic.Bool
	hook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		req := v1.DecoratorHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		// Ask for the same attachments, so the controller keeps them.
		desired := []*unstructured.Unstructured{}
		for _, attachment := range req.Attachments.List() {
			if attachment.GetName() == childName {
				sawAttachment.Store(true)
			}
			desired = append(desired, framework.UnstructuredCRD(childCRD, attachment.GetName()))
		}
		return json.Marshal(v1.DecoratorHookResponse{Attachments: desired})
	})

	f.CreateDecoratorController(ns, hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), nil)

	if err := f.Wait(func() (bool, error) { return sawAttachment.Load(), nil }); err != nil {
		t.Fatalf("the sync hook did not get the attachment that existed before the controller: %v", err)
	}
}
