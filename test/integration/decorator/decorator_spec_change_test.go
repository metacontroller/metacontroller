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

package decorator

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/test/integration/framework"
)

// specChangeCase returns the spec-change test case for a DecoratorController
// with the given name.
func specChangeCase(name string) framework.SpecChangeCase {
	return framework.SpecChangeCase{
		NewController: func(hookURL string, parent v1alpha1.ResourceRule) client.Object {
			return &v1alpha1.DecoratorController{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name},
				Spec: v1alpha1.DecoratorControllerSpec{
					ResyncPeriodSeconds: ptr.To[int32](3600),
					Resources:           []v1alpha1.DecoratorControllerResourceRule{{ResourceRule: parent}},
					Hooks: &v1alpha1.DecoratorControllerHooks{
						Sync:     &v1alpha1.Hook{Webhook: &v1alpha1.Webhook{URL: &hookURL}},
						Finalize: &v1alpha1.Hook{Webhook: &v1alpha1.Webhook{URL: &hookURL}},
					},
				},
			}
		},
		SetParent: func(controller client.Object, parent v1alpha1.ResourceRule) {
			controller.(*v1alpha1.DecoratorController).Spec.Resources = []v1alpha1.DecoratorControllerResourceRule{{ResourceRule: parent}}
		},
		ParentFinalizer: "metacontroller.io/decoratorcontroller-" + name,
	}
}

func TestFinalizerCleanup_whenParentResourceChanges_removesOldParentFinalizer(t *testing.T) {
	f := framework.NewFixture(t)
	defer f.TearDown()
	f.RunSpecChangeCleanup(specChangeCase("spec-change"), "test-spec-change")
}
