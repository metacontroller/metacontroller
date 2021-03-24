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
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"metacontroller.io/apis/metacontroller/v1alpha1"
)

func CRDResourceRule(crd *apiextensions.CustomResourceDefinition) *v1alpha1.ResourceRule {
	return &v1alpha1.ResourceRule{
		APIVersion: crd.Spec.Group + "/" + crd.Spec.Versions[0].Name,
		Resource:   crd.Spec.Names.Plural,
	}
}

// CreateCompositeController generates a test CompositeController and installs
// it in the test API server.
func (f *Fixture) CreateCompositeController(name, syncHookURL string, customizeHookUrl string, parentRule, childRule *v1alpha1.ResourceRule) *v1alpha1.CompositeController {
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
			ResyncPeriodSeconds: pointer.Int32Ptr(3600),
			ParentResource: v1alpha1.CompositeControllerParentResourceRule{
				ResourceRule: *parentRule,
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

	cc, err := f.metacontroller.MetacontrollerV1alpha1().CompositeControllers().Create(cc)
	if err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.MetacontrollerV1alpha1().CompositeControllers().Delete(cc.Name, nil)
	})

	return cc
}

// CreateDecoratorController generates a test DecoratorController and installs
// it in the test API server.
func (f *Fixture) CreateDecoratorController(name, syncHookURL string, customizeHookUrl string, parentRule, childRule *v1alpha1.ResourceRule) *v1alpha1.DecoratorController {
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
			ResyncPeriodSeconds: pointer.Int32Ptr(3600),
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

	dc, err := f.metacontroller.MetacontrollerV1alpha1().DecoratorControllers().Create(dc)
	if err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.metacontroller.MetacontrollerV1alpha1().DecoratorControllers().Delete(dc.Name, nil)
	})

	return dc
}
