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
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metacontroller/pkg/controller/common"
)

const shortTimeout = 100 * time.Millisecond

// TestInstrumentClient_whenRealTransportFails_countsReason uses the real HTTP
// transport. The errors of the transport are not simulated.
func TestInstrumentClient_whenRealTransportFails_countsReason(t *testing.T) {
	hanging := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// The server finds a closed connection only after it reads the body.
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(hanging.Close)
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(plain.Close)
	untrusted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(untrusted.Close)

	tests := []struct {
		name          string
		url           string
		clientTimeout time.Duration
		cancelAfter   time.Duration
		want          string
	}{
		{name: "dns", url: "http://wrongdomain.invalid/sync", clientTimeout: 10 * time.Second, want: errorReasonDNS},
		{name: "client timeout", url: hanging.URL, clientTimeout: shortTimeout, want: errorReasonTimeout},
		{name: "caller cancel", url: hanging.URL, clientTimeout: 10 * time.Second, cancelAfter: shortTimeout, want: errorReasonCanceled},
		{name: "plain http on tls", url: strings.Replace(plain.URL, "http://", "https://", 1), clientTimeout: 10 * time.Second, want: errorReasonTLS},
		{name: "untrusted certificate", url: untrusted.URL, clientTimeout: 10 * time.Second, want: errorReasonTLS},
		{name: "connection refused", url: "http://" + closedAddress(t), clientTimeout: 10 * time.Second, want: errorReasonConnection},
		{name: "unsupported scheme", url: "ftp://domain.invalid/sync", clientTimeout: 10 * time.Second, want: errorReasonOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerName := "real-transport-" + strings.ReplaceAll(tt.name, " ", "-")
			transport := &http.Transport{}
			t.Cleanup(transport.CloseIdleConnections)
			client := instrumentedClient(t, controllerName, tt.url, transport)
			client.Timeout = tt.clientTimeout

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelAfter > 0 {
				time.AfterFunc(tt.cancelAfter, cancel)
			}
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, tt.url, strings.NewReader("{}"))
			require.NoError(t, err)

			response, err := client.Do(request)
			if response != nil {
				require.NoError(t, response.Body.Close())
			}

			require.Error(t, err)
			assert.Equal(t, wantRequestErrors(tt.want), requestErrorSeries(t, controllerName), err.Error())
		})
	}
}

func TestRoundTripErrorReason_whenDeadlinePassed_returnsTimeout(t *testing.T) {
	canceledErr := errors.New("net/http: request canceled")
	dnsErr := &net.DNSError{Err: "no such host", Name: "domain.invalid", IsNotFound: true}
	tests := []struct {
		name     string
		deadline time.Time
		err      error
		want     string
	}{
		{"plain error after deadline", time.Now().Add(-time.Second), canceledErr, errorReasonTimeout},
		{"context canceled after deadline", time.Now().Add(-time.Second), context.Canceled, errorReasonTimeout},
		{"dns error after deadline", time.Now().Add(-time.Second), dnsErr, errorReasonDNS},
		{"plain error before deadline", time.Now().Add(time.Hour), canceledErr, errorReasonOther},
		{"context canceled before deadline", time.Now().Add(time.Hour), context.Canceled, errorReasonCanceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), tt.deadline)
			defer cancel()
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://domain.invalid", http.NoBody)
			require.NoError(t, err)

			assert.Equal(t, tt.want, roundTripErrorReason(request, tt.err))
		})
	}
}

func TestInstrumentClient_whenControllerTypesShareNameAndURL_keepsSeparateSeries(t *testing.T) {
	const controllerName = "shared-name"
	hookURL := "https://shared.invalid/sync"
	transportErr := &net.DNSError{Err: "no such host", Name: "shared.invalid", IsNotFound: true}
	clients := map[common.ControllerType]*http.Client{}
	for _, controllerType := range []common.ControllerType{common.CompositeController, common.DecoratorController} {
		client, err := InstrumentClientWithConstLabels(
			controllerName, controllerType, common.SyncHook,
			&http.Client{Transport: roundTripperStub{err: transportErr}}, hookURL)
		require.NoError(t, err)
		clients[controllerType] = client
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, hookURL, http.NoBody)
	require.NoError(t, err)
	response, err := clients[common.DecoratorController].Do(request)
	if response != nil {
		require.NoError(t, response.Body.Close())
	}
	require.Error(t, err)

	assert.Equal(t, 0.0, dnsErrorsOfType(t, controllerName, common.CompositeController))
	assert.Equal(t, 1.0, dnsErrorsOfType(t, controllerName, common.DecoratorController))
}

// dnsErrorsOfType reads the DNS error series of one controller type.
func dnsErrorsOfType(t *testing.T, controllerName string, controllerType common.ControllerType) float64 {
	t.Helper()

	families, err := registerer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != requestErrorsMetric {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels[controllerNameLabel] == controllerName &&
				labels["controller_type"] == controllerType.String() &&
				labels["reason"] == errorReasonDNS {
				return metric.GetCounter().GetValue()
			}
		}
	}
	t.Fatalf("no DNS series for controller %q of type %q", controllerName, controllerType)
	return 0
}

// closedAddress returns a local address that refuses connections.
func closedAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}
