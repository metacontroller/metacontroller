package common

import (
	"metacontroller/pkg/dynamic/apply"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNullifyLastAppliedAnnotation_DoNothingIfNoAnnotations(t *testing.T) {
	object := &unstructured.Unstructured{}

	nullifyLastAppliedAnnotation(object)

	if object.GetAnnotations() != nil {
		t.Logf("Annotations should be nil, but has %#v", object.GetAnnotations())
		t.Fail()
	}
}

func TestNullifyLastAppliedAnnotation_DoNothingIfLastAppliedNotPresentAnnotations(t *testing.T) {
	object := &unstructured.Unstructured{}
	emptyMap := make(map[string]string)
	object.SetAnnotations(emptyMap)

	nullifyLastAppliedAnnotation(object)

	if value, found := object.GetAnnotations()[apply.LastAppliedAnnotation]; found {
		t.Logf("Annotations should not be found, but has %s", value)
		t.Fail()
	}
}

func TestNullifyLastAppliedAnnotation_NullifyIfPresent(t *testing.T) {
	object := &unstructured.Unstructured{}
	annotationsWithLastApplied := make(map[string]string)
	annotationsWithLastApplied[apply.LastAppliedAnnotation] = "someValue"
	object.SetAnnotations(annotationsWithLastApplied)

	nullifyLastAppliedAnnotation(object)

	if object.GetAnnotations()[apply.LastAppliedAnnotation] != "" {
		t.Logf("Annotations should be '', but is %s", object.GetAnnotations()[apply.LastAppliedAnnotation])
		t.Fail()
	}
}
