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

package hooks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

const (
	connTestURL  = "https://example.com/sync"
	connTestHost = "example.com"
)

// --- matchConnection tests ---

func TestMatchConnection_whenNoConnections_returnsNil(t *testing.T) {
	result := matchConnection("https://example.com/path", nil)
	assert.Nil(t, result)
}

func TestMatchConnection_whenExactHostMatch_returnsConnection(t *testing.T) {
	conns := []v1alpha1.WebhookConnection{{Host: connTestHost}}
	result := matchConnection("https://example.com/path", conns)
	require.NotNil(t, result)
	assert.Equal(t, connTestHost, result.Host)
}

func TestMatchConnection_whenHostWithPortMatchesDefaultHTTPSPort_returnsConnection(t *testing.T) {
	// Connection specifies "example.com"; URL has explicit :443 — they match.
	conns := []v1alpha1.WebhookConnection{{Host: connTestHost}}
	result := matchConnection("https://example.com:443/path", conns)
	require.NotNil(t, result)
}

func TestMatchConnection_whenConnectionHasPortAndURLOmitsDefaultPort_returnsConnection(t *testing.T) {
	// Connection specifies "example.com:443"; URL omits port — they match.
	conns := []v1alpha1.WebhookConnection{{Host: "example.com:443"}}
	result := matchConnection("https://example.com/path", conns)
	require.NotNil(t, result)
}

func TestMatchConnection_whenCaseInsensitiveHost_returnsConnection(t *testing.T) {
	conns := []v1alpha1.WebhookConnection{{Host: "EXAMPLE.COM"}}
	result := matchConnection("https://example.com/path", conns)
	require.NotNil(t, result)
}

func TestMatchConnection_whenNoHostMatch_returnsNil(t *testing.T) {
	conns := []v1alpha1.WebhookConnection{{Host: "other.com"}}
	result := matchConnection("https://example.com/path", conns)
	assert.Nil(t, result)
}

func TestMatchConnection_whenFirstOfMultipleMatches_returnsFirst(t *testing.T) {
	conns := []v1alpha1.WebhookConnection{
		{Host: connTestHost, CABundle: &v1alpha1.CABundle{Inline: ptr.To("first")}},
		{Host: connTestHost, CABundle: &v1alpha1.CABundle{Inline: ptr.To("second")}},
	}
	result := matchConnection("https://example.com/path", conns)
	require.NotNil(t, result)
	require.NotNil(t, result.CABundle)
	assert.Equal(t, "first", *result.CABundle.Inline)
}

// --- ResolveConnectionConfig tests ---

func TestResolveConnectionConfig_whenNilWebhook_returnsNil(t *testing.T) {
	result, err := ResolveConnectionConfig(context.Background(), newFakeK8sClient(), nil, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveConnectionConfig_whenNoFieldsAndNoConnections_returnsNil(t *testing.T) {
	url := connTestURL
	webhook := &v1alpha1.Webhook{URL: &url}
	result, err := ResolveConnectionConfig(context.Background(), newFakeK8sClient(), webhook, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveConnectionConfig_whenPerHookCABundle_usesHookSettings(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	url := connTestURL
	webhook := &v1alpha1.Webhook{
		URL:      &url,
		CABundle: &v1alpha1.CABundle{Inline: ptr.To(string(caPEM))},
	}
	// Connections are present but should be ignored because the webhook sets caBundle.
	conns := []v1alpha1.WebhookConnection{{Host: connTestHost}}
	result, err := ResolveConnectionConfig(context.Background(), newFakeK8sClient(), webhook, conns)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caPEM, result.CABundle)
}

func TestResolveConnectionConfig_whenMatchingConnection_resolvesCABundle(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	url := connTestURL
	webhook := &v1alpha1.Webhook{URL: &url}
	conns := []v1alpha1.WebhookConnection{{
		Host:     "example.com",
		CABundle: &v1alpha1.CABundle{Inline: ptr.To(string(caPEM))},
	}}
	result, err := ResolveConnectionConfig(context.Background(), newFakeK8sClient(), webhook, conns)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caPEM, result.CABundle)
}

func TestResolveConnectionConfig_whenAuthAndBasicAuthBothSet_returnsError(t *testing.T) {
	url := connTestURL
	webhook := &v1alpha1.Webhook{
		URL: &url,
		Authorization: &v1alpha1.Authorization{
			SecretRef: v1alpha1.SecretKeyRef{Name: "s", Namespace: authNamespace, Key: authTokenKey},
		},
		BasicAuth: &v1alpha1.BasicAuth{
			SecretRef: v1alpha1.SecretRef{Name: "s", Namespace: authNamespace},
		},
	}
	_, err := ResolveConnectionConfig(context.Background(), newFakeK8sClient(), webhook, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestResolveConnectionConfig_whenAuthorizationOnConnection_resolvesAuthHeader(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{authTokenKey: []byte("abc123")},
	}
	url := connTestURL
	webhook := &v1alpha1.Webhook{URL: &url}
	conns := []v1alpha1.WebhookConnection{{
		Host: connTestHost,
		Authorization: &v1alpha1.Authorization{
			SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
		},
	}}
	result, err := ResolveConnectionConfig(context.Background(), newFakeK8sClient(secret), webhook, conns)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Bearer abc123", result.AuthHeader)
}

func TestResolveConnectionConfig_whenServiceFormWebhook_matchesConnectionByConstructedHost(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	name := "my-hook"
	ns := "my-ns"
	path := "/sync"
	// Webhook expressed via service+path — no explicit URL.
	webhook := &v1alpha1.Webhook{
		Service: &v1alpha1.ServiceReference{Name: name, Namespace: ns},
		Path:    &path,
	}
	// webhookURL constructs "http://my-hook.my-ns:80/sync"; host is "my-hook.my-ns:80".
	// The connection entry uses "my-hook.my-ns" which should match via default-port
	// normalisation (port 80 on http is treated as equivalent to no port).
	conns := []v1alpha1.WebhookConnection{{
		Host:     "my-hook.my-ns",
		CABundle: &v1alpha1.CABundle{Inline: ptr.To(string(caPEM))},
	}}
	result, err := ResolveConnectionConfig(context.Background(), newFakeK8sClient(), webhook, conns)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caPEM, result.CABundle)
}
