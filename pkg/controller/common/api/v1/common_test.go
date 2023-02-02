/*
Copyright 2022 Metacontroller authors.

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

package v1

import (
	api "metacontroller/pkg/controller/common/api"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	parameters = []struct {
		gvk  api.GroupVersionKind
		text string
	}{
		{gvk: api.GroupVersionKind{
			GroupVersionKind: schema.GroupVersionKind{
				Group: "", Version: "v1", Kind: "kind"}},
			text: "kind.v1"},
		{gvk: api.GroupVersionKind{
			GroupVersionKind: schema.GroupVersionKind{
				Group: "someGroup", Version: "v1", Kind: "kind"}},
			text: "kind.someGroup/v1"},
		{gvk: api.GroupVersionKind{
			GroupVersionKind: schema.GroupVersionKind{
				Group: "apps", Version: "v1", Kind: "StatefulSet"}},
			text: "StatefulSet.apps/v1"},
	}
)

func TestGroupVersionKind_MarshalText(t *testing.T) {
	for i := range parameters {
		actual, err := parameters[i].gvk.MarshalText()
		if string(actual) != parameters[i].text || err != nil {
			t.Logf("expected: [%s], actual: [%s]", parameters[i].text, string(actual))
			t.Fail()
		}
	}
}

func TestGroupVersionKind_UnmarshalText(t *testing.T) {
	for i := range parameters {
		gvk := &api.GroupVersionKind{}
		err := gvk.UnmarshalText([]byte(parameters[i].text))
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(*gvk, parameters[i].gvk) || err != nil {
			t.Logf("expected: [%s], actual: [%s]", parameters[i].gvk, gvk)
			t.Fail()
		}
	}
}

func TestGroupVersionKind_UnmrshalTextWithException(t *testing.T) {
	wrongGvk := []string{"wrongOne", "wrong.//"}
	for i := range wrongGvk {
		gvk := &api.GroupVersionKind{}
		err := gvk.UnmarshalText([]byte(wrongGvk[i]))
		if err == nil {
			t.Logf("expected exception but not thrown for [%s]", wrongGvk[i])
			t.Fail()
		}
	}
}

func TestRelativeObjectMap_InitGroup_ShouldInitializeNilGroup(t *testing.T) {
	underTest := make(RelativeObjectMap)
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}

	underTest.InitGroup(gvk)

	internalGvk := api.GroupVersionKind{GroupVersionKind: gvk}
	if underTest[internalGvk] == nil {
		t.Logf("%s should not be nil after initialization", internalGvk)
		t.Fail()
	}
}

func TestRelativeObjectMap_InitGroup_ShouldNotOverrideNonNilGroup(t *testing.T) {
	underTest := make(RelativeObjectMap)
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}
	internalGvk := api.GroupVersionKind{GroupVersionKind: gvk}
	expectedGroup := make(map[string]*unstructured.Unstructured)
	expectedGroup["test"] = &unstructured.Unstructured{}
	underTest[internalGvk] = expectedGroup

	underTest.InitGroup(gvk)

	if !reflect.DeepEqual(underTest[internalGvk], expectedGroup) {
		t.Logf("Group has ben replaced")
		t.Fail()
	}
}

func TestRelativeObjectMap_RelativeName_SameNamespace(t *testing.T) {
	parent := corev1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ss",
		},
		Spec:   corev1.PodSpec{},
		Status: corev1.PodStatus{},
	}
	object := &unstructured.Unstructured{}
	object.SetNamespace("ss")
	object.SetName("other")

	relativeStr := relativeName(&parent, object)

	if relativeStr != "other" {
		t.Logf("Expected relative name to be %s, but is %s", "other", relativeStr)
		t.Fail()
	}
}

func TestRelativeObjectMap_RelativeName_ClusteredParent(t *testing.T) {
	parent := corev1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
		},
		Spec:   corev1.PodSpec{},
		Status: corev1.PodStatus{},
	}
	object := &unstructured.Unstructured{}
	object.SetNamespace("some")
	object.SetName("other")

	relativeStr := relativeName(&parent, object)

	if relativeStr != "some/other" {
		t.Logf("Expected relative name to be %s, but is %s", "some/other", relativeStr)
		t.Fail()
	}
}

func TestRelativeObjectMap_RelativeName_BothClustered(t *testing.T) {
	parent := corev1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
		},
		Spec:   corev1.PodSpec{},
		Status: corev1.PodStatus{},
	}
	object := &unstructured.Unstructured{}
	object.SetNamespace("")
	object.SetName("other")

	relativeStr := relativeName(&parent, object)

	if relativeStr != "other" {
		t.Logf("Expected relative name to be %s, but is %s", "other", relativeStr)
		t.Fail()
	}
}
