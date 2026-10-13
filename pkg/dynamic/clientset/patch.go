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
	"reflect"
)

// MergePatchValue returns the RFC 7386 JSON merge patch fragment that
// transforms old into new. Mappings (map[string]interface{}) are diffed
// recursively, and keys present in old but missing from new are represented
// with an explicit null entry so that applying the patch removes them; all
// other values (scalars, arrays, type changes) are replaced wholesale, which
// is how a JSON merge patch treats them anyway.
// It returns nil when old and new are equal, which callers use to omit the
// field from the patch.
func MergePatchValue(old, new interface{}) interface{} {
	om, okOld := old.(map[string]interface{})
	nm, okNew := new.(map[string]interface{})
	if !okOld || !okNew {
		if reflect.DeepEqual(old, new) {
			return nil
		}
		return new
	}
	patch := map[string]interface{}{}
	for k, nv := range nm {
		ov, exists := om[k]
		if !exists {
			patch[k] = nv
			continue
		}
		if sub := MergePatchValue(ov, nv); sub != nil {
			patch[k] = sub
		}
	}
	for k := range om {
		if _, exists := nm[k]; !exists {
			patch[k] = nil
		}
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}

// StringMapMergePatchValue returns the merge patch fragment that transforms
// the old string map (e.g. labels or annotations) into the new one, with keys
// removed by the caller represented as explicit null entries so that applying
// the patch deletes them.
// It returns nil when the maps are equal.
func StringMapMergePatchValue(old, new map[string]string) map[string]interface{} {
	patch := map[string]interface{}{}
	for k, v := range new {
		if ov, exists := old[k]; !exists || ov != v {
			patch[k] = v
		}
	}
	for k := range old {
		if _, exists := new[k]; !exists {
			patch[k] = nil
		}
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}
