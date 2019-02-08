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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"metacontroller.app/apis/metacontroller/v1alpha1"
)

// CreateCompositeController generates a test CompositeController and installs
// it in the test API server.
func (f *Fixture) CreateCompositeController(name, syncHookURL string, parentCRD, childCRD *apiextensions.CustomResourceDefinition) *v1alpha1.CompositeController {
	cc := &v1alpha1.CompositeController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.CompositeControllerSpec{
			ParentResource: v1alpha1.CompositeControllerParentResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: parentCRD.Spec.Group + "/" + parentCRD.Spec.Versions[0].Name,
					Resource:   parentCRD.Spec.Names.Plural,
				},
			},
			ChildResources: []v1alpha1.CompositeControllerChildResourceRule{
				{
					ResourceRule: v1alpha1.ResourceRule{
						APIVersion: childCRD.Spec.Group + "/" + childCRD.Spec.Versions[0].Name,
						Resource:   childCRD.Spec.Names.Plural,
					},
				},
			},
			Hooks: &v1alpha1.CompositeControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
					},
				},
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
func (f *Fixture) CreateDecoratorController(name, syncHookURL string, parentCRD, childCRD *apiextensions.CustomResourceDefinition) *v1alpha1.DecoratorController {
	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{
					ResourceRule: v1alpha1.ResourceRule{
						APIVersion: parentCRD.Spec.Group + "/" + parentCRD.Spec.Versions[0].Name,
						Resource:   parentCRD.Spec.Names.Plural,
					},
				},
			},
			Attachments: []v1alpha1.DecoratorControllerAttachmentRule{
				{
					ResourceRule: v1alpha1.ResourceRule{
						APIVersion: childCRD.Spec.Group + "/" + childCRD.Spec.Versions[0].Name,
						Resource:   childCRD.Spec.Names.Plural,
					},
				},
			},
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL: &syncHookURL,
					},
				},
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
