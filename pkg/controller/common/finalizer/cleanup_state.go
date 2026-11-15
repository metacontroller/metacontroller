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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"metacontroller/pkg/events"
)

// CleanupRecordAnnotation is the annotation that holds the cleanup record on
// a controller object.
const CleanupRecordAnnotation = "metacontroller.io/parent-finalizer-cleanup-state"

const (
	cleanupStateVersion = 1

	// cleanupRequeueDelay is the requeue delay while the cleanup has more
	// work.
	cleanupRequeueDelay = time.Second
	// cleanupErrorDelay is the requeue delay after a cleanup error.
	cleanupErrorDelay = 30 * time.Second
)

// ErrInvalidCleanupRecord means that Metacontroller cannot use the record.
// The JSON is malformed, the version is unknown, or an entry is not valid. A
// person must repair the annotation.
var ErrInvalidCleanupRecord = errors.New("invalid cleanup record")

// HasCleanupRecord reports whether the controller object has a cleanup
// record annotation that is not empty, valid or not.
func HasCleanupRecord(controller client.Object) bool {
	raw, ok := controller.GetAnnotations()[CleanupRecordAnnotation]
	return ok && raw != ""
}

// KeepRequeue returns the result and the error of a reconcile.
// controller-runtime ignores RequeueAfter when the error is not nil, and uses
// its rate limiter instead. That delay can grow to many minutes. So, when the
// result has a RequeueAfter, KeepRequeue logs the error and returns a nil
// error. The caller must record the error as an event before it calls
// KeepRequeue.
func KeepRequeue(logger logr.Logger, result reconcile.Result, err error) (reconcile.Result, error) {
	if err != nil && result.RequeueAfter > 0 {
		logger.Error(err, "Reconcile error, requeue after the cleanup delay", "requeueAfter", result.RequeueAfter)
		return result, nil
	}
	return result, err
}

// cleanupState is the record of the parent resources of one controller. It is
// stored in an annotation on the controller object.
type cleanupState struct {
	Version int `json:"version"`
	// Active lists the parent resources of the spec of the running
	// controller.
	Active []ParentResource `json:"active"`
	// RemoveParentFinalizerFrom lists the parent resources that left the
	// spec. Their objects can still have the parent finalizer.
	RemoveParentFinalizerFrom []ParentResource `json:"removeParentFinalizerFrom"`
}

// groupResourceOf returns the identity of a parent resource: the API group and
// the resource name, without the version.
func groupResourceOf(p ParentResource) schema.GroupResource {
	gv, err := schema.ParseGroupVersion(p.APIVersion)
	if err != nil {
		return schema.GroupResource{Group: p.APIVersion, Resource: p.Resource}
	}
	return schema.GroupResource{Group: gv.Group, Resource: p.Resource}
}

// readCleanupState returns the record. It returns nil if the annotation is
// absent, and an error if the record is not valid.
func readCleanupState(controller client.Object) (*cleanupState, error) {
	raw, ok := controller.GetAnnotations()[CleanupRecordAnnotation]
	if !ok || raw == "" {
		return nil, nil
	}
	var state cleanupState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("%w: annotation %s: %w", ErrInvalidCleanupRecord, CleanupRecordAnnotation, err)
	}
	if state.Version != cleanupStateVersion {
		return nil, fmt.Errorf("%w: annotation %s has unsupported version %d", ErrInvalidCleanupRecord, CleanupRecordAnnotation, state.Version)
	}
	for _, r := range slices.Concat(state.Active, state.RemoveParentFinalizerFrom) {
		if r.APIVersion == "" || r.Resource == "" {
			return nil, fmt.Errorf("%w: annotation %s has an entry without apiVersion or resource", ErrInvalidCleanupRecord, CleanupRecordAnnotation)
		}
	}
	return &state, nil
}

