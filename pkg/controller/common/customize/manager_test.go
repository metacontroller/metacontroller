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
	"fmt"
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"reflect"
	"testing"

	"github.com/go-logr/logr/funcr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicinformer "metacontroller/pkg/dynamic/informer"

	. "metacontroller/pkg/internal/testutils/hooks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var fakeEnqueueParent = func(obj interface{}) {}
var dynClient = dynamicclientset.Clientset{}
var dynInformers = dynamicinformer.SharedInformerFactory{}

var fakeLogger = funcr.New(
	func(pfx, args string) { fmt.Println(pfx, args) },
	funcr.Options{
		LogCaller:    funcr.All,
		LogTimestamp: true,
	})

var customizeManagerWithNilController, _ = NewCustomizeManager(
	"test",
	fakeEnqueueParent,
	&NilCustomizableController{},
	&dynClient,
	&dynInformers,
	make(common.InformerMap),
	make(common.GroupKindMap),
	fakeLogger,
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
	fakeLogger,
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
	expectedResponse := &v1.CustomizeHookResponse{
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{{
			ResourceRule: v1alpha1.ResourceRule{
				APIVersion: "some",
				Resource:   "some",
			},
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
			Namespace:     "Namespace",
			Names:         []string{"name"},
		}},
	}

	customizeManagerWithFakeController.customizeHook = NewHookExecutorStub(expectedResponse)
	parent := &unstructured.Unstructured{}
	parent.SetUID("some")
	parent.SetGeneration(1)

	response, err := customizeManagerWithFakeController.getCustomizeHookResponse(parent)

	if err != nil {
		t.Errorf("Incorrect invocation, err should be nil, got: %v", err)
	}

	if !reflect.DeepEqual(*response, *expectedResponse) {
		t.Errorf("Response should be equal to %v, got %v", expectedResponse, response)
	}

	if _, found := customizeManagerWithFakeController.customizeCache.Get(customizeKey{"some", 1}); !found {
		t.Error("Expected not nil here, response should be cached")
	}
}

func TestDetermineSelectionType_returnErrorWhenLabelSelectorAndNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
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
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
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
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
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
