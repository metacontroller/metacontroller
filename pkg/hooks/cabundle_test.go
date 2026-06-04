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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

func generateSelfSignedCACert(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "generating test CA key")

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err, "creating test CA certificate")

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: pemTypeCert, Bytes: certDER}))
	return buf.Bytes()
}

const (
	testNamespace    = "default"
	testSecretName   = "my-tls-secret" //nolint:gosec
	testCABundleName = "my-ca-bundle"
)

func newFakeK8sClient(objs ...runtime.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	b := fake.NewClientBuilder().WithScheme(scheme)
	for _, obj := range objs {
		b = b.WithRuntimeObjects(obj)
	}
	return b.Build()
}

func TestResolveCABundle_whenNilSpec_returnsNil(t *testing.T) {
	result, err := ResolveCABundle(context.Background(), newFakeK8sClient(), nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveCABundle_whenInline_returnsPEMBytes(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	spec := &v1alpha1.CABundle{Inline: ptr.To(string(caPEM))}

	result, err := ResolveCABundle(context.Background(), newFakeK8sClient(), spec)

	require.NoError(t, err)
	assert.Equal(t, caPEM, result)
}

func TestResolveCABundle_whenNoSourceSet_returnsError(t *testing.T) {
	_, err := ResolveCABundle(context.Background(), newFakeK8sClient(), &v1alpha1.CABundle{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of inline, secretRef, or configMapRef")
}

func TestResolveCABundle_whenMultipleSourcesSet_returnsError(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	spec := &v1alpha1.CABundle{
		Inline:    ptr.To(string(caPEM)),
		SecretRef: &v1alpha1.ResourceKeyRef{Name: "my-secret", Namespace: testNamespace},
	}

	_, err := ResolveCABundle(context.Background(), newFakeK8sClient(), spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one source set")
}

func TestResolveCABundle_whenSecretRefWithDefaultKey_returnsCAbytes(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: testNamespace},
		Data:       map[string][]byte{"ca.crt": caPEM},
	}
	spec := &v1alpha1.CABundle{
		SecretRef: &v1alpha1.ResourceKeyRef{Name: testSecretName, Namespace: testNamespace},
	}

	result, err := ResolveCABundle(context.Background(), newFakeK8sClient(secret), spec)

	require.NoError(t, err)
	assert.Equal(t, caPEM, result)
}

func TestResolveCABundle_whenSecretRefWithExplicitKey_returnsCAbytes(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: testNamespace},
		Data:       map[string][]byte{"my-ca.crt": caPEM},
	}
	spec := &v1alpha1.CABundle{
		SecretRef: &v1alpha1.ResourceKeyRef{Name: testSecretName, Namespace: testNamespace, Key: "my-ca.crt"},
	}

	result, err := ResolveCABundle(context.Background(), newFakeK8sClient(secret), spec)

	require.NoError(t, err)
	assert.Equal(t, caPEM, result)
}

func TestResolveCABundle_whenSecretRefSecretNotFound_returnsError(t *testing.T) {
	spec := &v1alpha1.CABundle{
		SecretRef: &v1alpha1.ResourceKeyRef{Name: "missing-secret", Namespace: testNamespace},
	}

	_, err := ResolveCABundle(context.Background(), newFakeK8sClient(), spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Secret")
}

func TestResolveCABundle_whenSecretRefKeyNotFound_returnsError(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: testNamespace},
		Data:       map[string][]byte{"other-key": caPEM},
	}
	spec := &v1alpha1.CABundle{
		SecretRef: &v1alpha1.ResourceKeyRef{Name: testSecretName, Namespace: testNamespace},
	}

	_, err := ResolveCABundle(context.Background(), newFakeK8sClient(secret), spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `key "ca.crt" not found`)
}

func TestResolveCABundle_whenConfigMapRefWithDefaultKey_returnsCAbytes(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: testCABundleName, Namespace: "metacontroller"},
		Data:       map[string]string{"ca.crt": string(caPEM)},
	}
	spec := &v1alpha1.CABundle{
		ConfigMapRef: &v1alpha1.ResourceKeyRef{Name: testCABundleName, Namespace: "metacontroller"},
	}

	result, err := ResolveCABundle(context.Background(), newFakeK8sClient(cm), spec)

	require.NoError(t, err)
	assert.Equal(t, caPEM, result)
}

func TestResolveCABundle_whenConfigMapRefWithExplicitKey_returnsCAbytes(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: testCABundleName, Namespace: testNamespace},
		Data:       map[string]string{"custom-ca.crt": string(caPEM)},
	}
	spec := &v1alpha1.CABundle{
		ConfigMapRef: &v1alpha1.ResourceKeyRef{Name: testCABundleName, Namespace: testNamespace, Key: "custom-ca.crt"},
	}

	result, err := ResolveCABundle(context.Background(), newFakeK8sClient(cm), spec)

	require.NoError(t, err)
	assert.Equal(t, caPEM, result)
}

func TestResolveCABundle_whenConfigMapRefNotFound_returnsError(t *testing.T) {
	spec := &v1alpha1.CABundle{
		ConfigMapRef: &v1alpha1.ResourceKeyRef{Name: "missing-cm", Namespace: testNamespace},
	}

	_, err := ResolveCABundle(context.Background(), newFakeK8sClient(), spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ConfigMap")
}

func TestResolveCABundle_whenConfigMapRefKeyNotFound_returnsError(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: testCABundleName, Namespace: testNamespace},
		Data:       map[string]string{"other-key": string(caPEM)},
	}
	spec := &v1alpha1.CABundle{
		ConfigMapRef: &v1alpha1.ResourceKeyRef{Name: testCABundleName, Namespace: testNamespace},
	}

	_, err := ResolveCABundle(context.Background(), newFakeK8sClient(cm), spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `key "ca.crt" not found`)
}

func TestBuildTLSTransport_whenValidPEM_returnsTransportWithCustomRootCAs(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)

	transport, err := buildTLSTransport(caPEM, nil)

	require.NoError(t, err)
	require.NotNil(t, transport)
	require.NotNil(t, transport.TLSClientConfig)
	assert.NotNil(t, transport.TLSClientConfig.RootCAs)
}

func TestBuildTLSTransport_whenInvalidPEM_returnsError(t *testing.T) {
	_, err := buildTLSTransport([]byte("not-a-valid-pem-block"), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid PEM-encoded certificates")
}

func TestBuildTLSTransport_whenClientCert_populatesCertificates(t *testing.T) {
	certPEM, keyPEM := generateClientCertPEMs(t)
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	transport, err := buildTLSTransport(nil, &tlsCert)

	require.NoError(t, err)
	require.NotNil(t, transport)
	require.NotNil(t, transport.TLSClientConfig)
	assert.Len(t, transport.TLSClientConfig.Certificates, 1)
	assert.Nil(t, transport.TLSClientConfig.RootCAs)
}

func TestBuildTLSTransport_whenCABundleAndClientCert_setsRootCAsAndCertificates(t *testing.T) {
	caPEM := generateSelfSignedCACert(t)
	certPEM, keyPEM := generateClientCertPEMs(t)
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	transport, err := buildTLSTransport(caPEM, &tlsCert)

	require.NoError(t, err)
	require.NotNil(t, transport)
	require.NotNil(t, transport.TLSClientConfig)
	assert.NotNil(t, transport.TLSClientConfig.RootCAs)
	assert.Len(t, transport.TLSClientConfig.Certificates, 1)
}
