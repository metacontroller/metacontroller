/*
 *
 * Copyright 2026. Metacontroller authors.
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
	"metacontroller/pkg/controller/common/customize/api/common"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CustomizeHookRequest is a request send to customize hook
type CustomizeHookRequest struct {
	Controller v1alpha1.CustomizableController `json:"controller"`
	Parent     *unstructured.Unstructured      `json:"parent"`
}

func (r *CustomizeHookRequest) GetRootObject() *unstructured.Unstructured {
	return r.Parent
}

// CustomizeHookResponse is a response from customize hook
type CustomizeHookResponse struct {
	Version              v1alpha1.HookVersion            `json:"-"`
	RelatedResourceRules []*v1alpha1.RelatedResourceRule `json:"relatedResources,omitempty"`
}

type requestBuilder struct {
	controller v1alpha1.CustomizableController
	parent     *unstructured.Unstructured
}

func NewRequestBuilder() common.WebhookRequestBuilder {
	return &requestBuilder{}
}

func (r *requestBuilder) WithController(controller v1alpha1.CustomizableController) common.WebhookRequestBuilder {
	r.controller = controller
	return r
}

func (r *requestBuilder) WithParent(parent *unstructured.Unstructured) common.WebhookRequestBuilder {
	r.parent = parent
	return r
}

func (r *requestBuilder) Build() api.WebhookRequest {
	return &CustomizeHookRequest{
		Controller: r.controller,
		Parent:     r.parent,
	}
}
