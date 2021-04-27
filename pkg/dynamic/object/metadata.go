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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasFinalizer returns true if obj has the named finalizer.
func HasFinalizer(obj metav1.Object, name string) bool {
	for _, item := range obj.GetFinalizers() {
		if item == name {
			return true
		}
	}
	return false
}

// AddFinalizer adds the named finalizer to obj, if it isn't already present.
func AddFinalizer(obj metav1.Object, name string) {
	if HasFinalizer(obj, name) {
		// It's already present, so there's nothing to do.
		return
	}
	obj.SetFinalizers(append(obj.GetFinalizers(), name))
}

// RemoveFinalizer removes the named finalizer from obj, if it's present.
func RemoveFinalizer(obj metav1.Object, name string) {
	finalizers := obj.GetFinalizers()
	for i, item := range finalizers {
		if item == name {
			obj.SetFinalizers(append(finalizers[:i], finalizers[i+1:]...))
			return
		}
	}
	// We never found it, so it's already gone and there's nothing to do.
}
