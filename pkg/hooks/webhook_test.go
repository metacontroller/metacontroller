package hooks

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"metacontroller/pkg/controller/common"
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"metacontroller/pkg/logging"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-logr/logr/testr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

func TestNewHookExecutor_whenNilWebHook_returnNilWebhookExecutor(t *testing.T) {
	executor, err := NewWebhookExecutor(nil, nil, "", common.CompositeController, "", nil)

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
		"",
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
		"",
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
		"",
		time.Now,
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.Error(t, err, "V1 strict mode should error on unknown fields")
	assert.Contains(t, err.Error(), "strict validation failed for v1 webhookResponse", "Error message should indicate v1 strict failure")
}

func TestV2StrictByDefault(t *testing.T) {
	logging.Logger = testr.New(t)
	v2Version := v1alpha1.HookVersionV2
	// Test default behavior (strict for V2)
	webhookExecutor := newWebhookExecutor(
		newHttpClientMockWithResponse(`{"unknownField": "value"}`),
		"http://localhost/v2strict",
		common.CustomizeHook,
		&v2Version,
		nil, // Default mode for V2 should be strict
		&webhookExecutorPlain{},
		"",
		time.Now,
	)

	var response v1.CustomizeHookResponse
	err := webhookExecutor.Call(nil, &response)
	assert.Error(t, err, "V2 should default to strict mode and error on unknown fields")
	assert.Contains(t, err.Error(), "strict validation failed for v2", "Error message should indicate v2 strict failure")

	// Test that Loose mode is honored for V2
	webhookExecutorLoose := newWebhookExecutor(
		newHttpClientMockWithResponse(`{"unknownField": "value"}`),
		"http://localhost/v2loose",
		common.CustomizeHook,
		&v2Version,
		toPointer(v1alpha1.ResponseUnmarshallModeLoose),
		&webhookExecutorPlain{},
		"",
		time.Now,
	)

	err = webhookExecutorLoose.Call(nil, &response)
	assert.NoError(t, err, "V2 should honor ResponseUnmarshallModeLoose")
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
		"",
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
			executor := newWebhookExecutor(mockClient, "http://test.com", common.SyncHook, tt.hookVersion, nil, &webhookExecutorPlain{}, "", time.Now)

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
				"",
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

func TestNewWebhookExecutor_TLSEnforcement(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cert, err := x509.ParseCertificate(srv.TLS.Certificates[0].Certificate[0])
	require.NoError(t, err)
	var pemBuf bytes.Buffer
	require.NoError(t, pem.Encode(&pemBuf, &pem.Block{Type: pemTypeCert, Bytes: cert.Raw}))
	serverCAPEM := pemBuf.Bytes()

	url := srv.URL + "/sync"

	tests := []struct {
		name            string
		caBundle        []byte
		wantErrContains string
	}{
		{
			name:     "correct CA bundle, call succeeds",
			caBundle: serverCAPEM,
		},
		{
			name:            "absent CA bundle, call fails with certificate error",
			caBundle:        nil,
			wantErrContains: "certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := &v1alpha1.Webhook{URL: &url}
			var conn *ResolvedEndpointConfig
			if len(tt.caBundle) > 0 {
				conn = &ResolvedEndpointConfig{CABundle: tt.caBundle}
			}
			executor, err := NewWebhookExecutor(webhook, nil, "test-controller", common.CompositeController, "sync", conn)
			require.NoError(t, err)

			var response struct{}
			err = executor.Call(nil, &response)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewWebhookExecutor_AuthHeader_sentToServer(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	url := srv.URL + "/sync"
	webhook := &v1alpha1.Webhook{URL: &url}
	conn := &ResolvedEndpointConfig{AuthHeader: "Bearer secret-token"}
	executor, err := NewWebhookExecutor(webhook, nil, "test-controller", common.CompositeController, common.SyncHook, conn)
	require.NoError(t, err)

	var response struct{}
	require.NoError(t, executor.Call(nil, &response))
	assert.Equal(t, "Bearer secret-token", receivedAuth)
}

func TestNewWebhookExecutor_ClientTLS_presentedDuringHandshake(t *testing.T) {
	certPEM, keyPEM := generateClientCertPEMs(t)

	expectedCert, err := x509.ParseCertificate(mustParsePEMBlock(t, certPEM).Bytes)
	require.NoError(t, err)

	var receivedCert *x509.Certificate
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			receivedCert = r.TLS.PeerCertificates[0]
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	srv.StartTLS()
	defer srv.Close()

	// Build the CA PEM from the server's self-signed cert so we trust it.
	cert, err := x509.ParseCertificate(srv.TLS.Certificates[0].Certificate[0])
	require.NoError(t, err)
	var pemBuf bytes.Buffer
	require.NoError(t, pem.Encode(&pemBuf, &pem.Block{Type: pemTypeCert, Bytes: cert.Raw}))

	clientCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	url := srv.URL + "/sync"
	webhook := &v1alpha1.Webhook{URL: &url}
	conn := &ResolvedEndpointConfig{
		CABundle:   pemBuf.Bytes(),
		ClientCert: &clientCert,
	}
	executor, err := NewWebhookExecutor(webhook, nil, "test-controller", common.CompositeController, common.SyncHook, conn)
	require.NoError(t, err)

	var response struct{}
	require.NoError(t, executor.Call(nil, &response))
	require.NotNil(t, receivedCert, "server should have received a client certificate")
	assert.Equal(t, expectedCert.Raw, receivedCert.Raw, "server should have received the correct client certificate")
}

// mustParsePEMBlock decodes the first PEM block from data and fails the test if
// none is found.
func mustParsePEMBlock(t *testing.T, data []byte) *pem.Block {
	t.Helper()
	block, _ := pem.Decode(data)
	require.NotNil(t, block, "no PEM block found")
	return block
}

// TestNewWebhookExecutor_InvalidConfig verifies that a webhook with neither a
// full URL nor both service and path fails construction with a clear error
// rather than silently producing an unusable executor. It also checks that
// each distinct misconfiguration produces a specific message so operators
// are not misdirected by a single generic hint.
func TestNewWebhookExecutor_InvalidConfig(t *testing.T) {
	t.Run("missing url and service/path", func(t *testing.T) {
		webhook := &v1alpha1.Webhook{} // no URL, no Service, no Path
		executor, err := NewWebhookExecutor(webhook, nil, "test-controller", common.CompositeController, common.SyncHook, nil)
		require.Error(t, err)
		assert.Nil(t, executor)
		assert.Contains(t, err.Error(), "invalid webhook config")
	})

	t.Run("service missing name/namespace", func(t *testing.T) {
		webhook := &v1alpha1.Webhook{
			Service: &v1alpha1.ServiceReference{},
			Path:    ptr.To[string]("/hook"),
		}
		executor, err := NewWebhookExecutor(webhook, nil, "test-controller", common.CompositeController, common.SyncHook, nil)
		require.Error(t, err)
		assert.Nil(t, executor)
		assert.Contains(t, err.Error(), "invalid webhook config")
		assert.Contains(t, err.Error(), "name")
		assert.Contains(t, err.Error(), "namespace")
	})
}
