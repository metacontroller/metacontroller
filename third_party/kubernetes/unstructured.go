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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/util/json"
)

// These are based on functions from k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.
// They are copied here to make them exported.

func GetNestedFieldInto(out interface{}, obj map[string]interface{}, fields ...string) error {
	objMap, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil {
		return fmt.Errorf("can't get nested field %v: %v", strings.Join(fields, "."), err)
	}
	if !found {
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
