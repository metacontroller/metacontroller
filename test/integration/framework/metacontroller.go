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
	"context"

	"k8s.io/utils/ptr"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
)

func CRDResourceRule(crd *apiextensions.CustomResourceDefinition) *v1alpha1.ResourceRule {
	return &v1alpha1.ResourceRule{
		APIVersion: crd.Spec.Group + "/" + crd.Spec.Versions[0].Name,
		Resource:   crd.Spec.Names.Plural,
	}
}

// CreateCompositeController generates a test CompositeController and installs
// it in the test API server.
func (f *Fixture) CreateCompositeController(name, syncHookURL string, customizeHookUrl string, parentRule, childRule *v1alpha1.ResourceRule, labels *map[string]string) *v1alpha1.CompositeController {
	childResources := []v1alpha1.CompositeControllerChildResourceRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.CompositeControllerChildResourceRule{ResourceRule: *childRule})
	}

	var customizeHook *v1alpha1.Hook
	if len(customizeHookUrl) != 0 {
		customizeHook = &v1alpha1.Hook{
			Webhook: &v1alpha1.Webhook{
				URL: &customizeHookUrl,
			},
		}
	} else {
		customizeHook = nil
	}

	cc := &v1alpha1.CompositeController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.CompositeControllerSpec{
			// Set a big resyncPeriod so tests can precisely control when syncs happen.
			ResyncPeriodSeconds: ptr.To[int32](3600),
			ParentResource: v1alpha1.CompositeControllerParentResourceRule{
				ResourceRule:  *parentRule,
				LabelSelector: nil,
			},
			ChildResources: childResources,
			Hooks: &v1alpha1.CompositeControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
					},
				},
				Customize: customizeHook,
			},
		},
	}

	// Add labels if specified.
	if labels != nil {
		cc.ObjectMeta.Labels = *labels
	}

	err := f.metacontroller.Create(context.Background(), cc)
	if err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), cc)
	})

	return cc
}

// CreateCompositeControllerWithBearerAuth creates a CompositeController whose
// sync webhook uses per-hook caBundle TLS verification and bearer token
// authorization sourced from a Kubernetes Secret.
func (f *Fixture) CreateCompositeControllerWithBearerAuth(
	name, syncHookURL string,
	caBundlePEM []byte,
	tokenSecretNamespace, tokenSecretName, tokenSecretKey string,
	parentRule, childRule *v1alpha1.ResourceRule,
) *v1alpha1.CompositeController {
	childResources := []v1alpha1.CompositeControllerChildResourceRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.CompositeControllerChildResourceRule{ResourceRule: *childRule})
	}

	inlinePEM := string(caBundlePEM)
	cc := &v1alpha1.CompositeController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.CompositeControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			ParentResource: v1alpha1.CompositeControllerParentResourceRule{
				ResourceRule: *parentRule,
			},
			ChildResources: childResources,
			Hooks: &v1alpha1.CompositeControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
						CABundle: &v1alpha1.CABundle{
							Inline: &inlinePEM,
						},
						Authorization: &v1alpha1.Authorization{
							Type: "Bearer",
							SecretRef: v1alpha1.SecretKeyRef{
								Namespace: tokenSecretNamespace,
								Name:      tokenSecretName,
								Key:       tokenSecretKey,
							},
						},
					},
				},
			},
		},
	}

	if err := f.metacontroller.Create(context.Background(), cc); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), cc)
	})
	return cc
}

// CreateCompositeControllerWithBasicAuth creates a CompositeController whose
// sync webhook uses per-hook caBundle TLS verification and HTTP Basic
// authentication sourced from a Kubernetes Secret. The Secret must contain
// keys "username" and "password".
func (f *Fixture) CreateCompositeControllerWithBasicAuth(
	name, syncHookURL string,
	caBundlePEM []byte,
	credSecretNamespace, credSecretName string,
	parentRule, childRule *v1alpha1.ResourceRule,
) *v1alpha1.CompositeController {
	childResources := []v1alpha1.CompositeControllerChildResourceRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.CompositeControllerChildResourceRule{ResourceRule: *childRule})
	}

	inlinePEM := string(caBundlePEM)
	cc := &v1alpha1.CompositeController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.CompositeControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			ParentResource: v1alpha1.CompositeControllerParentResourceRule{
				ResourceRule: *parentRule,
			},
			ChildResources: childResources,
			Hooks: &v1alpha1.CompositeControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
						CABundle: &v1alpha1.CABundle{
							Inline: &inlinePEM,
						},
						BasicAuth: &v1alpha1.BasicAuth{
							SecretRef: v1alpha1.SecretRef{
								Namespace: credSecretNamespace,
								Name:      credSecretName,
							},
						},
					},
				},
			},
		},
	}

	if err := f.metacontroller.Create(context.Background(), cc); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), cc)
	})
	return cc
}

