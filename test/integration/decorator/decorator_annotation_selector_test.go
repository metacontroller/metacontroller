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
	customizev1 "metacontroller/pkg/controller/common/customize/api/v1"
	v1 "metacontroller/pkg/controller/decorator/api/v1"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/test/integration/framework"
)

// TestCustomizeWebhookRespectsAnnotationSelector makes sure that a change of
// a related resource causes a customize webhook call only for parents that
// satisfy the annotationSelector.
func TestCustomizeWebhookRespectsAnnotationSelector(t *testing.T) {
	namespace := "test-customize-webhook-annotation-selector"
	relatedResourceName := "related-config-map"
	matchingParentName := "matching-parent"
	nonMatchingParentName := "non-matching-parent"
	updatedDataKey := "updated"

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(namespace)
	parentCRD, parentClient := f.CreateCRD("TestAnnotationSelectorParent", apiextensions.NamespaceScoped)
	childCRD, childClient := f.CreateCRD("TestAnnotationSelectorChild", apiextensions.NamespaceScoped)

	var mu sync.Mutex
	var customizedParents []string
	var syncedUpdatedRelated bool

	customizeHook := f.ServeWebhook(func(body []byte) ([]byte, error) {
		type decoratorCustomizeRequest struct {
			Parent unstructured.Unstructured `json:"parent"`
		}
		req := decoratorCustomizeRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}

		mu.Lock()
		customizedParents = append(customizedParents, req.Parent.GetName())
		mu.Unlock()

		resp := customizev1.CustomizeHookResponse{
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
		req := v1.DecoratorHookRequest{}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		var attachments []*unstructured.Unstructured
		if len(req.Related.List()) == 0 {
			attachments = make([]*unstructured.Unstructured, 0)
		} else {
			related := req.Related.List()[0]
			if data, _, _ := unstructured.NestedStringMap(related.Object, "data"); data[updatedDataKey] == "true" {
				mu.Lock()
				syncedUpdatedRelated = true
				mu.Unlock()
			}
			child := framework.UnstructuredCRD(childCRD, req.Object.GetName()+"-"+related.GetName())
			attachments = []*unstructured.Unstructured{child}
		}

		resp := v1.DecoratorHookResponse{
			Attachments: attachments,
		}
		return json.Marshal(resp)
	})

	f.CreateDecoratorControllerWithAnnotationSelector(
		"dc-annotation-selector",
		syncHook.URL,
		customizeHook.URL,
		&v1alpha1.AnnotationSelector{
			MatchAnnotations: map[string]string{"transform": "true"},
		},
		framework.CRDResourceRule(parentCRD),
		framework.CRDResourceRule(childCRD),
	)

	// This parent does not have the annotation. The controller must not call
	// the customize webhook for it. Create it before the matching parent. The
	// informer gets the events in the order of creation. Thus this parent is
	// in the informer cache when the matching parent is there.
	nonMatchingParent := framework.UnstructuredCRD(parentCRD, nonMatchingParentName)
	if _, err := parentClient.Namespace(namespace).Create(context.TODO(), nonMatchingParent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	matchingParent := framework.UnstructuredCRD(parentCRD, matchingParentName)
	matchingParent.SetAnnotations(map[string]string{"transform": "true"})
	if _, err := parentClient.Namespace(namespace).Create(context.TODO(), matchingParent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	// The creation of the related ConfigMap starts findRelatedParents.
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
	}
	createdConfigMap, err := relatedClient.Create(context.TODO(), &relatedConfigMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for matching parent's child to be created...")
	err = f.Wait(func() (bool, error) {
		_, err := childClient.Namespace(namespace).Get(context.TODO(), matchingParentName+"-"+relatedResourceName, metav1.GetOptions{})
		return err == nil, err
	})
	if err != nil {
		t.Fatalf("didn't find expected child for matching parent: %v", err)
	}

	// The child shows that the matching parent is in the informer cache. The
	// non-matching parent is older, thus it is in the cache too. Update the
	// related ConfigMap to start findRelatedParents again. findRelatedParents calls
	// the customize webhook for all parents in one pass, before it enqueues
	// the matching parent. Thus the pass is complete when the sync webhook
	// gets the updated ConfigMap.
	createdConfigMap.Data = map[string]string{updatedDataKey: "true"}
	if _, err := relatedClient.Update(context.TODO(), createdConfigMap, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for sync with updated related object...")
	err = f.Wait(func() (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		return syncedUpdatedRelated, nil
	})
	if err != nil {
		t.Fatalf("sync webhook didn't get the updated related object: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, name := range customizedParents {
		if name == nonMatchingParentName {
			t.Errorf("customize webhook was called for parent %q, which doesn't satisfy the annotationSelector", nonMatchingParentName)
		}
	}
}
