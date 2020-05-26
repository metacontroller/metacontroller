/*
Copyright 2015 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/json"
)

// These are based on functions from k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.
// They are copied here to make them exported.

func GetNestedField(obj map[string]interface{}, fields ...string) interface{} {
	var val interface{} = obj
	for _, field := range fields {
		if _, ok := val.(map[string]interface{}); !ok {
			return nil
		}
		val = val.(map[string]interface{})[field]
	}
	return val
}

func GetNestedFieldInto(out interface{}, obj map[string]interface{}, fields ...string) error {
	objMap := GetNestedField(obj, fields...)
	if objMap == nil {
		// If field has no value, leave `out` as is.
		return nil
	}
	// Decode into the requested output type.
	data, err := json.Marshal(objMap)
	if err != nil {
		return fmt.Errorf("can't encode nested field %v: %v", strings.Join(fields, "."), err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("can't decode nested field %v into type %T: %v", strings.Join(fields, "."), out, err)
	}
	return nil
}

func GetNestedString(obj map[string]interface{}, fields ...string) string {
	if str, ok := GetNestedField(obj, fields...).(string); ok {
		return str
	}
	return ""
}

func GetNestedArray(obj map[string]interface{}, fields ...string) []interface{} {
	if arr, ok := GetNestedField(obj, fields...).([]interface{}); ok {
		return arr
	}
	return nil
}

func GetNestedObject(obj map[string]interface{}, fields ...string) map[string]interface{} {
	if obj, ok := GetNestedField(obj, fields...).(map[string]interface{}); ok {
		return obj
	}
	return nil
}

func GetNestedInt64(obj map[string]interface{}, fields ...string) int64 {
	if str, ok := GetNestedField(obj, fields...).(int64); ok {
		return str
	}
	return 0
}

func GetNestedInt64Pointer(obj map[string]interface{}, fields ...string) *int64 {
	nested := GetNestedField(obj, fields...)
	switch n := nested.(type) {
	case int64:
		return &n
	case *int64:
		return n
	default:
		return nil
	}
}

func GetNestedSlice(obj map[string]interface{}, fields ...string) []string {
	if m, ok := GetNestedField(obj, fields...).([]interface{}); ok {
		strSlice := make([]string, 0, len(m))
		for _, v := range m {
			if str, ok := v.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		return strSlice
	}
	return nil
}

func GetNestedMap(obj map[string]interface{}, fields ...string) map[string]string {
	if m, ok := GetNestedField(obj, fields...).(map[string]interface{}); ok {
		strMap := make(map[string]string, len(m))
		for k, v := range m {
			if str, ok := v.(string); ok {
				strMap[k] = str
			}
		}
		return strMap
	}
	return nil
}

func SetNestedField(obj map[string]interface{}, value interface{}, fields ...string) {
	m := obj
	if len(fields) > 1 {
		for _, field := range fields[0 : len(fields)-1] {
			if _, ok := m[field].(map[string]interface{}); !ok {
				m[field] = make(map[string]interface{})
			}
			m = m[field].(map[string]interface{})
		}
	}
	m[fields[len(fields)-1]] = value
}

func DeleteNestedField(obj map[string]interface{}, fields ...string) {
	m := obj
	if len(fields) > 1 {
		for _, field := range fields[0 : len(fields)-1] {
			if _, ok := m[field].(map[string]interface{}); !ok {
				m[field] = make(map[string]interface{})
			}
			m = m[field].(map[string]interface{})
		}
	}
	delete(m, fields[len(fields)-1])
}

func SetNestedSlice(obj map[string]interface{}, value []string, fields ...string) {
	m := make([]interface{}, 0, len(value))
	for _, v := range value {
		m = append(m, v)
	}
	SetNestedField(obj, m, fields...)
}

func SetNestedMap(obj map[string]interface{}, value map[string]string, fields ...string) {
	m := make(map[string]interface{}, len(value))
	for k, v := range value {
		m[k] = v
	}
	SetNestedField(obj, m, fields...)
}
