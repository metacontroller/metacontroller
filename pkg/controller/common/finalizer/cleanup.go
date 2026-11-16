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
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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

// ParentResource identifies one parent resource of a controller. The API
// group and the resource name identify the resource. In the cleanup record,
// APIVersion only suggests a version to use.
type ParentResource struct {
	APIVersion string `json:"apiVersion"`
	Resource   string `json:"resource"`
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

// ControllerCleaner removes the parent finalizer for a CompositeController or
// a DecoratorController. It cleans the retired resources of a live controller
// object, and all resources of a controller object that has a deletion
// timestamp. The two controller kinds use it, so they have the same behavior.
type ControllerCleaner struct {
	K8sClient     client.Client
	DynClient     *dynamicclientset.Clientset
	Resources     *dynamicdiscovery.ResourceMap
	EventRecorder record.EventRecorder
	Logger        logr.Logger
	// Budget limits the time of one cleanup call. Zero means CleanupBudget.
	Budget time.Duration
	// Cursors keeps the position of the cleanup of each controller object
	// between reconciles. It must live longer than one reconcile.
	Cursors *Cursors
}

// Cursors keeps one Cursor for each controller object. The zero value is
// ready to use.
type Cursors struct {
	mu      sync.Mutex
	cursors map[types.NamespacedName]*cursorEntry
}

// cursorEntry is the Cursor of one controller object. A new object with the
// same name has a different UID and gets a new Cursor.
type cursorEntry struct {
	uid    types.UID
	cursor *Cursor
}

// get returns the Cursor of the controller object. It makes a new Cursor if
// the object does not have one.
func (c *Cursors) get(controller client.Object) *Cursor {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursors == nil {
		c.cursors = map[types.NamespacedName]*cursorEntry{}
	}
	key := client.ObjectKeyFromObject(controller)
	entry := c.cursors[key]
	if entry == nil || entry.uid != controller.GetUID() {
		entry = &cursorEntry{uid: controller.GetUID(), cursor: &Cursor{}}
		c.cursors[key] = entry
	}
	return entry.cursor
}

func (c *Cursors) delete(controller client.Object) {
	c.Delete(client.ObjectKeyFromObject(controller))
}

// Delete removes the Cursor of the controller object with the given name.
// Call it when the object is gone. Then the Cursor does not stay in memory.
func (c *Cursors) Delete(key types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cursors, key)
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
// One call works for approximately the Budget. If the cleanup is not
// complete, the controller object goes back to the queue, and the next call
// continues at the same position. Finalize does not call the finalize hook.
func (c *ControllerCleaner) Finalize(ctx context.Context, controller client.Object, parentFinalizer string, parents []ParentResource, stop func() bool) (reconcile.Result, error) {
	name := controller.GetName()
	// Stop first, so that no worker adds the parent finalizer again.
	if stop() {
		c.EventRecorder.Eventf(controller, corev1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", name)
	}
	if !controllerutil.ContainsFinalizer(controller, ControllerFinalizer) {
		c.Cursors.delete(controller)
		return reconcile.Result{}, nil
	}

	state, err := readCleanupState(controller)
	if err != nil {
		// Keep the controller finalizer. The record can name resources that
		// are not in parents.
		c.EventRecorder.Eventf(controller, corev1.EventTypeWarning, events.ReasonSyncError, "[%s] invalid cleanup record - %s", name, err)
		return reconcile.Result{}, err
	}

	// Clean the resources of the record and of the current spec. When there
	// is no record, an older Metacontroller version wrote this object.
	var entries []ParentResource
	if state != nil {
		entries = slices.Concat(state.Active, state.RemoveParentFinalizerFrom)
	}
	res := c.resolveEntries(ctx, uniqueParents(slices.Concat(entries, parents)))

	_, done, scanErr := c.removeFromResolved(ctx, controller, parentFinalizer, res.resolved)
	if err := errors.Join(append(res.errs, scanErr)...); err != nil {
		// Keep the controller finalizer. Do not return the error: then the
		// rate limiter of the queue does not set the requeue delay.
		c.reportCleanupError(controller, err)
		return reconcile.Result{RequeueAfter: cleanupErrorDelay}, nil
	}
	if !done {
		return reconcile.Result{RequeueAfter: cleanupRequeueDelay}, nil
	}
	// Report the skipped resources once, at the end of the cleanup.
	c.reportDropped(controller, res.dropped)
	c.Cursors.delete(controller)
	return reconcile.Result{}, removeCleanupStateAndFinalizer(ctx, c.K8sClient, controller)
}

func (c *ControllerCleaner) budget() time.Duration {
	if c.Budget == 0 {
		return CleanupBudget
	}
	return c.Budget
}

var (
	crdGVK        = schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}
	apiServiceGVK = schema.GroupVersionKind{Group: "apiregistration.k8s.io", Version: "v1", Kind: "APIService"}
)

type resolution struct {
	// resolved lists the resources to clean. Each has a served version.
	resolved []ParentResource
	// dropped lists the entries that have no objects to clean.
	dropped []schema.GroupResource
	// errs lists the errors of the entries that stay in the record.
	errs []error
}

// resolveEntries resolves the entries. An error in one entry does not stop
// the other entries.
func (c *ControllerCleaner) resolveEntries(ctx context.Context, entries []ParentResource) resolution {
	var res resolution
	for _, entry := range entries {
		parent, drop, err := c.resolveEntry(ctx, entry)
		switch {
		case err != nil:
			res.errs = append(res.errs, err)
		case drop:
			res.dropped = append(res.dropped, groupResourceOf(entry))
		default:
			res.resolved = append(res.resolved, *parent)
		}
	}
	return res
}

// reportDropped records a log entry and a Warning event for each resource
// that the cleanup skips.
func (c *ControllerCleaner) reportDropped(controller client.Object, dropped []schema.GroupResource) {
	for _, gr := range dropped {
		c.Logger.Info("Skip the parent finalizer cleanup of a resource that is not in discovery", "name", controller.GetName(), "resource", gr.String())
		c.EventRecorder.Eventf(controller, corev1.EventTypeWarning, events.ReasonSyncError,
			"[%s] skip the cleanup of resource %s: discovery does not list it and no CRD defines it. If its objects exist, remove the finalizer by hand",
			controller.GetName(), gr.String())
	}
}

// resolveEntry finds a served version of the resource of the entry. It tries
// the recorded version first, then each other version that discovery lists.
// An object has the same finalizers in all versions.
//
// If discovery does not list the resource, resolveEntry asks for the CRD:
//   - The CRD exists: return an error. Discovery is a cache and can list the
//     resource later.
//   - No CRD, and discovery lists the apiVersion: drop the entry. The API
//     exists and does not have the resource.
//   - No CRD, and the APIService of the apiVersion exists: return an error.
//     An aggregated API can be down for a short time, and its objects stay
//     in its server.
//   - No CRD and no APIService: drop the entry. Kubernetes deletes all
//     objects of a CRD before it deletes the CRD. For an aggregated API whose
//     APIService was deleted, a person must remove the finalizer by hand.
func (c *ControllerCleaner) resolveEntry(ctx context.Context, parent ParentResource) (*ParentResource, bool, error) {
	if _, err := c.DynClient.Resource(parent.APIVersion, parent.Resource); err == nil {
		return &parent, false, nil
	}
	gv, err := schema.ParseGroupVersion(parent.APIVersion)
	if err != nil {
		// No controller can start with this apiVersion, so no parent object
		// has the finalizer.
		return nil, true, nil //nolint:nilerr // an invalid apiVersion has no objects
	}
	if versions := c.Resources.APIVersionsOf(gv.Group, parent.Resource); len(versions) > 0 {
		return &ParentResource{APIVersion: versions[0], Resource: parent.Resource}, false, nil
	}
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGVK)
	err = c.K8sClient.Get(ctx, client.ObjectKey{Name: parent.Resource + "." + gv.Group}, crd)
	switch {
	case err == nil:
		return nil, false, fmt.Errorf("the CRD of resource %s exists, but discovery lists no version of it", parent.Resource)
	case !apierrors.IsNotFound(err):
		return nil, false, fmt.Errorf("can't get the CRD of resource %s: %w", parent.Resource, err)
	}
	if c.Resources.HasGroupVersion(parent.APIVersion) {
		return nil, true, nil
	}
	apiService := &unstructured.Unstructured{}
	apiService.SetGroupVersionKind(apiServiceGVK)
	err = c.K8sClient.Get(ctx, client.ObjectKey{Name: gv.Version + "." + gv.Group}, apiService)
	switch {
	case apierrors.IsNotFound(err):
		return nil, true, nil
	case err != nil:
		return nil, false, fmt.Errorf("can't get the APIService of apiVersion %s: %w", parent.APIVersion, err)
	default:
		return nil, false, fmt.Errorf("apiVersion %s is registered, but it is not in discovery", parent.APIVersion)
	}
}