// writeCleanupState sorts the record, removes duplicate entries, and writes
// the record. It writes nothing when the value already matches. The write
// uses an optimistic lock.
func writeCleanupState(ctx context.Context, c client.Client, controller client.Object, state *cleanupState) error {
	normalizeCleanupState(state)
	value, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := checkAnnotationSize(controller, len(value)); err != nil {
		return err
	}
	if existing, ok := controller.GetAnnotations()[CleanupRecordAnnotation]; ok && existing == string(value) {
		return nil
	}
	orig, ok := controller.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("can't copy %T", controller)
	}
	annotations := controller.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[CleanupRecordAnnotation] = string(value)
	controller.SetAnnotations(annotations)
	return c.Patch(ctx, controller, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{}))
}

func normalizeCleanupState(state *cleanupState) {
	state.Active = sortedParents(state.Active)
	state.RemoveParentFinalizerFrom = sortedParents(state.RemoveParentFinalizerFrom)
}

// checkAnnotationSize returns an error if the write makes all annotations of
// the object larger than the Kubernetes size limit.
func checkAnnotationSize(controller client.Object, valueLen int) error {
	total := 0
	for k, v := range controller.GetAnnotations() {
		if k != CleanupRecordAnnotation {
			total += len(k) + len(v)
		}
	}
	total += len(CleanupRecordAnnotation) + valueLen
	if total > validation.TotalAnnotationSizeLimitB {
		return fmt.Errorf("can't write the cleanup record: annotations size %d is larger than limit %d", total, validation.TotalAnnotationSizeLimitB)
	}
	return nil
}

// removeCleanupStateAndFinalizer removes the record and the controller
// finalizer in one patch, so that the API server can delete the object.
func removeCleanupStateAndFinalizer(ctx context.Context, c client.Client, controller client.Object) error {
	orig, ok := controller.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("can't copy %T", controller)
	}
	annotations := controller.GetAnnotations()
	delete(annotations, CleanupRecordAnnotation)
	if len(annotations) == 0 {
		annotations = nil
	}
	controller.SetAnnotations(annotations)
	controllerutil.RemoveFinalizer(controller, ControllerFinalizer)
	return client.IgnoreNotFound(c.Patch(ctx, controller, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{})))
}

func groupResourceSet(parents []ParentResource) map[schema.GroupResource]bool {
	set := make(map[schema.GroupResource]bool, len(parents))
	for _, p := range parents {
		set[groupResourceOf(p)] = true
	}
	return set
}

// fingerprint identifies the GroupResource set of sorted parents.
func fingerprint(parents []ParentResource) string {
	keys := make([]string, 0, len(parents))
	for _, p := range parents {
		keys = append(keys, groupResourceOf(p).String())
	}
	return strings.Join(keys, ";")
}

// RecordActiveParents writes the record with active as the active resources.
// It moves the resources that left the spec into removeParentFinalizerFrom.
// It removes from that list the resources that are active again.
//
// running lists the parent resources of the running controller, or is nil.
// They count as old active resources, also when the record does not list
// them.
//
// RecordActiveParents does not start or stop a controller. The caller decides
// what to do with an error. The error wraps ErrInvalidCleanupRecord if a
// person must repair the annotation.
func (c *ControllerCleaner) RecordActiveParents(ctx context.Context, controller client.Object, running, active []ParentResource) error {
	err := c.recordActiveParents(ctx, controller, running, active)
	if err != nil {
		c.EventRecorder.Eventf(controller, corev1.EventTypeWarning, events.ReasonSyncError, "[%s] can't record the cleanup state - %s", controller.GetName(), err)
	}
	return err
}

func (c *ControllerCleaner) recordActiveParents(ctx context.Context, controller client.Object, running, active []ParentResource) error {
	old, err := readCleanupState(controller)
	if err != nil {
		return err
	}
	if old == nil {
		old = &cleanupState{}
	}
	activeSet := groupResourceSet(active)
	var retired []ParentResource
	// The running controller can use a resource that the record does not
	// list, for example after a person edits or removes the annotation.
	for _, r := range slices.Concat(old.RemoveParentFinalizerFrom, old.Active, running) {
		if !activeSet[groupResourceOf(r)] {
			retired = append(retired, r)
		}
	}
	return writeCleanupState(ctx, c.K8sClient, controller, &cleanupState{
		Version:                   cleanupStateVersion,
		Active:                    active,
		RemoveParentFinalizerFrom: retired,
	})
}

