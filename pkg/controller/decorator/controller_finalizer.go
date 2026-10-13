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
// DecoratorController with the given name puts on its parent objects.
func parentFinalizerName(controllerName string) string {
	return "metacontroller.io/decoratorcontroller-" + controllerName
}

// ensureControllerFinalizer puts finalizer.ControllerFinalizer on a
// DecoratorController that has a finalize hook. Only such a controller puts a
// finalizer on its parent objects.
func (mc *Metacontroller) ensureControllerFinalizer(ctx context.Context, dc *v1alpha1.DecoratorController) error {
	if dc.Spec.Hooks == nil || dc.Spec.Hooks.Finalize == nil {
		return nil
	}
	return finalizer.EnsureControllerFinalizer(ctx, mc.k8sClient, dc)
}

// finalizeDecoratorController does the work for a DecoratorController that is pending
// deletion. It stops the controller, removes the parent finalizer from all
// parent objects, and then removes finalizer.ControllerFinalizer, which lets
// the API server delete the DecoratorController.
//
// One call removes parent finalizers for finalizer.CleanupBudget at most. If
// parent objects are left, the DecoratorController goes back to the queue.
// The finalize hook is not called.
func (mc *Metacontroller) finalizeDecoratorController(ctx context.Context, dc *v1alpha1.DecoratorController) (reconcile.Result, error) {
	// Stop the controller first. A worker that is in operation adds the
	// parent finalizer again.
	if c, ok := mc.decoratorControllers.LoadAndDelete(dc.Name); ok {
		c.Stop()
		mc.eventRecorder.Eventf(dc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", dc.Name)
	}
	if !controllerutil.ContainsFinalizer(dc, finalizer.ControllerFinalizer) {
		return reconcile.Result{}, nil
	}

	parents := make([]finalizer.ParentResource, 0, len(dc.Spec.Resources))
	for _, resource := range dc.Spec.Resources {
		parents = append(parents, finalizer.ParentResource{APIVersion: resource.APIVersion, Resource: resource.Resource})
	}
	manager := finalizer.NewManager(parentFinalizerName(dc.Name), true)
	removed, done, err := manager.RemoveFromResources(ctx, mc.dynClient, parents, finalizer.CleanupBudget)
	if removed > 0 || done {
		mc.logger.Info("Removed the parent finalizer from parent objects", "name", dc.Name, "removed", removed, "done", done)
	}
	if err != nil {
		mc.eventRecorder.Eventf(dc, v1.EventTypeWarning, events.ReasonSyncError, "[%s] can't remove the parent finalizer - %s", dc.Name, err)
		return reconcile.Result{}, err
	}
	if !done {
		// Go to the end of the queue, so that the other controller objects
		// get their reconcile.
		return reconcile.Result{RequeueAfter: time.Millisecond}, nil
	}
	return reconcile.Result{}, finalizer.RemoveControllerFinalizer(ctx, mc.k8sClient, dc)
}
