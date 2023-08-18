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
	"metacontroller/pkg/controller/composite/api/common"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CompositeHookRequest is the object sent as JSON to the sync and finalize hooks.
type CompositeHookRequest struct {
	Controller *v1alpha1.CompositeController `json:"controller"`
	Parent     *unstructured.Unstructured    `json:"parent"`
	Children   v1.RelativeObjectMap          `json:"children"`
	Related    v1.RelativeObjectMap          `json:"related"`
	Finalizing bool                          `json:"finalizing"`
}

// CompositeHookResponse is the expected format of the JSON response from the sync and finalize hooks.
type CompositeHookResponse struct {
	Status   map[string]interface{}       `json:"status"`
	Children []*unstructured.Unstructured `json:"children"`

	ResyncAfterSeconds float64 `json:"resyncAfterSeconds"`

	// Finalized is only used by the finalize hook.
	Finalized bool `json:"finalized"`
}

type requestBuilder struct {
	controller *v1alpha1.CompositeController
	parent     *unstructured.Unstructured
	children   v2.UniformObjectMap
	related    v2.UniformObjectMap
	finalizing bool
}

func NewRequestBuilder() common.WebhookRequestBuilder {
	return &requestBuilder{}
}

func (r *requestBuilder) WithController(controller *v1alpha1.CompositeController) common.WebhookRequestBuilder {
	r.controller = controller
	return r
}

func (r *requestBuilder) WithParent(parent *unstructured.Unstructured) common.WebhookRequestBuilder {
	r.parent = parent
	return r
}

func (r *requestBuilder) WithChildren(children v2.UniformObjectMap) common.WebhookRequestBuilder {
	r.children = children
	return r
}

func (r *requestBuilder) WithRelatedObjects(related v2.UniformObjectMap) common.WebhookRequestBuilder {
	r.related = related
	return r
}

func (r *requestBuilder) IsFinalizing() common.WebhookRequestBuilder {
	r.finalizing = true
	return r
}

func (r *requestBuilder) Build() api.WebhookRequest {
	return &CompositeHookRequest{
		Controller: r.controller,
		Parent:     r.parent,
		Children:   r.children.Convert(r.parent, false),
		Related:    r.related.Convert(r.parent, true),
		Finalizing: r.finalizing,
	}
}

func (r *CompositeHookRequest) GetRootObject() *unstructured.Unstructured {
	return r.Parent
}