// CleanupRetiredParents removes the parent finalizer from the objects of the
// resources in removeParentFinalizerFrom, one slice per call. The Budget of
// the cleaner limits the time of a slice. It skips each resource in exclude,
// because the spec or the running controller still uses it.
//
// It removes an entry from the list when a pass completed the resource, or
// when the resource has no objects to clean. An entry with an error stays in
// the list and does not stop the other entries. It returns a requeue delay
// while work remains. Call it on each reconcile, also when the spec is
// unchanged.
//
// It records an entry error as an event and does not return it. Then the
// requeue delay is cleanupErrorDelay, not the delay of the rate limiter of
// the queue. It returns an error only when it cannot read or write the
// record.
func (c *ControllerCleaner) CleanupRetiredParents(ctx context.Context, controller client.Object, parentFinalizer string, exclude []ParentResource) (reconcile.Result, error) {
	state, err := readCleanupState(controller)
	if err != nil {
		c.reportCleanupError(controller, err)
		return reconcile.Result{}, err
	}
	excluded := groupResourceSet(exclude)
	var entries []ParentResource
	if state != nil {
		for _, r := range state.RemoveParentFinalizerFrom {
			if !excluded[groupResourceOf(r)] {
				entries = append(entries, r)
			}
		}
	}
	if len(entries) == 0 {
		c.Cursors.delete(controller)
		return reconcile.Result{}, nil
	}

	res := c.resolveEntries(ctx, entries)
	c.reportDropped(controller, res.dropped)
	completed, done, scanErr := c.removeFromResolved(ctx, controller, parentFinalizer, res.resolved)
	entryErr := errors.Join(append(res.errs, scanErr)...)
	if entryErr != nil {
		c.reportCleanupError(controller, entryErr)
	}

	finished := map[schema.GroupResource]bool{}
	for _, gr := range res.dropped {
		finished[gr] = true
	}
	for _, gr := range completed {
		finished[gr] = true
	}
	if done {
		for _, p := range res.resolved {
			finished[groupResourceOf(p)] = true
		}
	}
	var remaining []ParentResource
	for _, r := range state.RemoveParentFinalizerFrom {
		if !finished[groupResourceOf(r)] {
			remaining = append(remaining, r)
		}
	}

	requeue := reconcile.Result{RequeueAfter: cleanupRequeueDelay}
	if entryErr != nil {
		requeue.RequeueAfter = cleanupErrorDelay
	}
	if len(remaining) < len(state.RemoveParentFinalizerFrom) {
		err := writeCleanupState(ctx, c.K8sClient, controller, &cleanupState{
			Version:                   cleanupStateVersion,
			Active:                    state.Active,
			RemoveParentFinalizerFrom: remaining,
		})
		if apierrors.IsConflict(err) {
			// A conflict means that the record changed. Keep the entries.
			// The next reconcile reads the new record.
			return requeue, nil
		}
		if err != nil {
			c.reportCleanupError(controller, err)
			return requeue, err
		}
	}
	for _, r := range remaining {
		if !excluded[groupResourceOf(r)] {
			return requeue, nil
		}
	}
	c.Cursors.delete(controller)
	return reconcile.Result{}, nil
}

// reportCleanupError records a cleanup error as a log entry and an event.
func (c *ControllerCleaner) reportCleanupError(controller client.Object, err error) {
	c.Logger.Error(err, "Can't remove the parent finalizer", "name", controller.GetName())
	c.EventRecorder.Eventf(controller, corev1.EventTypeWarning, events.ReasonSyncError, "[%s] can't remove the parent finalizer - %s", controller.GetName(), err)
}
