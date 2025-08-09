/*
 *
 * Copyright 2023. Metacontroller authors.
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

package v1

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/api"
	v1 "metacontroller/pkg/controller/common/api/v1"
	v2 "metacontroller/pkg/controller/common/api/v2"
	"metacontroller/pkg/controller/decorator/api/common"
	"metacontroller/pkg/logging"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DecoratorHookRequest is the parent sent as JSON to the sync hook.
type DecoratorHookRequest struct {
	Controller  *v1alpha1.DecoratorController `json:"controller"`
	Object      *unstructured.Unstructured    `json:"object"`
	Attachments v1.RelativeObjectMap          `json:"attachments"`
	Related     v1.RelativeObjectMap          `json:"related"`
	Finalizing  bool                          `json:"finalizing"`
}

func (r *DecoratorHookRequest) GetRootObject() *unstructured.Unstructured {
	return r.Object
}

// DecoratorHookResponse is the expected format of the JSON response from the sync hook.
type DecoratorHookResponse struct {
	Labels      map[string]*string           `json:"labels"`
	Annotations map[string]*string           `json:"annotations"`
	Status      map[string]interface{}       `json:"status"`
	Attachments []*unstructured.Unstructured `json:"attachments"`

	ResyncAfterSeconds float64 `json:"resyncAfterSeconds"`

	// Finalized is only used by the finalize hook.
	Finalized bool `json:"finalized"`
}

type requestBuilder struct {
	controller *v1alpha1.DecoratorController
	parent     *unstructured.Unstructured
	children   api.ObjectMap
	related    api.ObjectMap
	finalizing bool
}

func NewRequestBuilder() common.WebhookRequestBuilder {
	return &requestBuilder{}
}

func (r *requestBuilder) WithController(controller *v1alpha1.DecoratorController) common.WebhookRequestBuilder {
	r.controller = controller
	return r
}

func (r *requestBuilder) WithParent(parent *unstructured.Unstructured) common.WebhookRequestBuilder {
	r.parent = parent
	return r
}

func (r *requestBuilder) WithChildren(children api.ObjectMap) common.WebhookRequestBuilder {
	r.children = children
	return r
}

func (r *requestBuilder) WithRelatedObjects(related api.ObjectMap) common.WebhookRequestBuilder {
	r.related = related
	return r
}

func (r *requestBuilder) IsFinalizing() common.WebhookRequestBuilder {
	r.finalizing = true
	return r
}

func (r *requestBuilder) Build() api.WebhookRequest {
	// Convert to RelativeObjectMap for v1 API compatibility
	childrenRelative := r.toRelativeObjectMap(r.children, "attachments")
	relatedRelative := r.toRelativeObjectMap(r.related, "related")

	return &DecoratorHookRequest{
		Controller:  r.controller,
		Object:      r.parent,
		Attachments: childrenRelative,
		Related:     relatedRelative,
		Finalizing:  r.finalizing,
	}
}

// toRelativeObjectMap safely converts an ObjectMap to RelativeObjectMap with validation
func (r *requestBuilder) toRelativeObjectMap(objMap api.ObjectMap, fieldName string) v1.RelativeObjectMap {
	if objMap == nil {
		return make(v1.RelativeObjectMap)
	}

	// Validate parent context exists for relative naming
	if r.parent == nil {
		// Log warning but continue with empty map rather than panicking
		logging.Logger.V(1).Info("Decorator v1 requestBuilder: parent context is nil when converting field to RelativeObjectMap; returning empty map", "field", fieldName)
		// This maintains backward compatibility while highlighting the issue
		return make(v1.RelativeObjectMap)
	}

	// Handle conversion from different ObjectMap types
	switch typed := objMap.(type) {
	case v1.RelativeObjectMap:
		// Already in correct format
		return typed
	case v2.UniformObjectMap:
		// Use efficient direct conversion
		return typed.Convert(r.parent)
	default:
		// Fallback for any other ObjectMap implementation
		// This preserves extensibility while providing safe conversion
		return v1.MakeRelativeObjectMap(r.parent, objMap.List())
	}
}