// CreateCompositeControllerWithConnections creates a CompositeController whose
// TLS and authentication are configured via spec.connections rather than
// per-hook fields. The sync webhook URL is used as-is with no per-hook auth.
func (f *Fixture) CreateCompositeControllerWithConnections(
	name, syncHookURL string,
	connections []v1alpha1.WebhookConnection,
	parentRule, childRule *v1alpha1.ResourceRule,
) *v1alpha1.CompositeController {
	childResources := []v1alpha1.CompositeControllerChildResourceRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.CompositeControllerChildResourceRule{ResourceRule: *childRule})
	}

	cc := &v1alpha1.CompositeController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.CompositeControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			ParentResource: v1alpha1.CompositeControllerParentResourceRule{
				ResourceRule: *parentRule,
			},
			ChildResources: childResources,
			Connections:    connections,
			Hooks: &v1alpha1.CompositeControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
					},
				},
			},
		},
	}

	if err := f.metacontroller.Create(context.Background(), cc); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), cc)
	})
	return cc
}

// CreateDecoratorControllerWithBearerAuth creates a DecoratorController whose
// sync webhook uses per-hook caBundle TLS verification and bearer token
// authorization sourced from a Kubernetes Secret.
func (f *Fixture) CreateDecoratorControllerWithBearerAuth(
	name, syncHookURL string,
	caBundlePEM []byte,
	tokenSecretNamespace, tokenSecretName, tokenSecretKey string,
	parentRule, childRule *v1alpha1.ResourceRule,
) *v1alpha1.DecoratorController {
	childResources := []v1alpha1.DecoratorControllerAttachmentRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.DecoratorControllerAttachmentRule{ResourceRule: *childRule})
	}

	inlinePEM := string(caBundlePEM)
	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: *parentRule},
			},
			Attachments: childResources,
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
						CABundle: &v1alpha1.CABundle{
							Inline: &inlinePEM,
						},
						Authorization: &v1alpha1.Authorization{
							Type: "Bearer",
							SecretRef: v1alpha1.SecretKeyRef{
								Namespace: tokenSecretNamespace,
								Name:      tokenSecretName,
								Key:       tokenSecretKey,
							},
						},
					},
				},
			},
		},
	}

	if err := f.metacontroller.Create(context.Background(), dc); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), dc)
	})
	return dc
}

// CreateDecoratorControllerWithBasicAuth creates a DecoratorController whose
// sync webhook uses per-hook caBundle TLS verification and HTTP Basic
// authentication sourced from a Kubernetes Secret. The Secret must contain
// keys "username" and "password".
func (f *Fixture) CreateDecoratorControllerWithBasicAuth(
	name, syncHookURL string,
	caBundlePEM []byte,
	credSecretNamespace, credSecretName string,
	parentRule, childRule *v1alpha1.ResourceRule,
) *v1alpha1.DecoratorController {
	childResources := []v1alpha1.DecoratorControllerAttachmentRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.DecoratorControllerAttachmentRule{ResourceRule: *childRule})
	}

	inlinePEM := string(caBundlePEM)
	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: *parentRule},
			},
			Attachments: childResources,
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
						CABundle: &v1alpha1.CABundle{
							Inline: &inlinePEM,
						},
						BasicAuth: &v1alpha1.BasicAuth{
							SecretRef: v1alpha1.SecretRef{
								Namespace: credSecretNamespace,
								Name:      credSecretName,
							},
						},
					},
				},
			},
		},
	}

	if err := f.metacontroller.Create(context.Background(), dc); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), dc)
	})
	return dc
}

// CreateDecoratorControllerWithConnections creates a DecoratorController where
// TLS and auth are configured via spec.connections rather than per-hook fields.
func (f *Fixture) CreateDecoratorControllerWithConnections(
	name, syncHookURL string,
	connections []v1alpha1.WebhookConnection,
	parentRule, childRule *v1alpha1.ResourceRule,
) *v1alpha1.DecoratorController {
	childResources := []v1alpha1.DecoratorControllerAttachmentRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.DecoratorControllerAttachmentRule{ResourceRule: *childRule})
	}

	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: *parentRule},
			},
			Attachments: childResources,
			Connections: connections,
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
					},
				},
			},
		},
	}

	if err := f.metacontroller.Create(context.Background(), dc); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), dc)
	})
	return dc
}

// CreateDecoratorController generates a test DecoratorController and installs
// it in the test API server.
func (f *Fixture) CreateDecoratorController(name, syncHookURL, customizeHookUrl string, parentRule, childRule *v1alpha1.ResourceRule, labels *map[string]string) *v1alpha1.DecoratorController {
	childResources := []v1alpha1.DecoratorControllerAttachmentRule{}
	if childRule != nil {
		childResources = append(childResources, v1alpha1.DecoratorControllerAttachmentRule{ResourceRule: *childRule})
	}

	var customizeHook *v1alpha1.Hook
	if len(customizeHookUrl) != 0 {
		customizeHook = &v1alpha1.Hook{
			Webhook: &v1alpha1.Webhook{
				URL: &customizeHookUrl,
			},
		}
	} else {
		customizeHook = nil
	}

	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			// Set a big resyncPeriod so tests can precisely control when syncs happen.
			ResyncPeriodSeconds: ptr.To[int32](3600),
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{
					ResourceRule: *parentRule,
				},
			},
			Attachments: childResources,
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
					},
				},
				Customize: customizeHook,
			},
		},
	}

	// Add labels if specified.
	if labels != nil {
		dc.ObjectMeta.Labels = *labels
	}

	err := f.metacontroller.Create(context.Background(), dc)
	if err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.Delete(context.Background(), dc)
	})

	return dc
}
