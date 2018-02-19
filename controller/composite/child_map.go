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

package composite

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type childMap map[string]map[string]*unstructured.Unstructured

func (m childMap) findGroupKindName(apiGroup, kind, name string) *unstructured.Unstructured {
	// The map is keyed by group-version-kind, but we don't know the version.
	// So, check inside any GVK that matches the group and kind, ignoring version.
	for key, children := range m {
		if gv, k := parseChildMapKey(key); k == kind {
			if g, _ := parseAPIVersion(gv); g == apiGroup {
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

func (m childMap) replaceChild(child *unstructured.Unstructured) {
	key := childMapKey(child.GetAPIVersion(), child.GetKind())
	children := m[key]
	if children == nil {
		// We only want to replace if it already exists, so do nothing.
		return
	}
	name := child.GetName()
	if _, found := children[name]; found {
		children[name] = child
	}
}

func makeChildMap(list []*unstructured.Unstructured) childMap {
	children := make(childMap)
	for _, child := range list {
		key := childMapKey(child.GetAPIVersion(), child.GetKind())

		if children[key] == nil {
			children[key] = make(map[string]*unstructured.Unstructured)
		}
		children[key][child.GetName()] = child
	}
	return children
}

func childMapKey(apiVersion, kind string) string {
	return fmt.Sprintf("%s.%s", kind, apiVersion)
}

func parseChildMapKey(key string) (apiVersion, kind string) {
	parts := strings.SplitN(key, ".", 2)
	return parts[1], parts[0]
}

func parseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		// It's a core version.
		return "", parts[0]
	}
	return parts[0], parts[1]
}

type childClaimMap map[string]map[string]*parentRevision

func claimMapKey(apiGroup, kind string) string {
	return fmt.Sprintf("%s.%s", kind, apiGroup)
}
