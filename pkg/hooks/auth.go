// Copyright 2026 Metacontroller authors.
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
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"

	v1alpha1 "metacontroller/pkg/apis/metacontroller/v1alpha1"
)

const (
	defaultUsernameKey   = "username"
	defaultPasswordKey   = "password"
	defaultCertKey       = "tls.crt"
	defaultPrivateKeyKey = "tls.key"
)

// ResolveAuthorization resolves the Authorization header value from the given
// Authorization spec. Returns an empty string when spec is nil.
// Returns an error if the type is "Basic" (case-insensitive); use basicAuth instead.
func ResolveAuthorization(ctx context.Context, k8sClient client.Client, spec *v1alpha1.Authorization) (string, error) {
	if spec == nil {
		return "", nil
	}

	if strings.EqualFold(strings.TrimSpace(spec.Type), "basic") {
		return "", fmt.Errorf("authorization type %q is not supported; use the basicAuth field instead", spec.Type)
	}

	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      spec.SecretRef.Name,
		Namespace: spec.SecretRef.Namespace,
	}, secret); err != nil {
		return "", fmt.Errorf("can't get authorization secret %s/%s: %w",
			spec.SecretRef.Namespace, spec.SecretRef.Name, err)
	}

	value, ok := secret.Data[spec.SecretRef.Key]
	if !ok {
		return "", fmt.Errorf("authorization secret %s/%s does not contain key %q",
			spec.SecretRef.Namespace, spec.SecretRef.Name, spec.SecretRef.Key)
	}

	authType := strings.TrimSpace(spec.Type)
	if authType == "" {
		authType = "Bearer"
	}

	return authType + " " + strings.TrimSpace(string(value)), nil
}

// ResolveBasicAuth resolves an HTTP Basic Authentication header value from the
// given BasicAuth spec. Returns an empty string when spec is nil.
func ResolveBasicAuth(ctx context.Context, k8sClient client.Client, spec *v1alpha1.BasicAuth) (string, error) {
	if spec == nil {
		return "", nil
	}

	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      spec.SecretRef.Name,
		Namespace: spec.SecretRef.Namespace,
	}, secret); err != nil {
		return "", fmt.Errorf("can't get basicAuth secret %s/%s: %w",
			spec.SecretRef.Namespace, spec.SecretRef.Name, err)
	}

	usernameKey := spec.UsernameKey
	if usernameKey == "" {
		usernameKey = defaultUsernameKey
	}

	passwordKey := spec.PasswordKey
	if passwordKey == "" {
		passwordKey = defaultPasswordKey
	}

	username, ok := secret.Data[usernameKey]
	if !ok {
		return "", fmt.Errorf("basicAuth secret %s/%s does not contain key %q",
			spec.SecretRef.Namespace, spec.SecretRef.Name, usernameKey)
	}

	password, ok := secret.Data[passwordKey]
	if !ok {
		return "", fmt.Errorf("basicAuth secret %s/%s does not contain key %q",
			spec.SecretRef.Namespace, spec.SecretRef.Name, passwordKey)
	}

	if strings.Contains(string(username), ":") {
		return "", fmt.Errorf("basicAuth username must not contain ':'")
	}

	encoded := base64.StdEncoding.EncodeToString(
		[]byte(strings.TrimSpace(string(username)) + ":" + strings.TrimSpace(string(password))),
	)
	return "Basic " + encoded, nil
}

// ResolveClientTLS resolves a client TLS certificate from the given ClientTLS
// spec. Returns nil when spec is nil.
func ResolveClientTLS(ctx context.Context, k8sClient client.Client, spec *v1alpha1.ClientTLS) (*tls.Certificate, error) {
	if spec == nil {
		return nil, nil
	}

	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      spec.SecretRef.Name,
		Namespace: spec.SecretRef.Namespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("can't get clientTLS secret %s/%s: %w",
			spec.SecretRef.Namespace, spec.SecretRef.Name, err)
	}

	certKey := spec.CertKey
	if certKey == "" {
		certKey = defaultCertKey
	}

	privateKeyKey := spec.PrivateKeyKey
	if privateKeyKey == "" {
		privateKeyKey = defaultPrivateKeyKey
	}

	certPEM, ok := secret.Data[certKey]
	if !ok {
		return nil, fmt.Errorf("clientTLS secret %s/%s does not contain key %q",
			spec.SecretRef.Namespace, spec.SecretRef.Name, certKey)
	}

	keyPEM, ok := secret.Data[privateKeyKey]
	if !ok {
		return nil, fmt.Errorf("clientTLS secret %s/%s does not contain key %q",
			spec.SecretRef.Namespace, spec.SecretRef.Name, privateKeyKey)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("can't parse clientTLS certificate from secret %s/%s: %w",
			spec.SecretRef.Namespace, spec.SecretRef.Name, err)
	}

	return &cert, nil
}
