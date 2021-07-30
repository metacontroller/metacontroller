package testutils

import (
	"fmt"
	"metacontroller/pkg/hooks"
	"reflect"
)

// NewHookExecutorStub creates new HookExecutorStub which returns given response
func NewHookExecutorStub(response interface{}) hooks.HookExecutor {
	return &hookExecutorStub{
		enabled:  true,
		response: response,
	}
}

// HookExecutorStub is HookExecutor stub to return any given response
type hookExecutorStub struct {
	enabled  bool
	response interface{}
}

func (h *hookExecutorStub) IsEnabled() bool {
	return true
}

func (h *hookExecutorStub) Execute(request interface{}, response interface{}) error {
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
