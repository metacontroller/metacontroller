/*
Copyright 2026 Metacontroller authors.

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
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	common "metacontroller/pkg/internal/testutils/common"
	"metacontroller/pkg/internal/testutils/dynamic/discovery"
)

// NewFakeClients returns a dynamic clientset, its fake API server, and a
// discovery map. The map lists the namespaced resources of
// common.TestAPIVersion. The kinds map gives the kind of each resource. The
// fake server starts with the given objects.
func NewFakeClients(kinds map[string]string, objects ...*unstructured.Unstructured) (*dynamicclientset.Clientset, *dynamicfake.FakeDynamicClient, *dynamicdiscovery.ResourceMap) {
	names := make([]string, 0, len(kinds))
	for name := range kinds {
		names = append(names, name)
	}
	sort.Strings(names)

	listKinds := map[schema.GroupVersionResource]string{}
	apiResources := make([]metav1.APIResource, 0, len(names))
	for _, name := range names {
		gvr := schema.GroupVersionResource{Group: common.TestGroup, Version: common.TestVersion, Resource: name}
		listKinds[gvr] = kinds[name] + "List"
		apiResources = append(apiResources, metav1.APIResource{
			Name:       name,
			Namespaced: true,
			Group:      common.TestGroup,
			Version:    common.TestVersion,
			Kind:       kinds[name],
		})
	}
	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, object := range objects {
		runtimeObjects = append(runtimeObjects, object)
	}
	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, runtimeObjects...)
	resourceList := []*metav1.APIResourceList{{GroupVersion: common.TestAPIVersion, APIResources: apiResources}}
	resourceMap := discovery.NewFakeResourceMap(NewFakeClientsetWithResources(resourceList))
	return NewClientset(common.NewDefaultRestConfig(), resourceMap, fakeClient), fakeClient, resourceMap
}
