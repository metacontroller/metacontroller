// Copyright 2026 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "metacontroller/pkg/apis/metacontroller/v1alpha1"
)

// ResolvedConnection holds the resolved credentials and TLS material for a
// webhook connection. All fields are optional; a nil ResolvedConnection is
// valid and means no custom TLS or authentication is configured.
type ResolvedConnection struct {
	// CABundle is the PEM-encoded CA certificate(s) used to verify the server.
	CABundle []byte
	// AuthHeader is the value for the Authorization request header, e.g.
	// "Bearer <token>" or "Basic <base64>". Empty means no header is sent.
	AuthHeader string
	// ClientCert is the client certificate used for mutual TLS. Nil means no
	// client certificate is presented.
	ClientCert *tls.Certificate
}

// ResolveConnectionConfig resolves the connection settings for the given
// webhook. Settings are determined as follows:
//
//  1. If the webhook defines any of caBundle, clientTLS, authorization, or
//     basicAuth directly, those per-hook settings are used in full — no
//     merging with connection entries occurs.
//  2. Otherwise, the webhook URL's host is matched against the connections
//     slice and the first matching entry's settings are resolved.
//
// It is an error to set both authorization and basicAuth on the same webhook
// or on the same connection entry.
func ResolveConnectionConfig(
	ctx context.Context,
	k8sClient client.Client,
	webhook *v1alpha1.Webhook,
	connections []v1alpha1.WebhookConnection,
) (*ResolvedConnection, error) {
	if webhook == nil {
		return nil, nil
	}

	// Per-hook override: if any auth/TLS field is set directly on the webhook,
	// use those exclusively without consulting connections.
	if webhook.CABundle != nil || webhook.ClientTLS != nil ||
		webhook.Authorization != nil || webhook.BasicAuth != nil {
		return resolveFields(ctx, k8sClient,
			webhook.CABundle, webhook.ClientTLS, webhook.Authorization, webhook.BasicAuth)
	}

	// No per-hook override — look up a matching connection entry.
	if len(connections) == 0 {
		return nil, nil
	}

	// Derive the effective URL to extract the host for matching. If the webhook
	// spec is malformed (neither url nor service+path), no connection applies;
	// the error will surface again when the executor attempts to call the hook.
	effectiveURL, urlErr := webhookURL(webhook)
	if urlErr != nil {
		return nil, nil //nolint:nilerr
	}

	conn := matchConnection(effectiveURL, connections)
	if conn == nil {
		return nil, nil
	}

	return resolveFields(ctx, k8sClient,
		conn.CABundle, conn.ClientTLS, conn.Authorization, conn.BasicAuth)
}

// resolveFields resolves all connection material from the four optional fields
// shared by both Webhook and WebhookConnection.
func resolveFields(
	ctx context.Context,
	k8sClient client.Client,
	caBundle *v1alpha1.CABundle,
	clientTLS *v1alpha1.ClientTLS,
	authorization *v1alpha1.Authorization,
	basicAuth *v1alpha1.BasicAuth,
) (*ResolvedConnection, error) {
	if authorization != nil && basicAuth != nil {
		return nil, fmt.Errorf("authorization and basicAuth are mutually exclusive")
	}

	resolved := &ResolvedConnection{}
	var err error

	resolved.CABundle, err = ResolveCABundle(ctx, k8sClient, caBundle)
	if err != nil {
		return nil, fmt.Errorf("can't resolve caBundle: %w", err)
	}

	resolved.ClientCert, err = ResolveClientTLS(ctx, k8sClient, clientTLS)
	if err != nil {
		return nil, fmt.Errorf("can't resolve clientTLS: %w", err)
	}

	if authorization != nil {
		resolved.AuthHeader, err = ResolveAuthorization(ctx, k8sClient, authorization)
		if err != nil {
			return nil, fmt.Errorf("can't resolve authorization: %w", err)
		}
	} else if basicAuth != nil {
		resolved.AuthHeader, err = ResolveBasicAuth(ctx, k8sClient, basicAuth)
		if err != nil {
			return nil, fmt.Errorf("can't resolve basicAuth: %w", err)
		}
	}

	// Return nil when nothing was configured to keep the zero-value semantics
	// consistent with a nil ResolvedConnection.
	if len(resolved.CABundle) == 0 && resolved.AuthHeader == "" && resolved.ClientCert == nil {
		return nil, nil
	}

	return resolved, nil
}

// matchConnection returns the first WebhookConnection whose host matches the
// host (and port) extracted from webhookURL. Matching is case-insensitive.
// For HTTPS URLs, both "example.com" and "example.com:443" match a URL whose
// effective port is 443.
func matchConnection(webhookURL string, connections []v1alpha1.WebhookConnection) *v1alpha1.WebhookConnection {
	u, err := url.Parse(webhookURL)
	if err != nil {
		return nil
	}

	// u.Host is already "host" or "host:port".
	urlHost := u.Host

	// Derive the canonical host:port for default-port comparison.
	canonicalHost := urlHost
	if u.Port() == "" {
		// No explicit port — infer from scheme.
		switch u.Scheme {
		case "https":
			canonicalHost = u.Hostname() + ":443"
		case schemeHTTP:
			canonicalHost = u.Hostname() + ":80"
		}
	}

	for i := range connections {
		c := &connections[i]
		// Match against both the raw host and the canonical host:port form.
		if stringsEqualFold(c.Host, urlHost) || stringsEqualFold(c.Host, canonicalHost) {
			return c
		}
		// Also allow the connection to be specified without port when the URL
		// has an explicit default port, e.g. connection host "example.com"
		// matching URL "https://example.com:443/path".
		if u.Port() != "" && stringsEqualFold(c.Host, u.Hostname()) {
			switch u.Scheme {
			case "https":
				if u.Port() == "443" {
					return c
				}
			case schemeHTTP:
				if u.Port() == "80" {
					return c
				}
			}
		}
	}

	return nil
}

// stringsEqualFold reports whether a and b are equal under Unicode case-folding.
func stringsEqualFold(a, b string) bool {
	return len(a) == len(b) && foldEqual(a, b)
}

// foldEqual is a simple ASCII case-insensitive comparison sufficient for
// hostnames (which are always ASCII).
func foldEqual(a, b string) bool {
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca == cb {
			continue
		}
		// Convert uppercase to lowercase for comparison.
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
