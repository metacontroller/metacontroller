/*
Copyright 2018 Google Inc.

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

package object

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/diff"
)

func TestAddFinalizer(t *testing.T) {
	table := []struct {
		name, input, want string
	}{
		{
			name: "unset",
			input: `{
				"metadata": {
					"blah": "moo"
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["test"]
				}
			}`,
		},
		{
			name: "empty list",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": []
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["test"]
				}
			}`,
		},
		{
			name: "should append",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two"]
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two", "test"]
				}
			}`,
		},
		{
			name: "already present",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "test", "three"]
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "test", "three"]
				}
			}`,
		},
	}

	for _, tc := range table {
		obj := makeUnstructured(tc.input)
		wantObj := makeUnstructured(tc.want)
		AddFinalizer(obj, "test")

		if got, want := obj.Object, wantObj.Object; !reflect.DeepEqual(got, want) {
			t.Logf("reflect diff: a=got, b=want:\n%s", diff.ObjectReflectDiff(got, want))
			t.Errorf("%v: obj = %#v, want %#v", tc.name, got, want)
		}
	}
}

func TestRemoveFinalizer(t *testing.T) {
	table := []struct {
		name, input, want string
	}{
		{
			name: "unset",
			input: `{
				"metadata": {
					"blah": "moo"
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo"
				}
			}`,
		},
		{
			name: "empty list",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": []
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": []
				}
			}`,
		},
		{
			name: "remove last item",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["test"]
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": []
				}
			}`,
		},
		{
			name: "remove from beginning",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["test", "one", "two"]
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two"]
				}
			}`,
		},
		{
			name: "remove from middle",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "test", "two"]
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two"]
				}
			}`,
		},
		{
			name: "remove from end",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two", "test"]
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two"]
				}
			}`,
		},
		{
			name: "not present",
			input: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two", "three"]
				}
			}`,
			want: `{
				"metadata": {
					"blah": "moo",
					"finalizers": ["one", "two", "three"]
				}
			}`,
		},
	}

	for _, tc := range table {
		obj := makeUnstructured(tc.input)
		wantObj := makeUnstructured(tc.want)
		RemoveFinalizer(obj, "test")

		if got, want := obj.Object, wantObj.Object; !reflect.DeepEqual(got, want) {
			t.Logf("reflect diff: a=got, b=want:\n%s", diff.ObjectReflectDiff(got, want))
			t.Errorf("%v: obj = %#v, want %#v", tc.name, got, want)
		}
	}
}

func makeUnstructured(objJSON string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	if err := json.Unmarshal([]byte(objJSON), &obj); err != nil {
		panic(fmt.Sprintf("can't unmarshal %q: %v", objJSON, err))
	}
	return &unstructured.Unstructured{Object: obj}
}
