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

package composite

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/finalizer"
)

// parentFinalizerName returns the name of the finalizer that the
// CompositeController with the given name adds to its parent objects.
func parentFinalizerName(controllerName string) string {
	return "metacontroller.io/compositecontroller-" + controllerName
}

// ensureControllerFinalizer adds finalizer.ControllerFinalizer to a
// CompositeController that has a finalize hook. Only such a controller adds a
// finalizer to its parent objects.
func (mc *Metacontroller) ensureControllerFinalizer(ctx context.Context, cc *v1alpha1.CompositeController) error {
	if cc.Spec.Hooks == nil || cc.Spec.Hooks.Finalize == nil {
		return nil
	}
	return finalizer.EnsureControllerFinalizer(ctx, mc.k8sClient, cc)
}

// finalizeCompositeController does the cleanup for a CompositeController that is
// pending deletion. See finalizer.ControllerCleaner.Finalize.
func (mc *Metacontroller) finalizeCompositeController(ctx context.Context, cc *v1alpha1.CompositeController) (reconcile.Result, error) {
	cleaner := &finalizer.ControllerCleaner{
		K8sClient:     mc.k8sClient,
		DynClient:     mc.dynClient,
		Resources:     mc.resources,
		EventRecorder: mc.eventRecorder,
		Logger:        mc.logger,
	}
	parents := []finalizer.ParentResource{
		{APIVersion: cc.Spec.ParentResource.APIVersion, Resource: cc.Spec.ParentResource.Resource},
	}
	stop := func() bool {
		pc, ok := mc.parentControllers.LoadAndDelete(cc.Name)
		if ok {
			pc.Stop()
		}
		return ok
	}
	return cleaner.Finalize(ctx, cc, parentFinalizerName(cc.Name), parents, stop)
}
