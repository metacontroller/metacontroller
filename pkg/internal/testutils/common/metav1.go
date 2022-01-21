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

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewDefaultAPIResource() metav1.APIResource {
	return metav1.APIResource{
		Name:       TestResource,
		Namespaced: true,
		Group:      TestGroup,
		Version:    TestVersion,
		Kind:       TestKind,
	}
}

func NewDefaultStatusAPIResource() metav1.APIResource {
	return metav1.APIResource{
		Name:       TestResourceStatus,
		Namespaced: true,
		Group:      TestGroup,
		Version:    TestVersion,
		Kind:       TestKind,
	}
}

func NewDefaultAPIResourceList() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       TestKind,
				APIVersion: TestAPIVersion,
			},
			GroupVersion: fmt.Sprintf("%s/%s", TestGroup, TestVersion),
			APIResources: []metav1.APIResource{
				NewDefaultAPIResource(),
			},
		},
	}
}

func NewDefaultStatusAPIResourceList() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       TestKind,
				APIVersion: TestAPIVersion,
			},
			GroupVersion: fmt.Sprintf("%s/%s", TestGroup, TestVersion),
			APIResources: []metav1.APIResource{
				NewDefaultAPIResource(),
				NewDefaultStatusAPIResource(),
			},
		},
	}
}
