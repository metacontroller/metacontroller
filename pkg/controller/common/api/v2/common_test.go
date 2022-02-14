/*
 *
 * Copyright 2022. Metacontroller authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package v2

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	parameters = []struct {
		gvk  GroupVersionKind
		text string
	}{
		{GroupVersionKind{
			schema.GroupVersionKind{
				Group: "", Version: "v1", Kind: "kind"}},
			"kind.v1"},
		{GroupVersionKind{
			schema.GroupVersionKind{
				Group: "someGroup", Version: "v1", Kind: "kind"}},
			"kind.someGroup/v1"},
		{GroupVersionKind{
			schema.GroupVersionKind{
				Group: "apps", Version: "v1", Kind: "StatefulSet"}},
			"StatefulSet.apps/v1"},
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
		gvk := &GroupVersionKind{}
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

func TestUniformObjectMap_relativeName(t *testing.T) {
	unfiromObjectMap := UniformObjectMap{}
	type testcase struct {
		name     string
		obj      *unstructured.Unstructured
		expected string
	}

	testcases := []testcase{
		{
			name:     "namespaced child in same namespace",
			obj:      createUnstructured("some", "object"),
			expected: "some/object",
		},
		{
			name:     "clustered child",
			obj:      createUnstructured("", "object"),
			expected: "object",
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			actual := unfiromObjectMap.qualifiedName(testcase.obj)
			assert.Equal(t, testcase.expected, actual)
		})
	}
}

func createUnstructured(namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}
