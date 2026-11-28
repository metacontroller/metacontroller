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
	"maps"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	pph "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Values of the "reason" label on the request_errors_total counter.
const (
	errorReasonCanceled   = "canceled"
	errorReasonDNS        = "dns"
	errorReasonTLS        = "tls"
	errorReasonTimeout    = "timeout"
	errorReasonConnection = "connection"
	errorReasonOther      = "other"
)

// tlsRemoteErrorOp is the net.OpError operation for a TLS alert from the peer.
const tlsRemoteErrorOp = "remote error"

var requestErrorReasons = []string{
	errorReasonCanceled,
	errorReasonDNS,
	errorReasonTLS,
	errorReasonTimeout,
	errorReasonConnection,
	errorReasonOther,
}

// newRequestErrorsCounter returns the counter for requests that end without an
// HTTP response. The requests_total counter does not count these requests,
// because it needs a status code. All reasons start at zero. Thus a rate query
// also shows the first error.
func newRequestErrorsCounter(hookType string, constLabels map[string]string) *prometheus.CounterVec {
	labels := maps.Clone(constLabels)
	labels["hook"] = hookType
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   metacontrollerPrefix,
			Subsystem:   "webhook",
			Name:        "request_errors_total",
			Help:        "A counter for outgoing webhook requests that ended without an HTTP response.",
			ConstLabels: labels,
		},
		[]string{"reason"},
	)
	for _, reason := range requestErrorReasons {
		counter.WithLabelValues(reason)
	}
	return counter
}

// instrumentRoundTripperErrors counts each error that next returns.
func instrumentRoundTripperErrors(counter *prometheus.CounterVec, next http.RoundTripper) pph.RoundTripperFunc {
	return func(r *http.Request) (*http.Response, error) {
		resp, err := next.RoundTrip(r)
		if err != nil {
			counter.WithLabelValues(roundTripErrorReason(r, err)).Inc()
		}
		return resp, err
	}
}

// roundTripErrorReason returns the label value for an error of one round trip.
// When http.Client.Timeout expires, the transport can return a plain "request
// canceled" error. Thus use the deadline of the request to find a timeout.
func roundTripErrorReason(r *http.Request, err error) string {
	reason := requestErrorReason(err)
	if reason == errorReasonDNS || reason == errorReasonTLS {
		return reason
	}
	if deadline, ok := r.Context().Deadline(); ok && !time.Now().Before(deadline) {
		return errorReasonTimeout
	}
	return reason
}

// requestErrorReason returns the label value for a transport error.
// Do not change the order of the checks. A DNS error or a TLS error can also be
// a timeout. Check the more specific errors first.
func requestErrorReason(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return errorReasonCanceled
	case isDNSError(err):
		return errorReasonDNS
	case isTLSError(err):
		return errorReasonTLS
	case isTimeoutError(err):
		return errorReasonTimeout
	case isConnectionError(err):
		return errorReasonConnection
	default:
		return errorReasonOther
	}
}

func isDNSError(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

func isTLSError(err error) bool {
	var (
		verificationErr *tls.CertificateVerificationError
		recordErr       tls.RecordHeaderError
		alertErr        tls.AlertError
		authorityErr    x509.UnknownAuthorityError
		hostnameErr     x509.HostnameError
		invalidErr      x509.CertificateInvalidError
		opErr           *net.OpError
	)
	return errors.As(err, &verificationErr) ||
		errors.As(err, &recordErr) ||
		errors.As(err, &alertErr) ||
		errors.As(err, &authorityErr) ||
		errors.As(err, &hostnameErr) ||
		errors.As(err, &invalidErr) ||
		(errors.As(err, &opErr) && opErr.Op == tlsRemoteErrorOp)
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isConnectionError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr)
}
