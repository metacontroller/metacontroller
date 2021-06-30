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

package customize

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/hooks"
)

var callCustomizeHook = hooks.Call

type CustomizableController interface {
	GetCustomizeHook() *v1alpha1.Hook
}

type CustomizeHookRequest struct {
	Controller CustomizableController     `json:"controller"`
	Parent     *unstructured.Unstructured `json:"parent"`
}

type CustomizeHookResponse struct {
	RelatedResourceRules []*v1alpha1.RelatedResourceRule `json:"relatedResources,omitempty"`
}

func CallCustomizeHook(cc CustomizableController, request *CustomizeHookRequest) (*CustomizeHookResponse, error) {
	var response CustomizeHookResponse

	hook := cc.GetCustomizeHook()
	// As the related hook is optional, return nothing
	if hook == nil {
		return &response, nil
	}

	if err := callCustomizeHook(hook, hooks.CustomizeHook, request, &response); err != nil {
		return nil, fmt.Errorf("related hook failed: %w", err)
	}

	return &response, nil
}
