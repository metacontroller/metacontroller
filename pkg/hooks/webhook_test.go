package hooks

import (
	"bytes"
	"fmt"
	"io"
	"metacontroller/pkg/controller/common"
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"metacontroller/pkg/logging"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"github.com/go-logr/logr/testr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

func TestNewHookExecutor_whenNilWebHook_returnNilWebhookExecutor(t *testing.T) {
	executor, err := NewWebhookExecutor(nil, nil, "", common.CompositeController, "")

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
				URL:     ptr.To[string](""),
				Timeout: &metav1.Duration{},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			10 * time.Second,
		},
	}

	for _, table := range tables {
		webhook := table.webhook
		duration, _ := webhookTimeout(&webhook)
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
				URL:     ptr.To[string](""),
				Timeout: &metav1.Duration{Duration: -2 * time.Second},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			10 * time.Second,
		},
	}

	for _, table := range tables {
		webhook := table.webhook
		duration, _ := webhookTimeout(&webhook)
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
				URL:     ptr.To[string](""),
				Timeout: &metav1.Duration{Duration: 2 * time.Second},
				Path:    new(string),
				Service: &v1alpha1.ServiceReference{},
			},
			2 * time.Second,
		},
	}

	for _, table := range tables {
		webhook := table.webhook
		duration, _ := webhookTimeout(&webhook)
		assert.Equal(t, table.duration, duration, "Duration was incorrect")
	}
}

type clientMock struct {
	jsonResponse string
	statusCode   int
	headers      http.Header
}

func newHttpClientMockWithResponse(jsonResponse string) *clientMock {
	return &clientMock{
		jsonResponse: jsonResponse,
		statusCode:   http.StatusOK,
		headers:      map[string][]string{},
	}
}

func newHttpClientMockWith429(retryAfter string) *clientMock {
	return &clientMock{
		jsonResponse: `{"some": "sother"}`,
		statusCode:   http.StatusTooManyRequests,
		headers: map[string][]string{
			"Retry-After": {retryAfter},
		},
	}
}

func (c *clientMock) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: c.statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(c.jsonResponse)),
		Header:     c.headers,
	}, nil
}

