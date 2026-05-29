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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

const defaultCABundleKey = "ca.crt"

// ResolveCABundle resolves the PEM-encoded CA certificate bytes from the given
// CABundle spec. Returns nil bytes (and no error) if spec is nil, which means
// the caller should use the system trust roots.
//
// The spec supports three mutually exclusive sources:
//   - inline: PEM bytes embedded directly in the spec
//   - secretRef: a key within a Kubernetes Secret
//   - configMapRef: a key within a Kubernetes ConfigMap
//
// Exactly one source must be set when spec is non-nil.
func ResolveCABundle(ctx context.Context, k8sClient client.Client, spec *v1alpha1.CABundle) ([]byte, error) {
	if spec == nil {
		return nil, nil
	}

	sourcesSet := 0
	if spec.Inline != nil {
		sourcesSet++
	}
	if spec.SecretRef != nil {
		sourcesSet++
	}
	if spec.ConfigMapRef != nil {
		sourcesSet++
	}
	if sourcesSet == 0 {
		return nil, fmt.Errorf("caBundle is set but none of inline, secretRef, or configMapRef is specified")
	}
	if sourcesSet > 1 {
		return nil, fmt.Errorf("caBundle has more than one source set: exactly one of inline, secretRef, or configMapRef must be specified")
	}

	if spec.Inline != nil {
		return []byte(*spec.Inline), nil
	}

	if spec.SecretRef != nil {
		return resolveFromSecret(ctx, k8sClient, spec.SecretRef)
	}

	return resolveFromConfigMap(ctx, k8sClient, spec.ConfigMapRef)
}

func resolveFromSecret(ctx context.Context, k8sClient client.Client, ref *v1alpha1.ResourceKeyRef) ([]byte, error) {
	key := ref.Key
	if key == "" {
		key = defaultCABundleKey
	}

	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, secret); err != nil {
		return nil, fmt.Errorf("caBundle: failed to get Secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	data, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("caBundle: key %q not found in Secret %s/%s", key, ref.Namespace, ref.Name)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("caBundle: key %q in Secret %s/%s is empty", key, ref.Namespace, ref.Name)
	}
	return data, nil
}

func resolveFromConfigMap(ctx context.Context, k8sClient client.Client, ref *v1alpha1.ResourceKeyRef) ([]byte, error) {
	key := ref.Key
	if key == "" {
		key = defaultCABundleKey
	}

	cm := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, cm); err != nil {
		return nil, fmt.Errorf("caBundle: failed to get ConfigMap %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	data, ok := cm.Data[key]
	if !ok {
		return nil, fmt.Errorf("caBundle: key %q not found in ConfigMap %s/%s", key, ref.Namespace, ref.Name)
	}
	if data == "" {
		return nil, fmt.Errorf("caBundle: key %q in ConfigMap %s/%s is empty", key, ref.Namespace, ref.Name)
	}
	return []byte(data), nil
}
