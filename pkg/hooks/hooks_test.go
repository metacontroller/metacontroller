package hooks

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	"testing"
)

func TestNewHookExecutor_whenNilHook_returnDisabledHookExecutor(t *testing.T) {
	executor, err := NewHook(nil, "", common.CompositeController, "")

	if err != nil {
		t.Errorf("err should be nil, got: %v", err)
	}

	if executor.IsEnabled() {
		t.Errorf("Hook should be disabled")
	}
}

func TestNewHookExecutor_whenHookWithNilWebhook_returnDisabledHookExecutor(t *testing.T) {
	executor, err := NewHook(&v1alpha1.Hook{
		Webhook: nil},
		"", common.CompositeController, "")

	if err != nil {
		t.Errorf("err should be nil, got: %v", err)
	}

	if executor.IsEnabled() {
		t.Errorf("Hook should be disabled")
	}
}
