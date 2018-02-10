/*
Copyright 2017 Google Inc.

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

package initializer

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	"k8s.io/metacontroller/webhook"
)

type initHookRequest struct {
	Object *unstructured.Unstructured `json:"object"`
}

type initHookResponse struct {
	Object *unstructured.Unstructured `json:"object"`
	Result *metav1.Status             `json:"result,omitempty"`
}

func callInitHook(ic *v1alpha1.InitializerController, request *initHookRequest) (*initHookResponse, error) {
	url := fmt.Sprintf("http://%s.%s%s", ic.Spec.ClientConfig.Service.Name, ic.Spec.ClientConfig.Service.Namespace, ic.Spec.Hooks.Init.Path)
	var response initHookResponse
	if err := webhook.Call(url, request, &response); err != nil {
		return nil, fmt.Errorf("init hook failed: %v", err)
	}
	return &response, nil
}
