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
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	"metacontroller/pkg/events"
)

const (
	// ControllerFinalizer is the finalizer that Metacontroller adds to a
	// CompositeController or DecoratorController that has a finalize hook.
	// It keeps the controller object until Metacontroller has removed the
	// parent finalizer from all parent objects.
	ControllerFinalizer = "metacontroller.io/parent-finalizer-cleanup"

	// CleanupBudget limits the time that one reconcile uses to remove parent
	// finalizers. The code examines the budget only between calls, thus the
	// last call can end later. Then the controller object goes back to the
	// queue. Thus a cleanup of many parent objects does not block the
	// reconcile of the other controller objects.
	CleanupBudget = 5 * time.Second

	// cleanupPageSize is the number of objects in one list call.
	cleanupPageSize = 500
)

// cleanupCallTimeout is the maximum time of one list call or one update call
// of the cleanup. The code examines CleanupBudget only between calls, thus a
// slow call needs its own limit. The time includes the wait for the client
// rate limiter. It is a variable so that a test can change it.
var cleanupCallTimeout = 15 * time.Second

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

// ControllerCleaner does the cleanup for a CompositeController or a
// DecoratorController that is pending deletion. The two controller kinds use
// it, thus they have the same behavior.
type ControllerCleaner struct {
	K8sClient     client.Client
	DynClient     *dynamicclientset.Clientset
	Resources     *dynamicdiscovery.ResourceMap
	EventRecorder record.EventRecorder
	Logger        logr.Logger
	// Cursors keeps the position of the cleanup of each controller object
	// between reconciles. It must live longer than one reconcile.
	Cursors *Cursors
}

// Cursors keeps one Cursor for each controller object. The zero value is
// ready to use.
type Cursors struct {
	mu      sync.Mutex
	cursors map[types.UID]*Cursor
}

// get returns the Cursor of the controller object. It makes a new Cursor if
// the object does not have one.
func (c *Cursors) get(uid types.UID) *Cursor {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursors == nil {
		c.cursors = map[types.UID]*Cursor{}
	}
	if c.cursors[uid] == nil {
		c.cursors[uid] = &Cursor{}
	}
	return c.cursors[uid]
}

func (c *Cursors) delete(uid types.UID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cursors, uid)
}

// Finalize does the cleanup for a controller object that is pending deletion.
// It stops the controller. Then it removes the parent finalizer from all parent
// objects. Then it removes ControllerFinalizer, so that the API server can
// delete the object. If the object does not have ControllerFinalizer, Finalize
// only stops the controller.
//
// The stop function stops the controller of the object, if there is one, and
// returns true if it stopped a controller. It must return only after all
// workers of the controller have ended. Otherwise a worker can add the parent
// finalizer again after the cleanup.
//
// One call works for approximately CleanupBudget. If the cleanup is not
// complete, the controller object goes back to the queue, and the next call
// continues at the same position. Finalize does not call the finalize hook.
func (c *ControllerCleaner) Finalize(ctx context.Context, controller client.Object, parentFinalizer string, parents []ParentResource, stop func() bool) (reconcile.Result, error) {
	name := controller.GetName()
	// Stop first, so that no worker adds the parent finalizer again.
	if stop() {
		c.EventRecorder.Eventf(controller, corev1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", name)
	}
	if !controllerutil.ContainsFinalizer(controller, ControllerFinalizer) {
		c.Cursors.delete(controller.GetUID())
		return reconcile.Result{}, nil
	}

	// A parent resource that is not in discovery needs a check. The cleanup
	// must not complete while objects of the resource can exist.
	parents, err := c.resolveParents(ctx, parents)
	if err != nil {
		c.EventRecorder.Eventf(controller, corev1.EventTypeWarning, events.ReasonSyncError, "[%s] can't remove the parent finalizer - %s", name, err)
		return reconcile.Result{}, err
	}

	cursor := c.Cursors.get(controller.GetUID())
	removed, done, err := NewManager(parentFinalizer, true).RemoveFromResources(ctx, c.DynClient, parents, CleanupBudget, cursor)
	if done {
		c.Cursors.delete(controller.GetUID())
	}
	if removed > 0 || done {
		c.Logger.Info("Removed the parent finalizer from parent objects", "name", name, "removed", removed, "done", done)
	}
	if err != nil {
		c.EventRecorder.Eventf(controller, corev1.EventTypeWarning, events.ReasonSyncError, "[%s] can't remove the parent finalizer - %s", name, err)
		return reconcile.Result{}, err
	}
	if !done {
		// Go to the end of the queue, so that the other controller objects
		// get their reconcile.
		return reconcile.Result{RequeueAfter: time.Millisecond}, nil
	}
	return reconcile.Result{}, RemoveControllerFinalizer(ctx, c.K8sClient, controller)
}

var (
	crdGVK        = schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}
	apiServiceGVK = schema.GroupVersionKind{Group: "apiregistration.k8s.io", Version: "v1", Kind: "APIService"}
)

