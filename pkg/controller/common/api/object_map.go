/*
Copyright 2023 Metacontroller authors.

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

package api

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ObjectMap provides a unified interface for managing Kubernetes objects grouped by
// GroupVersionKind. This interface allows internal controller code to work with either
// RelativeObjectMap (v1 API) or UniformObjectMap (v2 API) while maintaining backward
// compatibility for webhook APIs.
//
// Thread Safety:
//   - Implementations are safe for concurrent read operations (List, GetObjectsByGVK, etc.)
//   - Write operations (Insert, InsertAll, ReplaceObjectIfExists) must be synchronized by the caller
//   - InitGroup should be called before any operations on a new GroupVersionKind
//
// Usage Patterns:
//   - Use Insert/InsertAll to add objects with proper parent-relative naming semantics
//   - Use GetObjectsByGVK for efficient type-specific lookups when processing by resource type
//   - Use List() when you need all objects regardless of type (e.g., for conversions)
//   - Use GetAllGVKs() to iterate over all resource types present in the map
//
// Implementation Notes:
//   - RelativeObjectMap: Keys objects relative to parent (namespace/name or just name)
//   - UniformObjectMap: Keys objects uniformly (always namespace/name for namespaced resources)
//   - Parent parameter is required for operations that affect object naming/grouping
type ObjectMap interface {
	// Insert adds an object to the map with appropriate parent-relative naming.
	// The parent parameter is used to determine the correct key format for the object.
	// For RelativeObjectMap: uses relative naming (name only if same namespace as parent)
	// For UniformObjectMap: uses uniform naming (always namespace/name for namespaced objects)
	Insert(parent v1.Object, obj *unstructured.Unstructured)

	// InsertAll adds multiple objects to the map, equivalent to calling Insert for each object.
	// This is more efficient than individual Insert calls for bulk operations.
	InsertAll(parent v1.Object, objects []*unstructured.Unstructured)

	// FindGroupKindName searches for an object by name across all versions of a GroupKind.
	// This is useful when you know the object name but not the exact version.
	// Returns nil if no object is found with the given name in any version of the GroupKind.
	FindGroupKindName(gk schema.GroupKind, name string) *unstructured.Unstructured

	// ReplaceObjectIfExists replaces an existing object with the same name and namespace.
	// If no matching object exists, no action is taken (no error is returned).
	// The parent parameter is used for consistent naming semantics.
	ReplaceObjectIfExists(parent v1.Object, obj *unstructured.Unstructured)

	// List returns all objects in the map as a flat slice, in no particular order.
	// This creates a new slice on each call, so the returned slice is safe to modify.
	// Returns nil for empty maps (consistent with existing behavior).
	List() []*unstructured.Unstructured

	// InitGroup initializes storage for a specific GroupVersionKind if not already present.
	// This should be called before adding objects of a new type to ensure proper initialization.
	// Safe to call multiple times for the same GVK.
	InitGroup(gvk schema.GroupVersionKind)

	// GetObjectsByGVK returns all objects for a specific GroupVersionKind.
	// Returns nil if no objects exist for the given GVK.
	// The returned map should be treated as read-only to prevent unintended mutations.
	GetObjectsByGVK(gvk schema.GroupVersionKind) map[string]*unstructured.Unstructured

	// GetAllGVKs returns all GroupVersionKinds that have objects in the map.
	// Returns a new slice on each call, safe to modify.
	// Returns nil for empty maps to avoid unnecessary allocations.
	GetAllGVKs() []schema.GroupVersionKind
}
