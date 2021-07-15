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
)

const (
	FinalizeHook  string = "finalize"
	CustomizeHook string = "customize"
	SyncHook      string = "sync"
)

// HookExecutor an execute Hook requests
type HookExecutor interface {
	IsEnabled() bool
	Execute(request interface{}, response interface{}) error
}

// NewHookExecutor return new HookExecutor which implements given v1alpha1.Hook
func NewHookExecutor(hook *v1alpha1.Hook, hookType string) (HookExecutor, error) {
	if hook != nil {
		executor, err := NewWebhookExecutor(hook.Webhook, hookType)
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

// hookExecutorImpl is default implementation of HookExecutor
type hookExecutorImpl struct {
	webhookExecutor *WebhookExecutor
}

func (h *hookExecutorImpl) IsEnabled() bool {
	return h.webhookExecutor != nil
}

func (h *hookExecutorImpl) Execute(request interface{}, response interface{}) error {
	return h.webhookExecutor.Execute(request, response)
}