// resolveParents returns the parent resources to clean. It returns a resource
// that is in discovery without a change.
//
// The discovery data is a cache, and a resource that is not in it can have
// objects. For such a resource, resolveParents asks the API server:
//   - If the CRD of the resource exists, it uses a served version of the CRD
//     that is in discovery. An object has the same finalizers in all versions. If
//     no served version is in discovery, it returns an error.
//   - If there is no CRD and discovery has the apiVersion, the resource does
//     not exist, and resolveParents removes it from the result.
//   - If there is no CRD and no APIService for the apiVersion, the API does
//     not exist, and resolveParents removes the resource from the result.
//   - If the APIService exists, the API is not in discovery at this time, and
//     resolveParents returns an error.
//
// An error makes the reconcile fail, and the reconcile is done again later.
func (c *ControllerCleaner) resolveParents(ctx context.Context, parents []ParentResource) ([]ParentResource, error) {
	resolved := make([]ParentResource, 0, len(parents))
	for _, parent := range parents {
		if _, err := c.DynClient.Resource(parent.APIVersion, parent.Resource); err == nil {
			resolved = append(resolved, parent)
			continue
		}
		served, err := c.servedParent(ctx, parent)
		if err != nil {
			return nil, err
		}
		if served != nil {
			resolved = append(resolved, *served)
		}
	}
	return resolved, nil
}

// servedParent finds a served version of the parent resource that is in
// discovery. resolveParents calls it when the apiVersion of the controller
// spec is not in discovery. It returns nil when the resource does not exist.
func (c *ControllerCleaner) servedParent(ctx context.Context, parent ParentResource) (*ParentResource, error) {
	gv, err := schema.ParseGroupVersion(parent.APIVersion)
	if err != nil {
		// No controller can start with this apiVersion, thus no parent object
		// has the finalizer.
		return nil, nil //nolint:nilerr // an invalid apiVersion has no objects
	}

	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGVK)
	err = c.K8sClient.Get(ctx, client.ObjectKey{Name: parent.Resource + "." + gv.Group}, crd)
	if err == nil {
		versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
		for _, v := range versions {
			version, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			name, _, _ := unstructured.NestedString(version, "name")
			served, _, _ := unstructured.NestedBool(version, "served")
			apiVersion := schema.GroupVersion{Group: gv.Group, Version: name}.String()
			if !served || c.Resources.Get(apiVersion, parent.Resource) == nil {
				continue
			}
			return &ParentResource{APIVersion: apiVersion, Resource: parent.Resource}, nil
		}
		return nil, fmt.Errorf("the CRD of resource %s exists, but no served version of it is in discovery", parent.Resource)
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("can't get the CRD of resource %s: %w", parent.Resource, err)
	}

	if c.Resources.HasGroupVersion(parent.APIVersion) {
		// The API is in discovery and does not have the resource.
		return nil, nil
	}
	apiService := &unstructured.Unstructured{}
	apiService.SetGroupVersionKind(apiServiceGVK)
	err = c.K8sClient.Get(ctx, client.ObjectKey{Name: gv.Version + "." + gv.Group}, apiService)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("can't get the APIService of apiVersion %s: %w", parent.APIVersion, err)
	}
	return nil, fmt.Errorf("apiVersion %s is registered, but it is not in discovery", parent.APIVersion)
}

