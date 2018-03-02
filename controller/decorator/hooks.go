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

package decorator

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	"k8s.io/metacontroller/controller/common"
	"k8s.io/metacontroller/webhook"
)

type syncHookRequest struct {
	Controller  runtime.Object             `json:"controller"`
	Object      *unstructured.Unstructured `json:"object"`
	Attachments common.ChildMap            `json:"attachments"`
}

type syncHookResponse struct {
	Labels      map[string]*string           `json:"labels"`
	Annotations map[string]*string           `json:"annotations"`
	Attachments []*unstructured.Unstructured `json:"attachments"`
}

func callSyncHook(dc *v1alpha1.DecoratorController, request *syncHookRequest) (*syncHookResponse, error) {
	url := fmt.Sprintf("http://%s.%s%s", dc.Spec.ClientConfig.Service.Name, dc.Spec.ClientConfig.Service.Namespace, dc.Spec.Hooks.Sync.Path)
	var response syncHookResponse
	if err := webhook.Call(url, request, &response); err != nil {
		return nil, fmt.Errorf("sync hook failed: %v", err)
	}
	return &response, nil
}
