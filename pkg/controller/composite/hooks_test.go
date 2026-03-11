package composite

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	commonv1 "metacontroller/pkg/controller/common/api/v1"
	commonv2 "metacontroller/pkg/controller/common/api/v2"
	composite "metacontroller/pkg/controller/composite/api/v1"
	v2 "metacontroller/pkg/controller/composite/api/v2"
	"metacontroller/pkg/internal/testutils/hooks"
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestSyncHookRequest_MarshalJSON(t *testing.T) {
	expected := `
{
  "controller": {
    "metadata": {},
    "spec": {
      "parentResource": {
        "apiVersion": "",
        "resource": ""
      }
    },
    "status": {}
  },
  "parent": null,
  "children": {
    "Pod.v1": {
        "aaaaa": {
            "metadata": {
                "name": "aaaaa"
            },
            "apiVersion": "v1",
            "kind": "Pod"
        }
    }
  },
  "related": {},
  "finalizing": false
}`

	children := make(commonv1.RelativeObjectMap)
	parent := corev1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "some",
		},
		Spec:   corev1.PodSpec{},
		Status: corev1.PodStatus{},
	}

	child := &unstructured.Unstructured{}
	child.SetAPIVersion("v1")
	child.SetKind("Pod")
	child.SetName("aaaaa")
	children.Insert(&parent, child)

	request := composite.CompositeHookRequest{
		Controller: &v1alpha1.CompositeController{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       v1alpha1.CompositeControllerSpec{},
			Status:     v1alpha1.CompositeControllerStatus{},
		},
		Parent:     &unstructured.Unstructured{},
		Children:   children,
		Related:    make(commonv1.RelativeObjectMap),
		Finalizing: false,
	}

	output, err := json.Marshal(request)

	if err != nil {
		t.Error(err)
		t.Fail()
	}

	assert.JSONEq(t, expected, string(output))
}

func TestWhenChildrenArrayIsNullThenDeserializeToEmptySlice(t *testing.T) {
	input := `
{
	"children": [null]
}`
	parentController := parentController{
		syncHook:     hooks.NewSerializingExecutorStub(input),
		finalizeHook: hooks.NewDisabledExecutorStub(),
	}
	parent := &unstructured.Unstructured{}
	parent.SetNamespace("some")
	parent.SetDeletionTimestamp(nil)

	response, err := parentController.callHook(parent, make(commonv2.UniformObjectMap), make(commonv2.UniformObjectMap))

	if err != nil {
		t.Error(err)
		t.Fail()
	}

	if response == nil || response.Children == nil {
		t.Errorf("Children should not be nil")
	}
}

func TestWhenChildrenArrayHasInvalidValueShouldFail(t *testing.T) {
	input := `
{
	"children": [None]
}`
	parentController := parentController{
		syncHook:     hooks.NewSerializingExecutorStub(input),
		finalizeHook: hooks.NewDisabledExecutorStub(),
	}
	parent := &unstructured.Unstructured{}
	parent.SetNamespace("some")
	parent.SetDeletionTimestamp(nil)

	response, err := parentController.callHook(parent, make(commonv2.UniformObjectMap), make(commonv2.UniformObjectMap))

	if err == nil {
		t.Error("Should fail with invalid input")
	}
	if response != nil {
		t.Error("Response should be nil due to error raised")
	}
}

func TestWhenChildrenArrayHasInvalidJSONValueShouldFail(t *testing.T) {
	input := `
{
	"children": {None}
}`
	parentController := parentController{
		syncHook:     hooks.NewSerializingExecutorStub(input),
		finalizeHook: hooks.NewDisabledExecutorStub(),
	}
	parent := &unstructured.Unstructured{}
	parent.SetNamespace("some")
	parent.SetDeletionTimestamp(nil)

	response, err := parentController.callHook(parent, make(commonv2.UniformObjectMap), make(commonv2.UniformObjectMap))

	if err == nil {
		t.Error("Should fail with invalid input")
	}
	if response != nil {
		t.Error("Response should be nil due to error raised")
	}
}

func TestConvertV2ToV1Response(t *testing.T) {
	pc := &parentController{}
	v2Response := v2.CompositeHookResponse{
		Status: map[string]interface{}{
			"foo": "bar",
		},
		Children: []*unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "child",
					},
				},
			},
		},
		ResyncAfterSeconds: 10.5,
		Finalized:          true,
	}

	v1Response := pc.convertV2ToV1Response(v2Response)

	assert.Equal(t, v2Response.Status, v1Response.Status)
	assert.Equal(t, v2Response.Children, v1Response.Children)
	assert.Equal(t, v2Response.ResyncAfterSeconds, v1Response.ResyncAfterSeconds)
	assert.Equal(t, v2Response.Finalized, v1Response.Finalized)
}
