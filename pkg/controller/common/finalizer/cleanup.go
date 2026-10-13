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

package finalizer

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
)

const (
	// ControllerFinalizer is the finalizer that Metacontroller puts on a
	// CompositeController or DecoratorController that has a finalize hook.
	// It keeps the controller object until Metacontroller has removed the
	// parent finalizer from all parent objects.
	ControllerFinalizer = "metacontroller.io/parent-finalizer-cleanup"

	// CleanupBudget is the maximum time that one reconcile uses for the
	// updates that remove parent finalizers. After this time, the reconcile
	// stops and the
	// controller object goes back to the queue. Thus a cleanup of many parent
	// objects does not block the reconcile of the other controller objects.
	CleanupBudget = 5 * time.Second

	// cleanupPageSize is the number of objects in one list call.
	cleanupPageSize = 500
)

// ParentResource identifies one parent resource of a controller.
type ParentResource struct {
	APIVersion string
	Resource   string
}

// EnsureControllerFinalizer adds ControllerFinalizer to the controller object,
// if the object does not have it.
func EnsureControllerFinalizer(ctx context.Context, c client.Client, controller client.Object) error {
	if controllerutil.ContainsFinalizer(controller, ControllerFinalizer) {
		return nil
	}
	orig, ok := controller.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("can't copy %T", controller)
	}
	controllerutil.AddFinalizer(controller, ControllerFinalizer)
	return c.Patch(ctx, controller, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{}))
}

// RemoveControllerFinalizer removes ControllerFinalizer from the controller
// object, if the object has it.
func RemoveControllerFinalizer(ctx context.Context, c client.Client, controller client.Object) error {
	if !controllerutil.ContainsFinalizer(controller, ControllerFinalizer) {
		return nil
	}
	orig, ok := controller.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("can't copy %T", controller)
	}
	controllerutil.RemoveFinalizer(controller, ControllerFinalizer)
	return client.IgnoreNotFound(c.Patch(ctx, controller, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{})))
}

// cleanupClock measures the time budget of one cleanup. The time starts at
// the first update, not at the first list call. Thus each cleanup removes the
// finalizer from one object at least, also when the list calls are slow.
type cleanupClock struct {
	budget   time.Duration
	deadline time.Time
}

func (c *cleanupClock) expired() bool {
	if c.deadline.IsZero() {
		c.deadline = time.Now().Add(c.budget)
		return false
	}
	return time.Now().After(c.deadline)
}

// RemoveFromResources removes the finalizer from all objects of the given
// parent resources, in all namespaces. It reads the objects from the API
// server, not from an informer cache, thus it does not miss an object.
//
// It makes updates for the time of the budget at most. It returns the number
// of objects that it changed. It returns done == true only when no object of
// the resources has the finalizer. It returns done == false and no error when
// the budget was not sufficient or an object changed during the update. Call
// it again in these conditions.
//
// A resource that is not in discovery is ignored. The resource is deleted,
// thus it has no objects.
func (m *Manager) RemoveFromResources(ctx context.Context, dynClient *dynamicclientset.Clientset, parents []ParentResource, budget time.Duration) (removed int, done bool, err error) {
	if !dynClient.HasSynced() {
		return 0, false, errors.New("discovery is not synced")
	}
	clock := &cleanupClock{budget: budget}
	done = true
	var errs []error
	for _, parent := range parents {
		client, clientErr := dynClient.Resource(parent.APIVersion, parent.Resource)
		if clientErr != nil {
			continue
		}
		count, resourceDone, resourceErr := m.removeFromAll(ctx, client, clock)
		removed += count
		done = done && resourceDone
		if resourceErr != nil {
			errs = append(errs, resourceErr)
		}
	}
	return removed, done, errors.Join(errs...)
}

// removeFromAll removes the finalizer from all objects of the resource of the
// given client. See RemoveFromResources for the return values.
func (m *Manager) removeFromAll(ctx context.Context, client *dynamicclientset.ResourceClient, clock *cleanupClock) (removed int, done bool, err error) {
	done = true
	var errs []error
	opts := metav1.ListOptions{Limit: cleanupPageSize}
	for {
		list, listErr := client.List(ctx, opts)
		if listErr != nil {
			errs = append(errs, fmt.Errorf("can't list %s: %w", client.Name, listErr))
			return removed, false, errors.Join(errs...)
		}
		for i := range list.Items {
			obj := &list.Items[i]
			if !controllerutil.RemoveFinalizer(obj, m.Name) {
				continue
			}
			if clock.expired() {
				return removed, false, errors.Join(errs...)
			}
			_, updateErr := client.Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})
			switch {
			case updateErr == nil:
				removed++
			case apierrors.IsNotFound(updateErr):
				// The object is already deleted.
			case apierrors.IsConflict(updateErr):
				// The object changed after the list call. The next call
				// does this object again.
				done = false
			default:
				done = false
				errs = append(errs, fmt.Errorf("can't remove finalizer from %s %s/%s: %w", client.Kind, obj.GetNamespace(), obj.GetName(), updateErr))
			}
		}
		if list.GetContinue() == "" {
			return removed, done, errors.Join(errs...)
		}
		opts.Continue = list.GetContinue()
	}
}
