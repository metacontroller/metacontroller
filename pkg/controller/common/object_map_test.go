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

package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"metacontroller/pkg/controller/common/api"
	v1 "metacontroller/pkg/controller/common/api/v1"
	v2 "metacontroller/pkg/controller/common/api/v2"
)

// TestObjectMapInterface verifies that both RelativeObjectMap and UniformObjectMap
// implement the ObjectMap interface correctly.
func TestObjectMapInterface(t *testing.T) {
	// Verify RelativeObjectMap implements ObjectMap
	var _ api.ObjectMap = make(v1.RelativeObjectMap)

	// Verify UniformObjectMap implements ObjectMap
	var _ api.ObjectMap = make(v2.UniformObjectMap)

	t.Log("Both RelativeObjectMap and UniformObjectMap implement ObjectMap interface")
}

// TestObjectMapInterfaceUsage tests that we can use both types through the interface
func TestObjectMapInterfaceUsage(t *testing.T) {
	testCases := []struct {
		name      string
		objectMap api.ObjectMap
	}{
		{
			name:      "RelativeObjectMap",
			objectMap: make(v1.RelativeObjectMap),
		},
		{
			name:      "UniformObjectMap",
			objectMap: make(v2.UniformObjectMap),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that we can call interface methods
			objects := tc.objectMap.List()
			// List() returns nil for empty maps, which is expected behavior
			if len(objects) != 0 {
				t.Errorf("List() should return nil or empty slice for empty map, got %d items", len(objects))
			}

			// Test that we can call other interface methods without panic
			gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestKind"}
			tc.objectMap.InitGroup(gvk)
		})
	}
}