// Cursor is the position of a cleanup between calls of RemoveFromResources.
// One pass reads all pages of all parent resources. The zero value starts a
// new pass.
type Cursor struct {
	// Parent is the resource of the next page.
	Parent ParentResource
	// Continue is the continue token of the next page. It is empty for the
	// first page.
	Continue string
	// Skip is the number of objects at the start of the next page that the
	// cleanup already did. A page with a continue token is a snapshot, thus
	// it shows these objects with the finalizer again. The first page is
	// read again from the current data, thus Skip is 0 for it.
	Skip int
	// Retry is true when an object of this pass needs a new update.
	Retry bool
}

// cleanupClock measures the budget of one call of RemoveFromResources. The
// time starts at the start of the call. The call can always do one list call
// and one update, also after the deadline. Thus each call moves the cleanup
// forward.
type cleanupClock struct {
	deadline time.Time
	listed   bool
	updated  bool
}

func (c *cleanupClock) canList() bool {
	if c.listed && time.Now().After(c.deadline) {
		return false
	}
	c.listed = true
	return true
}

func (c *cleanupClock) canUpdate() bool {
	if c.updated && time.Now().After(c.deadline) {
		return false
	}
	c.updated = true
	return true
}

// RemoveFromResources removes the finalizer from all objects of the given
// parent resources, in all namespaces. It reads the objects from the API
// server, not from an informer cache, thus it does not miss an object.
//
// It starts at the position of the cursor, works for the time of the budget,
// and then keeps its position in the cursor. Each list call and each update
// call has a time limit of cleanupCallTimeout. It returns the number of
// objects that it changed. It returns done == true only when a full pass
// removed the finalizer from each object that had it, with no conflict and no
// error. It returns done == false and no error when the cleanup is not
// complete. Call it again with the same cursor in this condition.
//
// It returns an error for a resource that is not in discovery, because
// objects of that resource can exist.
func (m *Manager) RemoveFromResources(ctx context.Context, dynClient *dynamicclientset.Clientset, parents []ParentResource, budget time.Duration, cursor *Cursor) (removed int, done bool, err error) {
	if !dynClient.HasSynced() {
		return 0, false, errors.New("discovery is not synced")
	}
	// Remove duplicate parents. The cursor finds its parent by value. Thus
	// with a duplicate, the second copy starts again at its first page on
	// each call.
	parents = uniqueParents(parents)
	start := slices.Index(parents, cursor.Parent)
	if start < 0 {
		// A new pass, or the parent resources changed. Start at the first one.
		*cursor = Cursor{}
		start = 0
	}
	clock := &cleanupClock{deadline: time.Now().Add(budget)}
	var errs []error
	for _, parent := range parents[start:] {
		if cursor.Parent != parent {
			*cursor = Cursor{Parent: parent, Retry: cursor.Retry}
		}
		client, clientErr := dynClient.Resource(parent.APIVersion, parent.Resource)
		if clientErr != nil {
			cursor.Retry = true
			errs = append(errs, clientErr)
			continue
		}
		count, complete, resourceErr := m.removeFromAll(ctx, client, clock, cursor)
		removed += count
		if resourceErr != nil {
			errs = append(errs, resourceErr)
		}
		if !complete {
			return removed, false, errors.Join(errs...)
		}
	}
	done = !cursor.Retry
	*cursor = Cursor{}
	return removed, done, errors.Join(errs...)
}

