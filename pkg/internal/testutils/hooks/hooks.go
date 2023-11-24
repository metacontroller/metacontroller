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

package hooks

import (
	"fmt"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/api"
	"metacontroller/pkg/hooks"
	"reflect"

	k8sjson "k8s.io/apimachinery/pkg/util/json"
)

// NewHookExecutorStub creates new HookExecutorStub which returns given response
func NewHookExecutorStub(response interface{}) *hookExecutorStub {
	return &hookExecutorStub{
		enabled:  true,
		response: response,
	}
}

func NewDisabledExecutorStub() *hookExecutorStub {
	return &hookExecutorStub{
		enabled:  false,
		response: nil,
	}
}

func NewErrorExecutorStub(err error) *hookExecutorStub {
	return &hookExecutorStub{err: err, enabled: true}
}

// HookExecutorStub is Hook stub to return any given response
type hookExecutorStub struct {
	enabled  bool
	response interface{}
	err      error
}

func (h *hookExecutorStub) IsEnabled() bool {
	return h.enabled
}

func (h *hookExecutorStub) Call(request api.WebhookRequest, response interface{}) error {
	if h.err != nil {
		return h.err
	}

	val := reflect.ValueOf(response)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf(`panic("not a pointer")`)
	}

	val = val.Elem()

	newVal := reflect.Indirect(reflect.ValueOf(h.response))

	if !val.Type().AssignableTo(newVal.Type()) {
		return fmt.Errorf(`panic("mismatched types")`)
	}

	val.Set(newVal)
	return nil
}

func (h hookExecutorStub) Close() {}

type NilCustomizableController struct {
}

func (cc *NilCustomizableController) GetCustomizeHook() *v1alpha1.Hook {
	return nil
}

type FakeCustomizableController struct {
}

func (cc *FakeCustomizableController) GetCustomizeHook() *v1alpha1.Hook {
	url := "fake"
	return &v1alpha1.Hook{
		Webhook: &v1alpha1.Webhook{
			URL: &url,
		},
	}
}

func NewSerializingExecutorStub(responseJson string) hooks.Hook {
	return &serializingHookExecutorStub{response: responseJson}
}

// serializingHookExecutorStub is Hook stub to deserialize given json as response
type serializingHookExecutorStub struct {
	response string
}

func (s serializingHookExecutorStub) IsEnabled() bool {
	return true
}

func (s serializingHookExecutorStub) Call(request api.WebhookRequest, response interface{}) error {
	err := k8sjson.Unmarshal([]byte(s.response), response)
	if err != nil {
		panic(err)
	}
	return nil
}
