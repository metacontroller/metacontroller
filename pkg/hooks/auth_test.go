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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

const (
	authSecretName    = "auth-secret" //nolint:gosec
	authTokenKey      = "token"
	authTokenValue    = "my-token"
	authNamespace     = "default"
	tlsSecretName     = "tls-secret" //nolint:gosec
	pemTypeCert       = "CERTIFICATE"
	missingSecretName = "missing" //nolint:gosec
	unknownDataKey    = "other-key"
)

// generateClientCertPEMs returns a self-signed PEM certificate and its private
// key suitable for use as a client TLS certificate in tests.
func generateClientCertPEMs(t *testing.T) (certPEM []byte, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	var certBuf bytes.Buffer
	require.NoError(t, pem.Encode(&certBuf, &pem.Block{Type: pemTypeCert, Bytes: certDER}))

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	var keyBuf bytes.Buffer
	require.NoError(t, pem.Encode(&keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))

	return certBuf.Bytes(), keyBuf.Bytes()
}

func TestResolveAuthorization_whenNilSpec_returnsEmpty(t *testing.T) {
	result, err := ResolveAuthorization(context.Background(), newFakeK8sClient(), nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestResolveAuthorization_whenTypeBasic_returnsError(t *testing.T) {
	spec := &v1alpha1.Authorization{
		Type:      "Basic",
		SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	_, err := ResolveAuthorization(context.Background(), newFakeK8sClient(), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use the basicAuth field instead")
}

func TestResolveAuthorization_whenTypeBasicLowercase_returnsError(t *testing.T) {
	spec := &v1alpha1.Authorization{
		Type:      "basic",
		SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	_, err := ResolveAuthorization(context.Background(), newFakeK8sClient(), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use the basicAuth field instead")
}

func TestResolveAuthorization_whenSecretMissing_returnsError(t *testing.T) {
	spec := &v1alpha1.Authorization{
		SecretRef: v1alpha1.SecretKeyRef{Name: missingSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	_, err := ResolveAuthorization(context.Background(), newFakeK8sClient(), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't get authorization secret")
}

func TestResolveAuthorization_whenKeyMissing_returnsError(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{unknownDataKey: []byte("value")},
	}
	spec := &v1alpha1.Authorization{
		SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	_, err := ResolveAuthorization(context.Background(), newFakeK8sClient(secret), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), authTokenKey)
}

func TestResolveAuthorization_whenDefaultType_returnsBearerHeader(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{authTokenKey: []byte(authTokenValue)},
	}
	spec := &v1alpha1.Authorization{
		SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	result, err := ResolveAuthorization(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	assert.Equal(t, "Bearer "+authTokenValue, result)
}

func TestResolveAuthorization_whenExplicitType_returnsHeaderWithThatType(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{authTokenKey: []byte(authTokenValue)},
	}
	spec := &v1alpha1.Authorization{
		Type:      "Token",
		SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	result, err := ResolveAuthorization(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	assert.Equal(t, "Token "+authTokenValue, result)
}

func TestResolveAuthorization_whenEmptyToken_returnsBearerWithEmptyValue(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{authTokenKey: []byte("")},
	}
	spec := &v1alpha1.Authorization{
		SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	result, err := ResolveAuthorization(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	assert.Equal(t, "Bearer ", result)
}

func TestResolveAuthorization_whenWhitespaceType_returnsBearerHeader(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{authTokenKey: []byte(authTokenValue)},
	}
	spec := &v1alpha1.Authorization{
		Type:      "   ",
		SecretRef: v1alpha1.SecretKeyRef{Name: authSecretName, Namespace: authNamespace, Key: authTokenKey},
	}
	result, err := ResolveAuthorization(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	assert.Equal(t, "Bearer "+authTokenValue, result)
}

func TestResolveBasicAuth_whenNilSpec_returnsEmpty(t *testing.T) {
	result, err := ResolveBasicAuth(context.Background(), newFakeK8sClient(), nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestResolveBasicAuth_whenSecretMissing_returnsError(t *testing.T) {
	spec := &v1alpha1.BasicAuth{
		SecretRef: v1alpha1.SecretRef{Name: missingSecretName, Namespace: authNamespace},
	}
	_, err := ResolveBasicAuth(context.Background(), newFakeK8sClient(), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't get basicAuth secret")
}

func TestResolveBasicAuth_whenUsernameMissing_returnsError(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{"password": []byte("pass")},
	}
	spec := &v1alpha1.BasicAuth{
		SecretRef: v1alpha1.SecretRef{Name: authSecretName, Namespace: authNamespace},
	}
	_, err := ResolveBasicAuth(context.Background(), newFakeK8sClient(secret), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestResolveBasicAuth_whenPasswordMissing_returnsError(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{"username": []byte("user")},
	}
	spec := &v1alpha1.BasicAuth{
		SecretRef: v1alpha1.SecretRef{Name: authSecretName, Namespace: authNamespace},
	}
	_, err := ResolveBasicAuth(context.Background(), newFakeK8sClient(secret), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestResolveBasicAuth_whenDefaultKeys_returnsBasicHeader(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data: map[string][]byte{
			"username": []byte("alice"),
			"password": []byte("s3cr3t"),
		},
	}
	spec := &v1alpha1.BasicAuth{
		SecretRef: v1alpha1.SecretRef{Name: authSecretName, Namespace: authNamespace},
	}
	result, err := ResolveBasicAuth(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:s3cr3t"))
	assert.Equal(t, expected, result)
}

func TestResolveBasicAuth_whenCustomKeys_returnsBasicHeader(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data: map[string][]byte{
			"user": []byte("bob"),
			"pass": []byte("hunter2"),
		},
	}
	spec := &v1alpha1.BasicAuth{
		SecretRef:   v1alpha1.SecretRef{Name: authSecretName, Namespace: authNamespace},
		UsernameKey: "user",
		PasswordKey: "pass",
	}
	result, err := ResolveBasicAuth(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("bob:hunter2"))
	assert.Equal(t, expected, result)
}

func TestResolveBasicAuth_whenUsernameContainsColon_returnsError(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: authNamespace},
		Data: map[string][]byte{
			"username": []byte("user:name"),
			"password": []byte("pass"),
		},
	}
	spec := &v1alpha1.BasicAuth{
		SecretRef: v1alpha1.SecretRef{Name: authSecretName, Namespace: authNamespace},
	}
	_, err := ResolveBasicAuth(context.Background(), newFakeK8sClient(secret), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain ':'")
}

func TestResolveClientTLS_whenNilSpec_returnsNil(t *testing.T) {
	result, err := ResolveClientTLS(context.Background(), newFakeK8sClient(), nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveClientTLS_whenSecretMissing_returnsError(t *testing.T) {
	spec := &v1alpha1.ClientTLS{
		SecretRef: v1alpha1.SecretRef{Name: missingSecretName, Namespace: authNamespace},
	}
	_, err := ResolveClientTLS(context.Background(), newFakeK8sClient(), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't get clientTLS secret")
}

func TestResolveClientTLS_whenCertKeyMissing_returnsError(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{defaultPrivateKeyKey: []byte("key-data")},
	}
	spec := &v1alpha1.ClientTLS{
		SecretRef: v1alpha1.SecretRef{Name: tlsSecretName, Namespace: authNamespace},
	}
	_, err := ResolveClientTLS(context.Background(), newFakeK8sClient(secret), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), defaultCertKey)
}

func TestResolveClientTLS_whenPrivateKeyMissing_returnsError(t *testing.T) {
	certPEM, _ := generateClientCertPEMs(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: authNamespace},
		Data:       map[string][]byte{defaultCertKey: certPEM},
	}
	spec := &v1alpha1.ClientTLS{
		SecretRef: v1alpha1.SecretRef{Name: tlsSecretName, Namespace: authNamespace},
	}
	_, err := ResolveClientTLS(context.Background(), newFakeK8sClient(secret), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), defaultPrivateKeyKey)
}

func TestResolveClientTLS_whenValidCertAndKey_returnsCertificate(t *testing.T) {
	certPEM, keyPEM := generateClientCertPEMs(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: authNamespace},
		Data: map[string][]byte{
			defaultCertKey:       certPEM,
			defaultPrivateKeyKey: keyPEM,
		},
	}
	spec := &v1alpha1.ClientTLS{
		SecretRef: v1alpha1.SecretRef{Name: tlsSecretName, Namespace: authNamespace},
	}
	result, err := ResolveClientTLS(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestResolveClientTLS_whenCustomKeys_returnsCertificate(t *testing.T) {
	certPEM, keyPEM := generateClientCertPEMs(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: authNamespace},
		Data: map[string][]byte{
			"client.crt": certPEM,
			"client.key": keyPEM,
		},
	}
	spec := &v1alpha1.ClientTLS{
		SecretRef:     v1alpha1.SecretRef{Name: tlsSecretName, Namespace: authNamespace},
		CertKey:       "client.crt",
		PrivateKeyKey: "client.key",
	}
	result, err := ResolveClientTLS(context.Background(), newFakeK8sClient(secret), spec)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestResolveClientTLS_whenInvalidPEM_returnsError(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: authNamespace},
		Data: map[string][]byte{
			defaultCertKey:       []byte("not-a-cert"),
			defaultPrivateKeyKey: []byte("not-a-key"),
		},
	}
	spec := &v1alpha1.ClientTLS{
		SecretRef: v1alpha1.SecretRef{Name: tlsSecretName, Namespace: authNamespace},
	}
	_, err := ResolveClientTLS(context.Background(), newFakeK8sClient(secret), spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't parse clientTLS certificate")
}