// removeFromResolved runs one slice of the cleanup over the resolved
// resources. It returns the resources that became complete, and done == true
// when one full pass completed all resources. An empty set is done.
func (c *ControllerCleaner) removeFromResolved(ctx context.Context, controller client.Object, parentFinalizer string, resolved []ParentResource) ([]schema.GroupResource, bool, error) {
	if len(resolved) == 0 {
		return nil, true, nil
	}
	cursor := c.Cursors.get(controller)
	removed, completed, done, err := NewManager(parentFinalizer, true).RemoveFromResources(ctx, c.DynClient, resolved, c.budget(), cursor)
	if removed > 0 || done {
		c.Logger.Info("Removed the parent finalizer from parent objects", "name", controller.GetName(), "removed", removed, "done", done)
	}
	return completed, done, err
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
	// ResourceRetry is true when an object of the resource in Parent needs
	// another update. The resource is then not complete in this pass.
	ResourceRetry bool
	// Fingerprint identifies the GroupResource set of the pass. If the set
	// changes, RemoveFromResources starts a new pass. A resume in a different
	// set can skip a resource.
	Fingerprint string
}

// newPass resets the position and keeps the fingerprint.
func (c *Cursor) newPass() {
	*c = Cursor{Fingerprint: c.Fingerprint}
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
// and then keeps its position in the cursor. When the GroupResource set of
// parents is not the set of the cursor, it starts a new pass. Each list call
// and each update call has a time limit of cleanupCallTimeout.
//
// It returns the number of objects that it changed, and the resources that
// became complete in this call. A resource is complete when the pass read all
// its pages and updated each object that had the finalizer, with no conflict
// and no error. It returns done == true only when a full pass completed all
// resources. It returns done == false and no error when the cleanup is not
// complete. Call it again with the same cursor in this condition.
//
// It returns an error for a resource that is not in discovery, because
// objects of that resource can exist.
func (m *Manager) RemoveFromResources(ctx context.Context, dynClient *dynamicclientset.Clientset, parents []ParentResource, budget time.Duration, cursor *Cursor) (removed int, completed []schema.GroupResource, done bool, err error) {
	if !dynClient.HasSynced() {
		return 0, nil, false, errors.New("discovery is not synced")
	}
	// Remove duplicate parents and sort them. The cursor finds its parent by
	// value, and a pass resumes at the position of that parent. With a
	// duplicate or with a different order, the pass can do a resource twice
	// or skip one.
	parents = sortedParents(parents)
	if fp := fingerprint(parents); cursor.Fingerprint != fp {
		*cursor = Cursor{Fingerprint: fp}
	}
	start := slices.Index(parents, cursor.Parent)
	if start < 0 {
		// The cursor is not in this set. Start a new pass.
		cursor.newPass()
		start = 0
	}
	clock := &cleanupClock{deadline: time.Now().Add(budget)}
	var errs []error
	for _, parent := range parents[start:] {
		if cursor.Parent != parent {
			cursor.Parent, cursor.Continue, cursor.Skip, cursor.ResourceRetry = parent, "", 0, false
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
			return removed, completed, false, errors.Join(errs...)
		}
		if !cursor.ResourceRetry {
			completed = append(completed, groupResourceOf(parent))
		}
	}
	done = !cursor.Retry
	cursor.newPass()
	return removed, completed, done, errors.Join(errs...)
}

// uniqueParents returns the parents without duplicates, in the same order.
// Two parents with the same GroupResource are duplicates. They name the same
// stored objects.
func uniqueParents(parents []ParentResource) []ParentResource {
	seen := make(map[schema.GroupResource]bool, len(parents))
	unique := make([]ParentResource, 0, len(parents))
	for _, parent := range parents {
		if gr := groupResourceOf(parent); !seen[gr] {
			seen[gr] = true
			unique = append(unique, parent)
		}
	}
	return unique
}

// sortedParents returns the parents without duplicates, sorted by
// GroupResource.
func sortedParents(parents []ParentResource) []ParentResource {
	sorted := uniqueParents(parents)
	slices.SortFunc(sorted, func(a, b ParentResource) int {
		ga, gb := groupResourceOf(a), groupResourceOf(b)
		return cmp.Or(strings.Compare(ga.Group, gb.Group), strings.Compare(ga.Resource, gb.Resource))
	})
	return sorted
}

// removeFromAll removes the finalizer from the objects of the resource of the
// given client. It starts at the page of the cursor and moves the cursor to
// the next page. It returns complete == true after it read the last page, or
// after a list error. A list error sets the retry fields of the cursor.
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
			// Go to the next resource, so that one resource with an error does
			// not stop the others. The next pass lists this resource again.
			cursor.Retry, cursor.ResourceRetry = true, true
			errs = append(errs, fmt.Errorf("can't list %s: %w", client.Name, listErr))
			return removed, true, errors.Join(errs...)
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
				// NotFound can mean that the object is deleted. It can also
				// mean that the version is not served. Retry. The next pass
				// lists the resource again.
				cursor.Retry, cursor.ResourceRetry = true, true
			case apierrors.IsConflict(updateErr):
				// The object changed after the list call. The next pass does
				// this object again.
				cursor.Retry, cursor.ResourceRetry = true, true
			default:
				cursor.Retry, cursor.ResourceRetry = true, true
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
