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
	"metacontroller/pkg/dynamic/apply"

	jp "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sjson "k8s.io/apimachinery/pkg/util/json"
)

// JsonMergePatch returns merge patch which describes changes needed to apply
// to original, to get desired in result.
func JsonMergePatch(original, desired *unstructured.Unstructured) ([]byte, error) {
	originalCopy := original.DeepCopy()
	desiredCopy := desired.DeepCopy()
	nullifyLastAppliedAnnotation(originalCopy)
	nullifyLastAppliedAnnotation(desiredCopy)

	originalJson, err := k8sjson.Marshal(originalCopy)
	if err != nil {
		return nil, err
	}
	desiredJson, err := k8sjson.Marshal(desiredCopy)
	if err != nil {
		return nil, err
	}
	return jp.CreateMergePatch(originalJson, desiredJson)
}

// nullifyLastAppliedAnnotation removes the metadata.annotations apply.LastAppliedAnnotation
// value if present
func nullifyLastAppliedAnnotation(object *unstructured.Unstructured) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		return
	}
	_, exists := annotations[apply.LastAppliedAnnotation]
	if !exists {
		return
	}
	annotations[apply.LastAppliedAnnotation] = ""
	object.SetAnnotations(annotations)
}
