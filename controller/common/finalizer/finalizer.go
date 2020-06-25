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

package finalizer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dynamicclientset "metacontroller.io/dynamic/clientset"
	dynamicobject "metacontroller.io/dynamic/object"
)

// Manager encapsulates controller logic for dealing with finalizers.
type Manager struct {
	Name    string
	Enabled bool
}

// SyncObject adds or removes the finalizer on the given object as necessary.
func (m *Manager) SyncObject(client *dynamicclientset.ResourceClient, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// If the cached object passed in is already in the right state,
	// we'll assume we don't need to check the live object.
	if dynamicobject.HasFinalizer(obj, m.Name) == m.Enabled {
		return obj, nil
	}
	// Otherwise, we may need to update the object.
	if m.Enabled {
		// If the object is already pending deletion, we don't add the finalizer.
		// We might have already removed it.
		if obj.GetDeletionTimestamp() != nil {
			return obj, nil
		}
		return client.Namespace(obj.GetNamespace()).AddFinalizer(obj, m.Name)
	} else {
		return client.Namespace(obj.GetNamespace()).RemoveFinalizer(obj, m.Name)
	}
}

// ShouldFinalize returns true if the controller should take action to manage
// children even though the parent is pending deletion (i.e. finalize).
func (m *Manager) ShouldFinalize(parent metav1.Object) bool {
	// There's no point managing children if the parent has a GC finalizer,
	// because we'd be fighting the GC.
	if hasGCFinalizer(parent) {
		return false
	}
	// If we already removed the finalizer, don't try to manage children anymore.
	if !dynamicobject.HasFinalizer(parent, m.Name) {
		return false
	}
	return m.Enabled
}

// hasGCFinalizer returns true if obj has any GC finalizer.
// In other words, true means the GC will start messing with its children,
// either deleting or orphaning them.
func hasGCFinalizer(obj metav1.Object) bool {
	for _, item := range obj.GetFinalizers() {
		switch item {
		case metav1.FinalizerDeleteDependents, metav1.FinalizerOrphanDependents:
			return true
		}
	}
	return false
}
