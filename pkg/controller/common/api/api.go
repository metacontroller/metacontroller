/*
 *
 * Copyright 2022. Metacontroller authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package api

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type WebhookRequest interface {
	GetRootObject() *unstructured.Unstructured
}

// GroupVersionKind is metacontroller wrapper around schema.GroupVersionKind
// implementing encoding.TextMarshaler and encoding.TextUnmarshaler
type GroupVersionKind struct {
	schema.GroupVersionKind
}

// MarshalText is implementation of  encoding.TextMarshaler
func (gvk GroupVersionKind) MarshalText() ([]byte, error) {
	var marshalledText string
	if gvk.Group == "" {
		marshalledText = fmt.Sprintf("%s.%s", gvk.Kind, gvk.Version)
	} else {
		marshalledText = fmt.Sprintf("%s.%s/%s", gvk.Kind, gvk.Group, gvk.Version)
	}
	return []byte(marshalledText), nil
}

// UnmarshalText is implementation of encoding.TextUnmarshaler
func (gvk *GroupVersionKind) UnmarshalText(text []byte) error {
	kindGroupVersionString := string(text)
	parts := strings.SplitN(kindGroupVersionString, ".", 2)
	if len(parts) < 2 {
		return fmt.Errorf("could not unmarshall [%s], expected string in 'kind.group/version' format", string(text))
	}
	groupVersion, err := schema.ParseGroupVersion(parts[1])
	if err != nil {
		return err
	}
	*gvk = GroupVersionKind{
		groupVersion.WithKind(parts[0]),
	}
	return nil
}
