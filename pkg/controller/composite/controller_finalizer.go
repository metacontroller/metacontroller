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
	"time"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/finalizer"
	"metacontroller/pkg/events"
)

// parentFinalizerName returns the name of the finalizer that the
// CompositeController with the given name puts on its parent objects.
func parentFinalizerName(controllerName string) string {
	return "metacontroller.io/compositecontroller-" + controllerName
}

// ensureControllerFinalizer puts finalizer.ControllerFinalizer on a
// CompositeController that has a finalize hook. Only such a controller puts a
// finalizer on its parent objects.
func (mc *Metacontroller) ensureControllerFinalizer(ctx context.Context, cc *v1alpha1.CompositeController) error {
	if cc.Spec.Hooks == nil || cc.Spec.Hooks.Finalize == nil {
		return nil
	}
	return finalizer.EnsureControllerFinalizer(ctx, mc.k8sClient, cc)
}

// finalizeCompositeController does the work for a CompositeController that is pending
// deletion. It stops the controller, removes the parent finalizer from all
// parent objects, and then removes finalizer.ControllerFinalizer, which lets
// the API server delete the CompositeController.
//
// One call removes parent finalizers for finalizer.CleanupBudget at most. If
// parent objects are left, the CompositeController goes back to the queue.
// The finalize hook is not called.
func (mc *Metacontroller) finalizeCompositeController(ctx context.Context, cc *v1alpha1.CompositeController) (reconcile.Result, error) {
	// Stop the controller first. A worker that is in operation adds the
	// parent finalizer again.
	if pc, ok := mc.parentControllers.LoadAndDelete(cc.Name); ok {
		pc.Stop()
		mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", cc.Name)
	}
	if !controllerutil.ContainsFinalizer(cc, finalizer.ControllerFinalizer) {
		return reconcile.Result{}, nil
	}

	parents := []finalizer.ParentResource{
		{APIVersion: cc.Spec.ParentResource.APIVersion, Resource: cc.Spec.ParentResource.Resource},
	}
	manager := finalizer.NewManager(parentFinalizerName(cc.Name), true)
	removed, done, err := manager.RemoveFromResources(ctx, mc.dynClient, parents, finalizer.CleanupBudget)
	if removed > 0 || done {
		mc.logger.Info("Removed the parent finalizer from parent objects", "name", cc.Name, "removed", removed, "done", done)
	}
	if err != nil {
		mc.eventRecorder.Eventf(cc, v1.EventTypeWarning, events.ReasonSyncError, "[%s] can't remove the parent finalizer - %s", cc.Name, err)
		return reconcile.Result{}, err
	}
	if !done {
		// Go to the end of the queue, so that the other controller objects
		// get their reconcile.
		return reconcile.Result{RequeueAfter: time.Millisecond}, nil
	}
	return reconcile.Result{}, finalizer.RemoveControllerFinalizer(ctx, mc.k8sClient, cc)
}
