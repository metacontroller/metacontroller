/*
Copyright 2021 Metacontroller authors.

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

package customize

import (
	"reflect"
	"testing"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	"metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"

	. "metacontroller/pkg/internal/testutils/hooks"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var fakeEnqueueParent = func(obj interface{}) {}
var dynClient = dynamicclientset.Clientset{}
var dynInformers = dynamicinformer.SharedInformerFactory{}

type nilCustomizableController struct {
}

func (cc *nilCustomizableController) GetCustomizeHook() *v1alpha1.Hook {
	return nil
}

type fakeCustomizableController struct {
}

func (cc *fakeCustomizableController) GetCustomizeHook() *v1alpha1.Hook {
	url := "fake"
	return &v1alpha1.Hook{
		Webhook: &v1alpha1.Webhook{
			URL: &url,
		},
	}
}

var neGroupKindMap = common.GroupKindMap{
	schema.GroupKind{
		Group: "randomGroup",
		Kind:  "ingress",
	}: &discovery.APIResource{
		APIVersion:  "v1alpha1",
		APIResource: v1.APIResource{Namespaced: true, Group: "randomGroup"},
	},
}

var customizeManagerWithNilController, _ = NewCustomizeManager(
	"test",
	fakeEnqueueParent,
	&nilCustomizableController{},
	&dynClient,
	&dynInformers,
	make(common.InformerMap),
	make(common.GroupKindMap),
	nil,
	common.CompositeController,
)

var customizeManagerWithFakeController, _ = NewCustomizeManager(
	"test",
	fakeEnqueueParent,
	&FakeCustomizableController{},
	&dynClient,
	&dynInformers,
	make(common.InformerMap),
	make(common.GroupKindMap),
	nil,
	common.DecoratorController,
)

var customizeManagerWithFakeControllerAndGroupKindMap, _ = NewCustomizeManager(
	"test",
	fakeEnqueueParent,
	&fakeCustomizableController{},
	&dynClient,
	&dynInformers,
	make(common.InformerMap),
	neGroupKindMap,
	nil,
	common.DecoratorController,
)

func TestGetRelatedObjects_whenHookDisabled_returnEmptyMap(t *testing.T) {
	parent := &unstructured.Unstructured{}
	parent.SetName("test")
	parent.SetGeneration(1)

	relatedObjects, err := customizeManagerWithNilController.GetRelatedObjects(parent)

	if err != nil {
		t.Errorf("Incorrect invocation, err should be nil, got: %v", err)
	}

	if len(relatedObjects) != 0 {
		t.Errorf("Expected empty map, got %v", relatedObjects)
	}
}

func TestGetRelatedObject_requestResponse(t *testing.T) {
	expectedResponse := &CustomizeHookResponse{
		[]*v1alpha1.RelatedResourceRule{{
			ResourceRule: v1alpha1.ResourceRule{
				APIVersion: "some",
				Resource:   "some",
			},
			LabelSelector: &v1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
			Namespace:     "Namespace",
			Names:         []string{"name"},
		}},
	}

	customizeManagerWithFakeController.customizeHook = NewHookExecutorStub(expectedResponse)
	parent := &unstructured.Unstructured{}
	parent.SetName("othertest")
	parent.SetGeneration(1)

	response, err := customizeManagerWithFakeController.getCustomizeHookResponse(parent)

	if err != nil {
		t.Errorf("Incorrect invocation, err should be nil, got: %v", err)
	}

	if !reflect.DeepEqual(*response, *expectedResponse) {
		t.Errorf("Response should be equal to %v, got %v", expectedResponse, response)
	}

	if customizeManagerWithFakeController.customizeCache.Get("othertest", 1) == nil {
		t.Error("Expected not nil here, response should be cached")
	}
}

func TestDetermineSelectionType_returnErrorWhenLabelSelectorAndNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: &v1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
		Namespace:     "Namespace",
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != invalid && err == nil {
		t.Errorf("Expected error and 'invalid' selection type, but got %v", selectionType)
	}
}

func TestDetermineSelectionType_returnErrorWhenLabelSelectorAndNameIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: &v1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
		Names:         []string{"name"},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != invalid && err == nil {
		t.Errorf("Expected error and 'invalid' selection type, but got %v", selectionType)
	}
}

func TestDetermineSelectionType_returnLabelSelectorWhenPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: &v1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByLabels, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceAndNamesWhenNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
		Namespace:     "some",
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndNames || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndNames, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceAndNamesWhenNamesIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
		Names:         []string{"name"},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndNames || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndNames, selectionType)
	}
}

func TestDetermineSelectionType_returnLabelsWhenNothingIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByLabels, selectionType)
	}
}

func TestMatchesRelatedRule_nonMatchingNamespaceAndRuleShouldThrowError(t *testing.T) {

	parent := new(unstructured.Unstructured)
	related := new(unstructured.Unstructured)
	parent.SetAPIVersion("v1alpha1")
	parent.SetNamespace("istio-system")
	related.SetNamespace("istio-system")
	groupVersionKind := schema.GroupVersionKind{
		Group:   "randomGroup",
		Version: "v1alpha1",
		Kind:    "ingress",
	}
	parent.SetGroupVersionKind(groupVersionKind)

	relatedRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
		Namespace:     "other-namespace",
		Names:         []string{"some", "test"},
	}

	isMatching, err := customizeManagerWithFakeControllerAndGroupKindMap.matchesRelatedRule(parent, related, &relatedRule)
	if isMatching || err == nil {
		t.Errorf("Expected an error, but got none")
	}
}

func TestMatchesRelatedRule_nonMatchingNamespaceAndRuleShouldBeOkCaseWildcard(t *testing.T) {

	parent := new(unstructured.Unstructured)
	related := new(unstructured.Unstructured)
	parent.SetAPIVersion("v1alpha1")
	parent.SetNamespace("istio-system")

	groupVersionKind := schema.GroupVersionKind{
		Group:   "randomGroup",
		Version: "v1alpha1",
		Kind:    "ingress",
	}
	parent.SetGroupVersionKind(groupVersionKind)

	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
		Namespace:     "*",
		Names:         []string{"some", "test"},
	}

	isMatching, err := customizeManagerWithFakeControllerAndGroupKindMap.matchesRelatedRule(parent, related, &resourceRule)
	if !isMatching || err != nil {
		t.Errorf("Expected no error, but got %v", err.Error())
	}
}

func TestMatchesRelatedRule_nonMatchingNamespaceAndRuleShouldBeOkCaseNoNamesGiven(t *testing.T) {

	parent := new(unstructured.Unstructured)
	related := new(unstructured.Unstructured)
	parent.SetAPIVersion("v1alpha1")
	parent.SetNamespace("istio-system")

	groupVersionKind := schema.GroupVersionKind{
		Group:   "randomGroup",
		Version: "v1alpha1",
		Kind:    "ingress",
	}
	parent.SetGroupVersionKind(groupVersionKind)

	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
		Namespace:     "my-namespace",
		Names:         []string{},
	}

	isMatching, err := customizeManagerWithFakeControllerAndGroupKindMap.matchesRelatedRule(parent, related, &resourceRule)
	if !isMatching || err != nil {
		t.Errorf("Expected no error, but got %v", err.Error())
	}
}
