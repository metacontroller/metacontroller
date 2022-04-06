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

package hooks

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
)

type Hooks interface {
}

// Hook an execute Hook requests
type Hook interface {
	IsEnabled() bool
	Call(request interface{}, response interface{}) error
}

// NewHook return new Hook which implements given v1alpha1.Hook
func NewHook(
	hook *v1alpha1.Hook,
	controllerName string,
	controllerType common.ControllerType,
	hookType common.HookType) (Hook, error) {
	if hook != nil {
		executor, err := NewWebhookExecutor(hook.Webhook, controllerName, controllerType, hookType)
		if err != nil {
			return nil, err
		}
		return &hookExecutorImpl{
			webhookExecutor: executor,
		}, nil
	}
	return &hookExecutorImpl{
		webhookExecutor: nil,
	}, nil
}

// hookExecutorImpl is default implementation of Hook
type hookExecutorImpl struct {
	webhookExecutor *WebhookExecutor
}

func (h *hookExecutorImpl) IsEnabled() bool {
	return h.webhookExecutor != nil
}

func (h *hookExecutorImpl) Call(request interface{}, response interface{}) error {
	return h.webhookExecutor.Call(request, response)
}
