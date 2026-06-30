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

// Package composite_with_auth contains integration tests for webhook
// authentication and connection configuration in CompositeControllers.
package composite_with_auth

import (
	"context"
	"net/url"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	v1 "metacontroller/pkg/controller/composite/api/v1"
	"metacontroller/test/integration/framework"
)

const (
	testBearerToken = "test-integration-token" //nolint:gosec
	testUsername    = "testuser"
	testPassword    = "testpass" //nolint:gosec

	tokenSecretKey = "token"
	usernameKey    = "username"
	passwordKey    = "password"
)

func TestMain(m *testing.M) {
	framework.TestMain(m.Run)
}

// TestBearerTokenAuth verifies per-hook bearer token authentication.
// The happy path confirms the child is created when credentials match.
// The unhappy path confirms the child is never created when they do not.
func TestBearerTokenAuth(t *testing.T) {
	cases := []struct {
		name        string
		secretToken string
		authSucceeds bool
	}{
		{name: "CorrectToken", secretToken: testBearerToken, authSucceeds: true},
		{name: "WrongToken", secretToken: "wrong-token", authSucceeds: false}, //nolint:gosec
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ns := "test-bearer-" + strings.ToLower(tc.name)
			parentName := ns
			labels := map[string]string{"test": ns}

			f := framework.NewFixture(t)
			defer f.TearDown()

			f.CreateNamespace(ns)
			parentCRD, parentClient := f.CreateCRD("TestBearerParent"+tc.name, apiextensions.NamespaceScoped)
			childCRD, childClient := f.CreateCRD("TestBearerChild"+tc.name, apiextensions.NamespaceScoped)

			srv, caPEM := f.ServeWebhookTLSWithBearerAuth(testBearerToken, func(body []byte) ([]byte, error) {
				req := v1.CompositeHookRequest{}
				if err := json.Unmarshal(body, &req); err != nil {
					return nil, err
				}
				child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
				child.SetLabels(labels)
				resp := v1.CompositeHookResponse{Children: []*unstructured.Unstructured{child}}
				return json.Marshal(resp)
			})

			tokenSecret := f.CreateSecret(ns, "bearer-token-secret", map[string][]byte{
				tokenSecretKey: []byte(tc.secretToken),
			})

			f.CreateCompositeControllerWithBearerAuth(
				parentName,
				srv.URL+"/sync",
				caPEM,
				tokenSecret.Namespace, tokenSecret.Name, tokenSecretKey,
				framework.CRDResourceRule(parentCRD),
				framework.CRDResourceRule(childCRD),
			)

			parent := framework.UnstructuredCRD(parentCRD, parentName)
			unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
			if _, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}

			if tc.authSucceeds {
				t.Log("Waiting for child to be created...")
				if err := f.Wait(func() (bool, error) {
					_, err := childClient.Namespace(ns).Get(context.TODO(), parentName, metav1.GetOptions{})
					return err == nil, nil
				}); err != nil {
					t.Errorf("didn't find expected child: %v", err)
				}
			} else {
				t.Log("Waiting for SyncError event...")
				if err := f.WaitForSyncError(ns); err != nil {
					t.Errorf("expected SyncError event but timed out: %v", err)
				}
				_, err := childClient.Namespace(ns).Get(context.TODO(), parentName, metav1.GetOptions{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("child should not exist, got: %v", err)
				}
			}
		})
	}
}

// TestBasicAuth verifies per-hook HTTP Basic Authentication.
// The happy path confirms the child is created when credentials match.
// The unhappy path confirms the child is never created when they do not.
func TestBasicAuth(t *testing.T) {
	cases := []struct {
		name           string
		secretPassword string
		authSucceeds   bool
	}{
		{name: "CorrectCreds", secretPassword: testPassword, authSucceeds: true},
		{name: "WrongPassword", secretPassword: "wrongpass", authSucceeds: false}, //nolint:gosec
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ns := "test-basic-" + strings.ToLower(tc.name)
			parentName := ns
			labels := map[string]string{"test": ns}

			f := framework.NewFixture(t)
			defer f.TearDown()

			f.CreateNamespace(ns)
			parentCRD, parentClient := f.CreateCRD("TestBasicParent"+tc.name, apiextensions.NamespaceScoped)
			childCRD, childClient := f.CreateCRD("TestBasicChild"+tc.name, apiextensions.NamespaceScoped)

			srv, caPEM := f.ServeWebhookTLSWithBasicAuth(testUsername, testPassword, func(body []byte) ([]byte, error) {
				req := v1.CompositeHookRequest{}
				if err := json.Unmarshal(body, &req); err != nil {
					return nil, err
				}
				child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
				child.SetLabels(labels)
				resp := v1.CompositeHookResponse{Children: []*unstructured.Unstructured{child}}
				return json.Marshal(resp)
			})

			credSecret := f.CreateSecret(ns, "basic-auth-secret", map[string][]byte{
				usernameKey: []byte(testUsername),
				passwordKey: []byte(tc.secretPassword),
			})

			f.CreateCompositeControllerWithBasicAuth(
				parentName,
				srv.URL+"/sync",
				caPEM,
				credSecret.Namespace, credSecret.Name,
				framework.CRDResourceRule(parentCRD),
				framework.CRDResourceRule(childCRD),
			)

			parent := framework.UnstructuredCRD(parentCRD, parentName)
			unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
			if _, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}

			if tc.authSucceeds {
				t.Log("Waiting for child to be created...")
				if err := f.Wait(func() (bool, error) {
					_, err := childClient.Namespace(ns).Get(context.TODO(), parentName, metav1.GetOptions{})
					return err == nil, nil
				}); err != nil {
					t.Errorf("didn't find expected child: %v", err)
				}
			} else {
				t.Log("Waiting for SyncError event...")
				if err := f.WaitForSyncError(ns); err != nil {
					t.Errorf("expected SyncError event but timed out: %v", err)
				}
				_, err := childClient.Namespace(ns).Get(context.TODO(), parentName, metav1.GetOptions{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("child should not exist, got: %v", err)
				}
			}
		})
	}
}

