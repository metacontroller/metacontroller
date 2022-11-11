package hooks

import (
	"bytes"
	"io"
	"metacontroller/pkg/controller/common"
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"metacontroller/pkg/logging"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/go-logr/logr/testr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

func TestNewHookExecutor_whenNilWebHook_returnNilWebhookExecutor(t *testing.T) {
	executor, err := NewWebhookExecutor(nil, "", common.CompositeController, "")

	assert.NoError(t, err)

	assert.Nil(t, executor)
}

func TestWebhookTimeout_defaultTimeoutIfNotSpecified(t *testing.T) {
	tables := []struct {
		webhook  v1alpha1.Webhook
		duration time.Duration
	}{
		{
			v1alpha1.Webhook{
				URL:     pointer.String(""),
				Timeout: &metav1.Duration{},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			10 * time.Second,
		},
	}

	for _, table := range tables {
		duration, _ := webhookTimeout(&table.webhook)
		assert.Equal(t, table.duration, duration, "Duration was incorrect")
	}
}
func TestWebhookTimeout_defaultTimeoutIfNegative(t *testing.T) {
	tables := []struct {
		webhook  v1alpha1.Webhook
		duration time.Duration
	}{
		{
			v1alpha1.Webhook{
				URL:     pointer.String(""),
				Timeout: &metav1.Duration{Duration: -2 * time.Second},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			10 * time.Second,
		},
	}

	for _, table := range tables {
		duration, _ := webhookTimeout(&table.webhook)
		assert.Equal(t, table.duration, duration, "Duration was incorrect")
	}
}

func TestWebhookTimeout_givenTimeoutIfPositive(t *testing.T) {
	tables := []struct {
		webhook  v1alpha1.Webhook
		duration time.Duration
	}{
		{
			v1alpha1.Webhook{
				URL:     pointer.String(""),
				Timeout: &metav1.Duration{Duration: 2 * time.Second},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			2 * time.Second,
		},
	}

	for _, table := range tables {
		duration, _ := webhookTimeout(&table.webhook)
		assert.Equal(t, table.duration, duration, "Duration was incorrect")
	}
}

type clientMock struct {
	jsonResponse string
}

func NewHttpClientMockWithResponse(jsonResponse string) *clientMock {
	return &clientMock{
		jsonResponse: jsonResponse,
	}
}

func (c *clientMock) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(c.jsonResponse)),
	}, nil
}

func Test_when_incorrectJsonResponseInLooseMode_deserializeToEmptyResponse(t *testing.T) {
	logging.Logger = testr.New(t)
	webhookExecutor := newWebhookExecutor(
		NewHttpClientMockWithResponse(`{"some": "sother"}`),
		"",
		common.CustomizeHook,
		nil,
		&webhookExecutorPlain{},
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.NoError(t, err)
}

func Test_when_incorrectJsonResponseInStrictMode_thrownError(t *testing.T) {
	logging.Logger = testr.New(t)
	webhookExecutor := newWebhookExecutor(
		NewHttpClientMockWithResponse(`{"some": "sother"}`),
		"",
		common.CustomizeHook,
		toPointer(v1alpha1.ResponseUnmarshallModeStrict),
		&webhookExecutorPlain{},
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.Error(t, err)
}

func toPointer(mode v1alpha1.ResponseUnmarshallMode) *v1alpha1.ResponseUnmarshallMode {
	return &mode
}
