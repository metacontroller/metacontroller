package customize

import (
	"encoding/json"
	"reflect"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1alpha1 "metacontroller.io/pkg/apis/metacontroller.io/v1alpha1"
	"metacontroller.io/pkg/controller/common"
	dynamicclientset "metacontroller.io/pkg/dynamic/clientset"
	dynamicinformer "metacontroller.io/pkg/dynamic/informer"
)

var fakeEnqueueParent func(interface{}) = func(obj interface{}) {}
var dynClient = dynamicclientset.Clientset{}
var dynInformers = dynamicinformer.SharedInformerFactory{}

type nilCustomizableController struct {
}

func (cc *nilCustomizableController) GetCustomizeHook() *v1alpha1.Hook {
	return nil
}

var fakeWebhook = v1alpha1.Webhook{}
var fakeHook = v1alpha1.Hook{Webhook: &fakeWebhook}

type fakeCustomizableController struct {
}

func (cc *fakeCustomizableController) GetCustomizeHook() *v1alpha1.Hook {
	return &fakeHook
}

var customizeManagerWithNilController = NewCustomizeManager("test",
	fakeEnqueueParent,
	&nilCustomizableController{},
	&dynClient,
	&dynInformers,
	make(common.InformerMap),
	make(common.GroupKindMap),
)

var customizeManagerWithFakeController = NewCustomizeManager("test",
	fakeEnqueueParent,
	&fakeCustomizableController{},
	&dynClient,
	&dynInformers,
	make(common.InformerMap),
	make(common.GroupKindMap),
)

func TestGetCustomizeHookResponse_returnNilRelatedResourceRulesIfHookNotSet(t *testing.T) {
	parent := &unstructured.Unstructured{}
	parent.SetName("test")
	parent.SetGeneration(1)

	response, _ := customizeManagerWithNilController.GetCustomizeHookResponse(parent)

	if response.RelatedResourceRules != nil {
		t.Errorf("Incorrect response, should be nil, got: %v", response)
	}
}

func TestGetCustomizeHookResponse_returnErrWhenHookIsInvalid(t *testing.T) {
	parent := &unstructured.Unstructured{}
	parent.SetName("test")
	parent.SetGeneration(1)

	response, err := customizeManagerWithFakeController.GetCustomizeHookResponse(parent)

	if response != nil && err == nil {
		t.Errorf("Incorrect invocation, response should be nil, got: %v, error is nil", response)
	}
}

func TestGetCustomizeHookResponse_returnResponse(t *testing.T) {
	originalCallCustomizeHook := callCustomizeHook
	defer func() { callCustomizeHook = originalCallCustomizeHook }()
	expectedResponse := CustomizeHookResponse{
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
	callCustomizeHook = func(hook *v1alpha1.Hook, request, response interface{}) error {
		byteArray, _ := json.Marshal(expectedResponse)
		json.Unmarshal(byteArray, response)
		return nil
	}

	parent := &unstructured.Unstructured{}
	parent.SetName("othertest")
	parent.SetGeneration(1)

	response, err := customizeManagerWithFakeController.GetCustomizeHookResponse(parent)

	if err != nil {
		t.Errorf("Incorrect invocation, err should be nil, got: %v", err)
	}

	if !reflect.DeepEqual(*response, expectedResponse) {
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