// TestEndpointConfigsBearerToken verifies that endpointConfigs-level bearer token auth
// is applied to a webhook that carries no per-hook auth fields.
// The happy path confirms the child is created when credentials match.
// The unhappy path confirms the child is never created when they do not.
func TestEndpointConfigsBearerToken(t *testing.T) {
	cases := []struct {
		name        string
		secretToken string
		authSucceeds bool
	}{
		{name: "CorrectToken", secretToken: testBearerToken, authSucceeds: true},
		{name: "WrongToken", secretToken: "wrong-token", authSucceeds: false}, //nolint:gosec
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ns := "test-conn-bearer-" + strings.ToLower(tc.name)
			parentName := ns
			labels := map[string]string{"test": ns}

			f := framework.NewFixture(t)
			defer f.TearDown()

			f.CreateNamespace(ns)
			parentCRD, parentClient := f.CreateCRD("TestConnParent"+tc.name, apiextensions.NamespaceScoped)
			childCRD, childClient := f.CreateCRD("TestConnChild"+tc.name, apiextensions.NamespaceScoped)

			srv, caPEM := f.ServeWebhookTLSWithBearerAuth(testBearerToken, func(body []byte) ([]byte, error) {
				req := v1.CompositeHookRequest{}
				if err := json.Unmarshal(body, &req); err != nil {
					return nil, err
				}
				child := framework.UnstructuredCRD(childCRD, req.Parent.GetName())
				child.SetLabels(labels)
				resp := v1.CompositeHookResponse{Children: []*unstructured.Unstructured{child}}
				return json.Marshal(resp)
			})

			tokenSecret := f.CreateSecret(ns, "conn-bearer-token-secret", map[string][]byte{
				tokenSecretKey: []byte(tc.secretToken),
			})

			parsedURL, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatalf("failed to parse server URL: %v", err)
			}

			inlinePEM := string(caPEM)
			endpointConfigs := []v1alpha1.EndpointConfig{
				{
					Host: parsedURL.Host,
					CABundle: &v1alpha1.CABundle{
						Inline: &inlinePEM,
					},
					Authorization: &v1alpha1.Authorization{
						Type: "Bearer",
						SecretRef: v1alpha1.SecretKeyRef{
							Namespace: tokenSecret.Namespace,
							Name:      tokenSecret.Name,
							Key:       tokenSecretKey,
						},
					},
				},
			}

			f.CreateCompositeControllerWithEndpointConfigs(
				parentName,
				srv.URL+"/sync",
				endpointConfigs,
				framework.CRDResourceRule(parentCRD),
				framework.CRDResourceRule(childCRD),
			)

			parent := framework.UnstructuredCRD(parentCRD, parentName)
			unstructured.SetNestedStringMap(parent.Object, labels, "spec", "selector", "matchLabels")
			if _, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}

			if tc.authSucceeds {
				t.Log("Waiting for child to be created...")
				if err := f.Wait(func() (bool, error) {
					_, err := childClient.Namespace(ns).Get(context.TODO(), parentName, metav1.GetOptions{})
					return err == nil, nil
				}); err != nil {
					t.Errorf("didn't find expected child: %v", err)
				}
			} else {
				t.Log("Waiting for SyncError event...")
				if err := f.WaitForSyncError(ns); err != nil {
					t.Errorf("expected SyncError event but timed out: %v", err)
				}
				_, err := childClient.Namespace(ns).Get(context.TODO(), parentName, metav1.GetOptions{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("child should not exist, got: %v", err)
				}
			}
		})
	}
}