func Test_when_incorrectJsonResponseInLooseMode_deserializeToEmptyResponse(t *testing.T) {
	logging.Logger = testr.New(t)
	webhookExecutor := newWebhookExecutor(
		newHttpClientMockWithResponse(`{"some": "sother"}`),
		"",
		common.CustomizeHook,
		nil, // hookVersion (defaults to v1)
		nil,
		&webhookExecutorPlain{},
		time.Now,
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.NoError(t, err)
}

func Test_when_incorrectJsonResponseInLooseMode_V1_deserializeToEmptyResponse(t *testing.T) {
	logging.Logger = testr.New(t)
	v1Version := v1alpha1.HookVersionV1
	webhookExecutor := newWebhookExecutor(
		newHttpClientMockWithResponse(`{"unknownField": "value"}`),
		"http://localhost/v1loose",
		common.CustomizeHook,
		&v1Version,
		toPointer(v1alpha1.ResponseUnmarshallModeLoose),
		&webhookExecutorPlain{},
		time.Now,
	)

	var response v1.CustomizeHookResponse // Simple structure without 'unknownField'
	err := webhookExecutor.Call(nil, &response)
	assert.NoError(t, err, "V1 loose mode should not error on unknown fields")
}

func Test_when_incorrectJsonResponseInStrictMode_V1_throwsError(t *testing.T) {
	logging.Logger = testr.New(t)
	v1Version := v1alpha1.HookVersionV1
	webhookExecutor := newWebhookExecutor(
		newHttpClientMockWithResponse(`{"unknownField": "value"}`),
		"http://localhost/v1strict",
		common.CustomizeHook,
		&v1Version,
		toPointer(v1alpha1.ResponseUnmarshallModeStrict),
		&webhookExecutorPlain{},
		time.Now,
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.Error(t, err, "V1 strict mode should error on unknown fields")
	assert.Contains(t, err.Error(), "strict validation failed for V1 webhookResponse", "Error message should indicate V1 strict failure")
}

func Test_when_incorrectJsonResponse_V2_alwaysThrowsError(t *testing.T) {
	logging.Logger = testr.New(t)
	v2Version := v1alpha1.HookVersionV2
	// ResponseUnmarshallModeLoose should be ignored for V2
	webhookExecutor := newWebhookExecutor(
		newHttpClientMockWithResponse(`{"unknownField": "value"}`),
		"http://localhost/v2strict",
		common.CustomizeHook,
		&v2Version,
		toPointer(v1alpha1.ResponseUnmarshallModeLoose), // This mode should be ignored for v2
		&webhookExecutorPlain{},
		time.Now,
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.Error(t, err, "V2 should always error on unknown fields regardless of ResponseUnmarshallMode")
	assert.Contains(t, err.Error(), "strict validation failed for V2 webhookResponse", "Error message should indicate V2 strict failure")
}

func Test_when_incorrectJsonResponseInStrictMode_thrownError(t *testing.T) {
	logging.Logger = testr.New(t)
	webhookExecutor := newWebhookExecutor(
		newHttpClientMockWithResponse(`{"some": "sother"}`),
		"",
		common.CustomizeHook,
		nil, // hookVersion (defaults to v1)
		toPointer(v1alpha1.ResponseUnmarshallModeStrict),
		&webhookExecutorPlain{},
		time.Now,
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.Error(t, err)
}

func TestWebhookExecutor_Call_WithDifferentVersions(t *testing.T) {
	tests := []struct {
		name            string
		hookVersion     *v1alpha1.HookVersion
		expectedVersion v1alpha1.HookVersion
	}{
		{"v1 version", ptr.To(v1alpha1.HookVersionV1), v1alpha1.HookVersionV1},
		{"v2 version", ptr.To(v1alpha1.HookVersionV2), v1alpha1.HookVersionV2},
		{"nil version (defaults to v1)", nil, v1alpha1.HookVersionV1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newHttpClientMockWithResponse(`{}`) // Empty JSON response
			executor := newWebhookExecutor(mockClient, "http://test.com", common.SyncHook, tt.hookVersion, nil, &webhookExecutorPlain{}, time.Now)

			var respData map[string]interface{}
			err := executor.Call(nil, &respData)
			assert.NoError(t, err)

			// Verify the effective version is calculated correctly
			effectiveVersion := executor.effectiveHookVersion()
			assert.Equal(t, tt.expectedVersion, effectiveVersion, "Effective hook version should match expected")
		})
	}
}

func Test429Response_thrown_TooManyRequestError(t *testing.T) {
	logging.Logger = testr.New(t)
	now := time.Now()
	expectAfterSecond := 10
	tests := []struct {
		name       string
		retryAfter string
	}{
		{
			name:       "is number",
			retryAfter: strconv.Itoa(expectAfterSecond),
		},
		{
			name:       "is http date format",
			retryAfter: now.Add(time.Duration(expectAfterSecond) * time.Second).Format(time.RFC1123),
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("error with right retryAfter seconds when backend service return 429 given retryAfter %s", tt.name), func(t *testing.T) {
			webhookExecutor := newWebhookExecutor(
				newHttpClientMockWith429(tt.retryAfter),
				"",
				common.CustomizeHook,
				nil, // hookVersion
				toPointer(v1alpha1.ResponseUnmarshallModeStrict),
				&webhookExecutorPlain{},
				func() time.Time {
					return now
				},
			)

			err := webhookExecutor.Call(nil, &v1.CustomizeHookResponse{})
			var tooManyRequestError *TooManyRequestError
			errors.As(err, &tooManyRequestError)
			assert.Equal(t, expectAfterSecond, tooManyRequestError.AfterSecond)
		})
	}
}

func toPointer(mode v1alpha1.ResponseUnmarshallMode) *v1alpha1.ResponseUnmarshallMode {
	return &mode
}