// uniqueParents returns the parents without duplicates, in the same order.
func uniqueParents(parents []ParentResource) []ParentResource {
	seen := make(map[ParentResource]bool, len(parents))
	unique := make([]ParentResource, 0, len(parents))
	for _, parent := range parents {
		if !seen[parent] {
			seen[parent] = true
			unique = append(unique, parent)
		}
	}
	return unique
}

// removeFromAll removes the finalizer from the objects of the resource of the
// given client. It starts at the page of the cursor and moves the cursor to
// the next page. It returns complete == true after it read the last page.
func (m *Manager) removeFromAll(ctx context.Context, client *dynamicclientset.ResourceClient, clock *cleanupClock, cursor *Cursor) (removed int, complete bool, err error) {
	var errs []error
	for {
		if !clock.canList() {
			return removed, false, errors.Join(errs...)
		}
		list, listErr := listWithTimeout(ctx, client, metav1.ListOptions{Limit: cleanupPageSize, Continue: cursor.Continue})
		if apierrors.IsResourceExpired(listErr) {
			// The continue token is too old. The next call starts again at the
			// first page of this resource.
			cursor.Continue, cursor.Skip = "", 0
			return removed, false, errors.Join(errs...)
		}
		if listErr != nil {
			errs = append(errs, fmt.Errorf("can't list %s: %w", client.Name, listErr))
			return removed, false, errors.Join(errs...)
		}
		for i := cursor.Skip; i < len(list.Items); i++ {
			obj := &list.Items[i]
			if !controllerutil.RemoveFinalizer(obj, m.Name) {
				continue
			}
			if !clock.canUpdate() {
				// The next call reads this page again, and skips the objects
				// that this call did.
				cursor.Skip = skipTo(cursor, i)
				return removed, false, errors.Join(errs...)
			}
			timedOut, updateErr := updateWithTimeout(ctx, client.Namespace(obj.GetNamespace()), obj)
			switch {
			case updateErr == nil:
				removed++
			case apierrors.IsNotFound(updateErr):
				// The object is already deleted.
			case apierrors.IsConflict(updateErr):
				// The object changed after the list call. The next pass does
				// this object again.
				cursor.Retry = true
			default:
				cursor.Retry = true
				errs = append(errs, fmt.Errorf("can't remove finalizer from %s %s/%s: %w", client.Kind, obj.GetNamespace(), obj.GetName(), updateErr))
			}
			if timedOut {
				// The API server or the rate limiter is slow. Stop, so that
				// this reconcile does not do more slow calls. Retry is set, thus
				// the next pass does this object again.
				cursor.Skip = skipTo(cursor, i+1)
				return removed, false, errors.Join(errs...)
			}
		}
		cursor.Continue, cursor.Skip = list.GetContinue(), 0
		if cursor.Continue == "" {
			return removed, true, errors.Join(errs...)
		}
	}
}

// skipTo returns the Skip value for a stop before object next of the current
// page. The first page is not a snapshot, thus its Skip is 0.
func skipTo(cursor *Cursor, next int) int {
	if cursor.Continue == "" {
		return 0
	}
	return next
}

func listWithTimeout(ctx context.Context, client *dynamicclientset.ResourceClient, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	callCtx, cancel := context.WithTimeout(ctx, cleanupCallTimeout)
	defer cancel()
	return client.List(callCtx, opts)
}

// updateWithTimeout returns timedOut == true when the update failed and the
// call context is done: cleanupCallTimeout is over, or ctx is canceled.
func updateWithTimeout(ctx context.Context, client *dynamicclientset.ResourceClient, obj *unstructured.Unstructured) (timedOut bool, err error) {
	callCtx, cancel := context.WithTimeout(ctx, cleanupCallTimeout)
	defer cancel()
	_, err = client.Update(callCtx, obj, metav1.UpdateOptions{})
	return err != nil && callCtx.Err() != nil, err
}
