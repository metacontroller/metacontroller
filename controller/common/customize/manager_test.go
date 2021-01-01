package customize

import (
	"encoding/json"
	"reflect"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1alpha1 "metacontroller.io/apis/metacontroller/v1alpha1"
	"metacontroller.io/controller/common"
	dynamicclientset "metacontroller.io/dynamic/clientset"
	dynamicinformer "metacontroller.io/dynamic/informer"
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
