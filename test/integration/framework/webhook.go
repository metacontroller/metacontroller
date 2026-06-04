/*
Copyright 2019 Google Inc.

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

package framework

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
)

// ServeWebhook is a helper for quickly creating a webhook server in tests.
func (f *Fixture) ServeWebhook(handler func(request []byte) (response []byte, err error)) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		resp, err := handler(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
	f.deferTeardown(func() error {
		srv.Close()
		return nil
	})
	return srv
}

// ServeWebhookTLS starts a TLS webhook server using a self-signed certificate.
// It returns the server and the PEM-encoded CA certificate of the server's
// self-signed cert, so callers can supply it as a caBundle to the controller.
func (f *Fixture) ServeWebhookTLS(handler func(request []byte) (response []byte, err error)) (*httptest.Server, []byte) {
	return f.ServeWebhookTLSRaw(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		resp, err := handler(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	})
}

// ServeWebhookTLSRaw starts a TLS webhook server using a self-signed
// certificate, delegating all request handling to the provided HandlerFunc.
// It returns the server and the PEM-encoded CA certificate, so callers can
// supply it as a caBundle to the controller.
func (f *Fixture) ServeWebhookTLSRaw(handler http.HandlerFunc) (*httptest.Server, []byte) {
	srv := httptest.NewTLSServer(handler)
	f.deferTeardown(func() error {
		srv.Close()
		return nil
	})

	cert, err := x509.ParseCertificate(srv.TLS.Certificates[0].Certificate[0])
	if err != nil {
		f.t.Fatalf("ServeWebhookTLSRaw: failed to parse server certificate: %v", err)
	}
	var pemBuf bytes.Buffer
	if err := pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		f.t.Fatalf("ServeWebhookTLSRaw: failed to encode CA PEM: %v", err)
	}

	return srv, pemBuf.Bytes()
}

// ServeWebhookTLSWithBearerAuth starts a TLS webhook server that requires the
// Authorization header to equal "Bearer <wantToken>", responding 403 to any
// request that does not match.
func (f *Fixture) ServeWebhookTLSWithBearerAuth(wantToken string, handler func(body []byte) ([]byte, error)) (*httptest.Server, []byte) {
	wantHeader := "Bearer " + wantToken
	return f.ServeWebhookTLSRaw(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != wantHeader {
			f.t.Logf("webhook: unexpected Authorization %q, want %q", got, wantHeader)
			http.Error(w, "unauthorized", http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		resp, err := handler(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp) //nolint:errcheck
	})
}

// ServeWebhookTLSWithBasicAuth starts a TLS webhook server that requires HTTP
// Basic Authentication credentials to match wantUser and wantPass, responding
// 403 to any request that does not match.
func (f *Fixture) ServeWebhookTLSWithBasicAuth(wantUser, wantPass string, handler func(body []byte) ([]byte, error)) (*httptest.Server, []byte) {
	return f.ServeWebhookTLSRaw(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != wantUser || pass != wantPass {
			f.t.Logf("webhook: unexpected basic auth user=%q pass=%q ok=%v", user, pass, ok)
			http.Error(w, "unauthorized", http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		resp, err := handler(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp) //nolint:errcheck
	})
}
