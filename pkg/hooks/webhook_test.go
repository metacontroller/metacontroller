package hooks

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"metacontroller/pkg/controller/common"
	compositev1 "metacontroller/pkg/controller/composite/api/v1"
	"metacontroller/pkg/logging"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"k8s.io/utils/pointer"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

type MockResponse struct {
	resp *http.Response
	err  error
}

type CallResult struct {
	res compositev1.CompositeHookResponse
	err error
}

type TestStep struct {
	httpResponse    *MockResponse
	expectedHeaders map[string]string
	hookResult      *CallResult
}

type TestSteps []TestStep

type httpClientMock struct {
	doResponses     []*MockResponse
	expectedHeaders []map[string]string
	doIdx           int
}

func (m *httpClientMock) Do(req *http.Request) (*http.Response, error) {
	if m.doIdx < len(m.doResponses) {
		m.doIdx++
	}

	if m.expectedHeaders != nil && m.expectedHeaders[m.doIdx-1] != nil {
		for name, expectedValue := range m.expectedHeaders[m.doIdx-1] {
			if expectedValue == "*" {
				if req.Header.Values(name) == nil {
					panic("expected header is missing")
				}
			} else {
				headerValue := req.Header.Get(name)
				if headerValue != expectedValue {
					panic(fmt.Sprintf("expected header %s has value %s while expected to be %s", name, headerValue, expectedValue))
				}
			}
		}
	}

	data := m.doResponses[m.doIdx-1]
	return data.resp, data.err
}

func TestNewHookExecutor_whenNilWebHook_returnNilWebhookExecutor(t *testing.T) {
	executor, err := NewWebhookExecutor(nil, "", common.CompositeController, "")

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
				Timeout: &metav1.Duration{},
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
				Timeout: &metav1.Duration{Duration: -2 * time.Second},
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
				Timeout: &metav1.Duration{Duration: 2 * time.Second},
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

func httpResponse304NotModified() *MockResponse {
	return &MockResponse{
		&http.Response{
			Status:     "304 Not modified",
			StatusCode: 304,
			Header:     map[string][]string{},
			Body:       ioutil.NopCloser(bytes.NewBufferString("")),
		},
		nil,
	}
}

func httpResponse500(resp string) *MockResponse {
	return &MockResponse{
		&http.Response{
			Status:     "500 Server Internal Error",
			StatusCode: 500,
			Header:     map[string][]string{},
			Body:       ioutil.NopCloser(bytes.NewBufferString(resp)),
		},
		nil,
	}
}

func httpResponse200(body string, etag string) *MockResponse {
	t := http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Header:     map[string][]string{},
		Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
	}

	switch etag {
	case "":
		break
	default:
		t.Header.Set("Etag", etag)
	}

	return &MockResponse{
		&t,
		nil,
	}
}

func hookResultFromJson(body string) *CallResult {
	var res compositev1.CompositeHookResponse
	err := json.Unmarshal([]byte(body), &res)
	if err != nil {
		panic("unmarshal failed")
	}
	return &CallResult{
		res: res,
		err: err,
	}
}

func TestHookETag(t *testing.T) {
	body1 := `
		{
		  "resyncAfterSeconds": 30,
		  "children": [
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name"},
			  "spec": {"key": "val"}
			},
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name2"},
			  "spec": {"key": "val2"}
			}
		  ]
		}`
	body2 := `
		{
		  "resyncAfterSeconds": 30,
		  "children": [
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name"},
			  "spec": {"key": "val2"}
			},
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name2"},
			  "spec": {"key": "val2","key2": "val3"}
			}
		  ]
		}`
	RunHookTest(t, TestSteps{
		{
			// Check if non-200 responses are no tolerated when no cache
			httpResponse:    httpResponse500("Internal Server Error"),
			expectedHeaders: nil,
			hookResult: &CallResult{
				res: compositev1.CompositeHookResponse{},
				err: fmt.Errorf("remote error: %s", "Internal Server Error"),
			},
		},
		{
			httpResponse: httpResponse200(body1, "000-000-000-001"),
			hookResult:   hookResultFromJson(body1),
		},
		{
			// Check if non-200 responses are not breaking the cache
			httpResponse: httpResponse500("Internal Server Error"),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-001",
			},
			hookResult: &CallResult{
				res: compositev1.CompositeHookResponse{},
				err: fmt.Errorf("remote error: %s", "Internal Server Error"),
			},
		},
		{
			httpResponse: httpResponse304NotModified(),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-001",
			},
			hookResult: hookResultFromJson(body1),
		},
		{
			// Check if non-200 responses are no tolerated when cache is present
			httpResponse: httpResponse500("Internal Server Error"),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-001",
			},
			hookResult: &CallResult{
				res: compositev1.CompositeHookResponse{},
				err: fmt.Errorf("remote error: %s", "Internal Server Error"),
			},
		},
		{
			httpResponse: httpResponse200(body2, "000-000-000-002"),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-001",
			},
			hookResult: hookResultFromJson(body2),
		},
		{
			httpResponse: httpResponse200(body2, "000-000-000-002"),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-002",
			},
			hookResult: hookResultFromJson(body2),
		},
		{
			httpResponse: httpResponse304NotModified(),
			hookResult:   hookResultFromJson(body2),
		},
	})
}

func RunHookTest(t *testing.T, steps TestSteps) {
	logging.InitLogging(&zap.Options{})
	responses := make([]*MockResponse, len(steps))
	expectedHeaders := make([]map[string]string, len(steps))
	for id := range steps {
		responses[id] = steps[id].httpResponse
		expectedHeaders[id] = steps[id].expectedHeaders
	}

	parent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Some",
			"metadata": map[string]interface{}{
				"name":      "name",
				"namespace": "default",
			},
		},
	}

	request := &compositev1.CompositeHookRequest{
		Controller: nil,
		Parent:     parent,
	}

	boolTrue := true

	hook, err := newWebhookExecutorWithCustomHttpClient(
		&v1alpha1.Webhook{
			URL:     pointer.StringPtr(""),
			Timeout: &metav1.Duration{Duration: 2 * time.Second},
			Path:    new(string),
			Service: &v1alpha1.ServiceReference{},
			Etag: &v1alpha1.WebhookEtagConfig{
				Enabled: &boolTrue,
			},
		},
		common.CustomizeHook,
		&httpClientMock{
			responses,
			expectedHeaders,
			0,
		},
		"",
	)
	assert.NoError(t, err)

	for id := range steps {
		expected := steps[id].hookResult
		response := compositev1.CompositeHookResponse{}
		err = hook.Call(request, &response)
		assert.Equal(t, expected.err, err)
		assert.Equal(t, expected.res, response)
	}
}
