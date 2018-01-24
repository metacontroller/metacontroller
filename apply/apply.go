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

// Package apply is a dynamic, client-side substitute for `kubectl apply` that
// tries to guess the right thing to do without any type-specific knowledge.
// Instead of generating a PATCH request, it does the patching locally and
// returns a full object with the ResourceVersion intact.
//
// We can't use actual `kubectl apply` yet because it doesn't support strategic
// merge for CRDs, which would make it infeasible to include a PodTemplateSpec
// in a CRD (e.g. containers and volumes will merge incorrectly).
package apply

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	lastAppliedAnnotation = "metacontroller.k8s.io/last-applied-configuration"
)

func SetLastApplied(obj *unstructured.Unstructured, lastApplied map[string]interface{}) error {
	lastAppliedJSON, err := json.Marshal(lastApplied)
	if err != nil {
		return fmt.Errorf("can't marshal last applied config: %v", err)
	}

	ann := obj.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[lastAppliedAnnotation] = string(lastAppliedJSON)
	obj.SetAnnotations(ann)
	return nil
}

func GetLastApplied(obj *unstructured.Unstructured) (map[string]interface{}, error) {
	lastAppliedJSON := obj.GetAnnotations()[lastAppliedAnnotation]
	if lastAppliedJSON == "" {
		return nil, nil
	}
	lastApplied := make(map[string]interface{})
	err := json.Unmarshal([]byte(lastAppliedJSON), &lastApplied)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal %q annotation: %v", lastAppliedAnnotation, err)
	}
	return lastApplied, nil
}

// Merge updates the given observed object to apply the desired changes.
// It returns an updated copy of the observed object if no error occurs.
func Merge(observed, lastApplied, desired map[string]interface{}) (map[string]interface{}, error) {
	// Make a copy of observed since merge() mutates the destination.
	destination := runtime.DeepCopyJSON(observed)

	if _, err := merge("", destination, lastApplied, desired); err != nil {
		return nil, fmt.Errorf("can't merge desired changes: %v", err)
	}
	return destination, nil
}

// merge finds the diff from lastApplied to desired,
// and applies it to destination, returning the replacement destination value.
func merge(fieldPath string, destination, lastApplied, desired interface{}) (interface{}, error) {
	switch destVal := destination.(type) {
	case map[string]interface{}:
		// destination is an object.
		// Make sure the others are objects too (or null).
		lastVal, ok := lastApplied.(map[string]interface{})
		if !ok && lastVal != nil {
			return nil, fmt.Errorf("lastApplied%s: expecting map[string]interface, got %T", fieldPath, lastApplied)
		}
		desVal, ok := desired.(map[string]interface{})
		if !ok && desVal != nil {
			return nil, fmt.Errorf("desired%s: expecting map[string]interface, got %T", fieldPath, desired)
		}
		return mergeObject(fieldPath, destVal, lastVal, desVal)
	case []interface{}:
		// destination is an array.
		// Make sure the others are arrays too (or null).
		lastVal, ok := lastApplied.([]interface{})
		if !ok && lastVal != nil {
			return nil, fmt.Errorf("lastApplied%s: expecting []interface, got %T", fieldPath, lastApplied)
		}
		desVal, ok := desired.([]interface{})
		if !ok && desVal != nil {
			return nil, fmt.Errorf("desired%s: expecting []interface, got %T", fieldPath, desired)
		}
		return mergeArray(fieldPath, destVal, lastVal, desVal)
	default:
		// destination is a scalar or null.
		// Just take the desired value. We won't be called if there's none.
		return desired, nil
	}
}

func mergeObject(fieldPath string, destination, lastApplied, desired map[string]interface{}) (interface{}, error) {
	// Remove fields that were present in lastApplied, but no longer in desired.
	for key := range lastApplied {
		if _, present := desired[key]; !present {
			delete(destination, key)
		}
	}

	// Add/Update all fields present in desired.
	var err error
	for key, desVal := range desired {
		destination[key], err = merge(fmt.Sprintf("%s[%s]", fieldPath, key), destination[key], lastApplied[key], desVal)
		if err != nil {
			return nil, err
		}
	}

	return destination, nil
}

func mergeArray(fieldPath string, destination, lastApplied, desired []interface{}) (interface{}, error) {
	// If all the lists *could be* "list maps", use the special merge.
	if detectListMap(destination) && detectListMap(lastApplied) && detectListMap(desired) {
		return mergeListMap(fieldPath, destination, lastApplied, desired)
	}

	// It's a normal array. Just replace for now.
	// TODO(enisoc): Check if there are any common cases where we want to merge.
	return desired, nil
}

func mergeListMap(fieldPath string, destination, lastApplied, desired []interface{}) (interface{}, error) {
	// Treat each list of objects as if it were a map, keyed by the "name" field.
	destMap := makeListMap(destination)
	lastMap := makeListMap(lastApplied)
	desMap := makeListMap(desired)

	_, err := mergeObject(fieldPath, destMap, lastMap, desMap)
	if err != nil {
		return nil, err
	}

	// Turn destMap back into a list, trying to preserve partial order.
	destList := make([]interface{}, 0, len(destMap))
	added := make(map[string]bool, len(destMap))
	// First take items that were already in destination.
	for _, item := range destination {
		key := item.(map[string]interface{})["name"].(string)
		if newItem, ok := destMap[key]; ok {
			destList = append(destList, newItem)
			// Remember which items we've already added to the final list.
			added[key] = true
		}
	}
	// Then take items in desired that haven't been added yet.
	for _, item := range desired {
		key := item.(map[string]interface{})["name"].(string)
		if !added[key] {
			destList = append(destList, destMap[key])
			added[key] = true
		}
	}

	return destList, nil
}

func makeListMap(list []interface{}) map[string]interface{} {
	res := make(map[string]interface{}, len(list))
	for _, item := range list {
		// We only call this if detectListMap() returned true.
		itemMap := item.(map[string]interface{})
		res[itemMap["name"].(string)] = item
	}
	return res
}

// detectListMap tries to guess whether a list is a Kubernetes-style "list map".
// For now, we only support "name" as the merge key.
// TODO(enisoc): Support other conventional merge keys.
func detectListMap(list []interface{}) bool {
	// If the list is empty, we default to saying yes.
	// In other words, we're just answering, "Could this be a list map?"
	if len(list) == 0 {
		return false
	}

	for _, item := range list {
		// All the items must be objects.
		obj, ok := item.(map[string]interface{})
		if !ok {
			return false
		}
		// For now, all the items must have a string-type "name" field.
		if _, ok := obj["name"].(string); !ok {
			return false
		}
	}
	return true
}
