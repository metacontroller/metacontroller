package hooks

import (
	"testing"
	"time"

	"k8s.io/utils/pointer"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewHookExecutor_whenNilWebHook_returnNilWebhookExecutor(t *testing.T) {
	executor, err := NewWebhookExecutor(nil, "")

	if err != nil {
		t.Errorf("err should be nil, got: %v", err)
	}

	if executor != nil {
		t.Errorf("WebhookExecutor should be nil")
	}
}

func TestWebhookTimeout_defaultTimeoutIfNotSpecified(t *testing.T) {
	tables := []struct {
		webhook  v1alpha1.Webhook
		duration time.Duration
	}{
		{
			v1alpha1.Webhook{
				URL:     pointer.StringPtr(""),
				Timeout: &v1.Duration{},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			10 * time.Second,
		},
	}

	for _, table := range tables {
		duration, _ := webhookTimeout(&table.webhook)
		if duration != table.duration {
			t.Errorf("Duration was incorrect, got: %d, want: %d.", duration, table.duration)
		}
	}
}
func TestWebhookTimeout_defaultTimeoutIfNegative(t *testing.T) {
	tables := []struct {
		webhook  v1alpha1.Webhook
		duration time.Duration
	}{
		{
			v1alpha1.Webhook{
				URL:     pointer.StringPtr(""),
				Timeout: &v1.Duration{Duration: -2 * time.Second},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			10 * time.Second,
		},
	}

	for _, table := range tables {
		duration, _ := webhookTimeout(&table.webhook)
		if duration != table.duration {
			t.Errorf("Duration was incorrect, got: %d, want: %d.", duration, table.duration)
		}
	}
}

func TestWebhookTimeout_givenTimeoutIfPositive(t *testing.T) {
	tables := []struct {
		webhook  v1alpha1.Webhook
		duration time.Duration
	}{
		{
			v1alpha1.Webhook{
				URL:     pointer.StringPtr(""),
				Timeout: &v1.Duration{Duration: 2 * time.Second},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			2 * time.Second,
		},
	}

	for _, table := range tables {
		duration, _ := webhookTimeout(&table.webhook)
		if duration != table.duration {
			t.Errorf("Duration was incorrect, got: %d, want: %d.", duration, table.duration)
		}
	}
}
