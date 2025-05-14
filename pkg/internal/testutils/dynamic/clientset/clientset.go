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

package clientset

import (
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	common "metacontroller/pkg/internal/testutils/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func NewFakeNewSimpleClientsetWithResources(apiResourceList []*metav1.APIResourceList) *fakeclientset.Clientset {
	simpleClientset := fakeclientset.NewSimpleClientset(common.NewDefaultUnstructured())
	simpleClientset.Resources = apiResourceList
	return simpleClientset
}

func NewClientset(restConfig *rest.Config, resourceMap *dynamicdiscovery.ResourceMap, dc dynamic.Interface) *dynamicclientset.Clientset {
	return dynamicclientset.NewClientset(
		restConfig,
		resourceMap,
		dc,
	)
}
