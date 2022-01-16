package composite

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/internal/testutils/hooks"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/nsf/jsondiff"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestSyncHookRequest_MarshalJSON(t *testing.T) {
	expected := `
{
  "controller": {
    "metadata": {
      "creationTimestamp": null
    },
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

	children := make(common.RelativeObjectMap)
	parent := v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "some",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	child := &unstructured.Unstructured{}
	child.SetAPIVersion("v1")
	child.SetKind("Pod")
	child.SetName("aaaaa")
	children.Insert(&parent, child)

	request := SyncHookRequest{
		Controller: &v1alpha1.CompositeController{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       v1alpha1.CompositeControllerSpec{},
			Status:     v1alpha1.CompositeControllerStatus{},
		},
		Parent:     &unstructured.Unstructured{},
		Children:   children,
		Related:    make(common.RelativeObjectMap),
		Finalizing: false,
	}

	output, err := json.Marshal(request)

	if err != nil {
		t.Error(err)
		t.Fail()
	}

	diffOpts := jsondiff.DefaultConsoleOptions()
	res, diff := jsondiff.Compare([]byte(expected), output, &diffOpts)

	if res != jsondiff.FullMatch {
		t.Errorf("the expected result is not equal to actual: %s", diff)
	}
}

func TestWhenChildrenArrayIsNullThenDeserializeToEmptySlice(t *testing.T) {
	input := `
{
	"children": [null]
}`
	parentController := parentController{
		syncHook: hooks.NewSerializingExecutorStub(input),
	}
	parent := &unstructured.Unstructured{}
	parent.SetDeletionTimestamp(nil)
	request := SyncHookRequest{Parent: parent}

	response, err := parentController.callHook(&request)

	if err != nil {
		t.Error(err)
		t.Fail()
	}

	if response.Children == nil {
		t.Errorf("Children should not be nil")
	}
}
