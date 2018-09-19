package common

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestRevertObjectMetaSystemFields(t *testing.T) {
	origJSON := `{
		"metadata": {
			"origMeta": "should stay gone",
			"otherMeta": "should change value",
			"creationTimestamp": "should restore orig value",
			"deletionTimestamp": "should restore orig value",
			"uid": "should bring back removed value"
		},
		"other": "should change value"
	}`
	newObjJSON := `{
		"metadata": {
			"creationTimestamp": null,
			"deletionTimestamp": "new value",
			"newMeta": "new value",
			"otherMeta": "new value",
			"selfLink": "should be removed"
		},
		"other": "new value"
	}`
	wantJSON := `{
		"metadata": {
			"otherMeta": "new value",
			"newMeta": "new value",
			"creationTimestamp": "should restore orig value",
			"deletionTimestamp": "should restore orig value",
			"uid": "should bring back removed value"
		},
		"other": "new value"
	}`

	orig := make(map[string]interface{})
	if err := json.Unmarshal([]byte(origJSON), &orig); err != nil {
		t.Fatalf("can't unmarshal orig: %v", err)
	}
	newObj := make(map[string]interface{})
	if err := json.Unmarshal([]byte(newObjJSON), &newObj); err != nil {
		t.Fatalf("can't unmarshal newObj: %v", err)
	}
	want := make(map[string]interface{})
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("can't unmarshal want: %v", err)
	}

	err := revertObjectMetaSystemFields(&unstructured.Unstructured{Object: newObj}, &unstructured.Unstructured{Object: orig})
	if err != nil {
		t.Fatalf("revertObjectMetaSystemFields error: %v", err)
	}

	if got := newObj; !reflect.DeepEqual(got, want) {
		t.Logf("reflect diff: a=got, b=want:\n%s", diff.ObjectReflectDiff(got, want))
		t.Fatalf("revertObjectMetaSystemFields() = %#v, want %#v", got, want)
	}
}
