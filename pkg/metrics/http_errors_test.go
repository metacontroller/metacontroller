/*
Copyright 2026 Metacontroller authors.

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

package metrics

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metacontroller/pkg/controller/common"
)

const requestErrorsMetric = "metacontroller_webhook_request_errors_total"

type roundTripperStub struct {
	response *http.Response
	err      error
}

func (s roundTripperStub) RoundTrip(*http.Request) (*http.Response, error) {
	return s.response, s.err
}

func TestRequestErrorReason_whenTransportError_returnsReason(t *testing.T) {
	dialErr := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"canceled", context.Canceled, errorReasonCanceled},
		{"dns", &net.DNSError{Err: "no such host", Name: "wrongdomain.com", IsNotFound: true}, errorReasonDNS},
		{"dns timeout", &net.DNSError{Err: "timeout", Name: "domain.com", IsTimeout: true}, errorReasonDNS},
		{"unknown authority", x509.UnknownAuthorityError{}, errorReasonTLS},
		{"hostname mismatch", x509.HostnameError{Certificate: &x509.Certificate{}, Host: "domain.com"}, errorReasonTLS},
		{"not TLS server", tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, errorReasonTLS},
		{"remote TLS alert", &net.OpError{Op: tlsRemoteErrorOp, Err: errors.New("tls: bad certificate")}, errorReasonTLS},
		{"deadline", context.DeadlineExceeded, errorReasonTimeout},
		{"connection refused", dialErr, errorReasonConnection},
		{"wrapped", fmt.Errorf("wrapped: %w", dialErr), errorReasonConnection},
		{"unknown", io.ErrUnexpectedEOF, errorReasonOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, requestErrorReason(tt.err))
		})
	}
}

func TestInstrumentClient_whenTransportFails_countsRequestError(t *testing.T) {
	hookURL := "https://wrongdomain.invalid/sync"
	wantErr := &net.DNSError{Err: "no such host", Name: "wrongdomain.invalid", IsNotFound: true}
	client := instrumentedClient(t, "transport-error", hookURL, roundTripperStub{err: wantErr})

	_, err := client.Post(hookURL, "application/json", strings.NewReader("{}")) //nolint:bodyclose,noctx

	var urlErr *url.Error
	require.ErrorAs(t, err, &urlErr)
	assert.Equal(t, wantRequestErrors(errorReasonDNS), requestErrorSeries(t, "transport-error"))
}

func TestInstrumentClient_whenServerReturnsError_doesNotCountRequestError(t *testing.T) {
	hookURL := "https://domain.invalid/sinc"
	stub := roundTripperStub{response: &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       http.NoBody,
		Header:     http.Header{},
	}}
	client := instrumentedClient(t, "http-error", hookURL, stub)

	response, err := client.Post(hookURL, "application/json", strings.NewReader("{}")) //nolint:noctx
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	assert.Equal(t, wantRequestErrors(), requestErrorSeries(t, "http-error"))
}

// wantRequestErrors returns all reasons at zero, and one error for each given
// reason.
func wantRequestErrors(reasons ...string) map[string]float64 {
	want := map[string]float64{}
	for _, reason := range requestErrorReasons {
		want[reason] = 0
	}
	for _, reason := range reasons {
		want[reason]++
	}
	return want
}

func instrumentedClient(t *testing.T, controllerName, hookURL string, transport http.RoundTripper) *http.Client {
	t.Helper()

	client, err := InstrumentClientWithConstLabels(
		controllerName,
		common.CompositeController,
		common.SyncHook,
		&http.Client{Transport: transport},
		hookURL)
	require.NoError(t, err)
	return client
}

// requestErrorSeries reads the request error series of one controller from the
// metrics registry. The result maps each reason to its value.
func requestErrorSeries(t *testing.T, controllerName string) map[string]float64 {
	t.Helper()

	families, err := registerer.Gather()
	require.NoError(t, err)
	series := map[string]float64{}
	for _, family := range families {
		if family.GetName() != requestErrorsMetric {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels[controllerNameLabel] == controllerName && labels["hook"] == common.SyncHook.String() {
				series[labels["reason"]] = metric.GetCounter().GetValue()
			}
		}
	}
	return series
}
