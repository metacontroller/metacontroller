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

package common

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	dynamicdiscovery "metacontroller.io/dynamic/discovery"
	dynamicinformer "metacontroller.io/dynamic/informer"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type ChildMap map[string]map[string]*unstructured.Unstructured

func (m ChildMap) InitGroup(apiVersion, kind string) {
	m[childMapKey(apiVersion, kind)] = make(map[string]*unstructured.Unstructured)
}

func (m ChildMap) Insert(parent metav1.Object, obj *unstructured.Unstructured) {
	key := childMapKey(obj.GetAPIVersion(), obj.GetKind())
	group := m[key]
	if group == nil {
		group = make(map[string]*unstructured.Unstructured)
		m[key] = group
	}
	name := relativeName(parent, obj)
	group[name] = obj
}

func (m ChildMap) InsertAll(parent metav1.Object, objects []*unstructured.Unstructured) {
	for _, object := range objects {
		m.Insert(parent, object)
	}
}

func (m ChildMap) FindGroupKindName(apiGroup, kind, name string) *unstructured.Unstructured {
	// The map is keyed by group-version-kind, but we don't know the version.
	// So, check inside any GVK that matches the group and kind, ignoring version.
	for key, children := range m {
		if gv, k := ParseChildMapKey(key); k == kind {
			if g, _ := ParseAPIVersion(gv); g == apiGroup {
				for n, child := range children {
					if n == name {
						return child
					}
				}
			}
		}
	}
	return nil
}

// relativeName returns the name of the child relative to the parent.
// If the parent is cluster scoped and the child namespaced scoped the
// name is of the format <namespace>/<name>. Otherwise the name of the child
// is returned.
func relativeName(parent metav1.Object, child *unstructured.Unstructured) string {
	if parent.GetNamespace() == "" && child.GetNamespace() != "" {
		return fmt.Sprintf("%s/%s", child.GetNamespace(), child.GetName())
	}
	return child.GetName()
}

// describeObject returns a human-readable string to identify a given object.
func describeObject(obj *unstructured.Unstructured) string {
	if ns := obj.GetNamespace(); ns != "" {
		return fmt.Sprintf("%s %s/%s", obj.GetKind(), ns, obj.GetName())
	}
	return fmt.Sprintf("%s %s", obj.GetKind(), obj.GetName())
}

// ReplaceChild replaces the child object with the same name & namespace as
// the given child with the contents of the given child. If no child exists
// in the existing map then no action is taken.
func (m ChildMap) ReplaceChild(parent metav1.Object, child *unstructured.Unstructured) {
	key := childMapKey(child.GetAPIVersion(), child.GetKind())
	children := m[key]
	if children == nil {
		// We only want to replace if it already exists, so do nothing.
		return
	}

	name := relativeName(parent, child)
	if _, found := children[name]; found {
		children[name] = child
	}
}

// List expands the ChildMap into a flat list of child objects, in random order.
func (m ChildMap) List() []*unstructured.Unstructured {
	var list []*unstructured.Unstructured
	for _, group := range m {
		for _, obj := range group {
			list = append(list, obj)
		}
	}
	return list
}

// MakeChildMap builds the map of children resources that is suitable for use
// in the `children` field of a CompositeController SyncRequest or
// `attachments` field of  the  DecoratorControllers SyncRequest.
//
// This function returns a ChildMap which is a map of maps. The outer most map
// is keyed  using the child's type and the inner map is keyed using the
// child's name. If the parent resource is clustered and the child resource
// is namespaced the inner map's keys are prefixed by the namespace of the
// child resource.
//
// This function requires parent resources has the meta.Namespace accurately
// set. If the namespace of the pareent is empty it's considered a clustered
// resource.
//
// If a user returns a namespaced as a child of a clustered resources without
// the namespace set this is considered a user error but it's not handled here
// since the api errorstrying to create the child is clear.
func MakeChildMap(parent metav1.Object, list []*unstructured.Unstructured) ChildMap {
	children := make(ChildMap)

	for _, child := range list {
		children.Insert(parent, child)
	}
	return children
}

func childMapKey(apiVersion, kind string) string {
	return fmt.Sprintf("%s.%s", kind, apiVersion)
}

func ParseChildMapKey(key string) (apiVersion, kind string) {
	parts := strings.SplitN(key, ".", 2)
	return parts[1], parts[0]
}

func ParseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		// It's a core version.
		return "", parts[0]
	}
	return parts[0], parts[1]
}

type GroupKindMap map[string]*dynamicdiscovery.APIResource

func (m GroupKindMap) Set(apiGroup, kind string, resource *dynamicdiscovery.APIResource) {
	m[groupKindKey(apiGroup, kind)] = resource
}

func (m GroupKindMap) Get(apiGroup, kind string) *dynamicdiscovery.APIResource {
	return m[groupKindKey(apiGroup, kind)]
}

func groupKindKey(apiGroup, kind string) string {
	return fmt.Sprintf("%s.%s", kind, apiGroup)
}

type InformerMap map[string]*dynamicinformer.ResourceInformer

func (m InformerMap) Set(apiVersion, resource string, informer *dynamicinformer.ResourceInformer) {
	m[informerMapKey(apiVersion, resource)] = informer
}

func (m InformerMap) Get(apiVersion, resource string) *dynamicinformer.ResourceInformer {
	return m[informerMapKey(apiVersion, resource)]
}

func informerMapKey(apiVersion, resource string) string {
	return fmt.Sprintf("%s.%s", resource, apiVersion)
}
