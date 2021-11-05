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

package common

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	TestGroup          = "testgroup"
	TestVersion        = "testversion"
	TestResource       = "testkinds"
	TestResourceStatus = "testkinds/status"
	TestResourceList   = "TestkindsList"
	TestNamespace      = "testns"
	TestName           = "testname"
	TestKind           = "TestKind"
	TestAPIVersion     = "testgroup/testversion"
)

func NewDefaultUnstructured() *unstructured.Unstructured {
	return NewUnstructured(TestAPIVersion, TestKind, TestNamespace, TestName)
}

func NewUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}
