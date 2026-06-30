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

// --- matchEndpointConfig tests ---

func TestMatchEndpointConfig_whenNoEndpointConfigs_returnsNil(t *testing.T) {
	result := matchEndpointConfig("https://example.com/path", nil)
	assert.Nil(t, result)
}

func TestMatchEndpointConfig_whenExactHostMatch_returnsEndpointConfig(t *testing.T) {
	cfgs := []v1alpha1.EndpointConfig{{Host: connTestHost}}
	result := matchEndpointConfig("https://example.com/path", cfgs)
	require.NotNil(t, result)
	assert.Equal(t, connTestHost, result.Host)
}

func TestMatchEndpointConfig_whenHostWithPortMatchesDefaultHTTPSPort_returnsEndpointConfig(t *testing.T) {
	// EndpointConfig specifies "example.com"; URL has explicit :443 — they match.
	cfgs := []v1alpha1.EndpointConfig{{Host: connTestHost}}
	result := matchEndpointConfig("https://example.com:443/path", cfgs)
	require.NotNil(t, result)
}

func TestMatchEndpointConfig_whenEndpointConfigHasPortAndURLOmitsDefaultPort_returnsEndpointConfig(t *testing.T) {
	// EndpointConfig specifies "example.com:443"; URL omits port — they match.
	cfgs := []v1alpha1.EndpointConfig{{Host: "example.com:443"}}
	result := matchEndpointConfig("https://example.com/path", cfgs)
	require.NotNil(t, result)
}

func TestMatchEndpointConfig_whenCaseInsensitiveHost_returnsEndpointConfig(t *testing.T) {
	cfgs := []v1alpha1.EndpointConfig{{Host: "EXAMPLE.COM"}}
	result := matchEndpointConfig("https://example.com/path", cfgs)
	require.NotNil(t, result)
}

func TestMatchEndpointConfig_whenNoHostMatch_returnsNil(t *testing.T) {
	cfgs := []v1alpha1.EndpointConfig{{Host: "other.com"}}
	result := matchEndpointConfig("https://example.com/path", cfgs)
	assert.Nil(t, result)
}

func TestMatchEndpointConfig_whenFirstOfMultipleMatches_returnsFirst(t *testing.T) {
	cfgs := []v1alpha1.EndpointConfig{
		{Host: connTestHost, CABundle: &v1alpha1.CABundle{Inline: ptr.To("first")}},
		{Host: connTestHost, CABundle: &v1alpha1.CABundle{Inline: ptr.To("second")}},
	}
	result := matchEndpointConfig("https://example.com/path", cfgs)
	require.NotNil(t, result)
	require.NotNil(t, result.CABundle)
	assert.Equal(t, "first", *result.CABundle.Inline)
}

// --- ResolveEndpointConfig tests ---

func TestResolveEndpointConfig_whenNilWebhook_returnsNil(t *testing.T) {
	result, err := ResolveEndpointConfig(context.Background(), newFakeK8sClient(), nil, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveEndpointConfig_whenNoFieldsAndNoEndpointConfigs_returnsNil(t *testing.T) {
	url := connTestURL
	webhook := &v1alpha1.Webhook{URL: &url}
	result, err := ResolveEndpointConfig(context.Background(), newFakeK8sClient(), webhook, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveEndpointConfig_whenPerHookCABundle_usesHookSettings(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	url := connTestURL
	webhook := &v1alpha1.Webhook{
		URL:      &url,
		CABundle: &v1alpha1.CABundle{Inline: ptr.To(string(caPEM))},
	}
	// EndpointConfigs are present but should be ignored because the webhook sets caBundle.
	cfgs := []v1alpha1.EndpointConfig{{Host: connTestHost}}
	result, err := ResolveEndpointConfig(context.Background(), newFakeK8sClient(), webhook, cfgs)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caPEM, result.CABundle)
}

func TestResolveEndpointConfig_whenMatchingEndpointConfig_resolvesCABundle(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	url := connTestURL
	webhook := &v1alpha1.Webhook{URL: &url}
	cfgs := []v1alpha1.EndpointConfig{{
		Host:     "example.com",
		CABundle: &v1alpha1.CABundle{Inline: ptr.To(string(caPEM))},
	}}
	result, err := ResolveEndpointConfig(context.Background(), newFakeK8sClient(), webhook, cfgs)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caPEM, result.CABundle)
}

func TestResolveEndpointConfig_whenAuthAndBasicAuthBothSet_returnsError(t *testing.T) {
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
	_, err := ResolveEndpointConfig(context.Background(), newFakeK8sClient(), webhook, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestResolveEndpointConfig_whenAuthorizationOnEndpointConfig_resolvesAuthHeader(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{authTokenKey: []byte("abc123")},
	}
	url := connTestURL
	webhook := &v1alpha1.Webhook{URL: &url}
	cfgs := []v1alpha1.EndpointConfig{{
		Host: connTestHost,
		Authorization: &v1alpha1.Authorization{
			SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
		},
	}}
	result, err := ResolveEndpointConfig(context.Background(), newFakeK8sClient(secret), webhook, cfgs)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Bearer abc123", result.AuthHeader)
}

func TestResolveEndpointConfig_whenServiceFormWebhook_matchesEndpointConfigByConstructedHost(t *testing.T) {
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
	// The endpoint config entry uses "my-hook.my-ns" which should match via default-port
	// normalisation (port 80 on http is treated as equivalent to no port).
	cfgs := []v1alpha1.EndpointConfig{{
		Host:     "my-hook.my-ns",
		CABundle: &v1alpha1.CABundle{Inline: ptr.To(string(caPEM))},
	}}
	result, err := ResolveEndpointConfig(context.Background(), newFakeK8sClient(), webhook, cfgs)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caPEM, result.CABundle)
}
