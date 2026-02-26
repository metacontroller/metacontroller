/*
Copyright 2018 Google Inc.

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

package common

import (
	"context"
	"encoding/json"
	"fmt"
	commonv2 "metacontroller/pkg/controller/common/api/v2"
	"metacontroller/pkg/logging"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
	"k8s.io/utils/ptr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	dynamicapply "metacontroller/pkg/dynamic/apply"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func ApplyUpdate(orig, update *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// The controller only returns a partial object.
	// We compute the full updated object in the style of "kubectl apply".
	lastApplied, err := dynamicapply.GetLastApplied(orig)
	if err != nil {
		return nil, err
	}

	// prevent setting last applied values in the new object
	nullifyLastAppliedAnnotation(update)

	newObj := &unstructured.Unstructured{}
	newObj.Object, err = dynamicapply.Merge(orig.UnstructuredContent(), lastApplied, update.UnstructuredContent())
	if err != nil {
		return nil, err
	}
	// Revert metadata fields that are known to be read-only, system fields,
	// so that attempts to change those fields will never cause a diff to be found
	// by DeepEqual, which would cause needless, no-op updates or recreates.
	// See: https://github.com/GoogleCloudPlatform/metacontroller/issues/76
	if err := revertObjectMetaSystemFields(newObj, orig); err != nil {
		return nil, fmt.Errorf("failed to revert ObjectMeta system fields: %w", err)
	}
	// Revert status because we don't currently support a parent changing status of
	// its children, so we need to ensure no diffs on the children involve status.
	if err := revertField(newObj, orig, "status"); err != nil {
		return nil, fmt.Errorf("failed to revert .status: %w", err)
	}

	if err = dynamicapply.SetLastApplied(newObj, update.UnstructuredContent()); err != nil {
		logging.Logger.Error(err, "failed to set lastApplied")
	}
	return newObj, nil
}

// objectMetaSystemFields is a list of JSON field names within ObjectMeta that
// are both read-only and system-populated according to the comments in
// k8s.io/apimachinery/pkg/apis/meta/v1/types.go.
var objectMetaSystemFields = []string{
	"selfLink",
	"uid",
	"resourceVersion",
	"generation",
	"creationTimestamp",
	"deletionTimestamp",
	"deletionGracePeriodSeconds",
}

// revertObjectMetaSystemFields overwrites the read-only, system-populated
// fields of ObjectMeta in newObj to match what they were in orig.
// If the field existed before, we create it if necessary and set the value.
// If the field was unset before, we delete it if necessary.
func revertObjectMetaSystemFields(newObj, orig *unstructured.Unstructured) error {
	for _, fieldName := range objectMetaSystemFields {
		if err := revertField(newObj, orig, "metadata", fieldName); err != nil {
			return err
		}
	}
	return nil
}

// revertField reverts field in newObj to match what it was in orig.
func revertField(newObj, orig *unstructured.Unstructured, fieldPath ...string) error {
	field, found, err := unstructured.NestedFieldNoCopy(orig.UnstructuredContent(), fieldPath...)
	if err != nil {
		return fmt.Errorf("can't traverse UnstructuredContent to look for field %v: %w", fieldPath, err)
	}
	if found {
		// The original had this field set, so make sure it remains the same.
		// SetNestedField will recursively ensure the field and all its parent
		// fields exist, and then set the value.
		if err := unstructured.SetNestedField(newObj.UnstructuredContent(), field, fieldPath...); err != nil {
			return fmt.Errorf("can't revert field %v: %w", fieldPath, err)
		}
	} else {
		// The original had this field unset, so make sure it remains unset.
		// RemoveNestedField is a no-op if the field or any of its parents
		// don't exist.
		unstructured.RemoveNestedField(newObj.UnstructuredContent(), fieldPath...)
	}
	return nil
}

func MakeControllerRef(parent *unstructured.Unstructured) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion:         parent.GetAPIVersion(),
		Kind:               parent.GetKind(),
		Name:               parent.GetName(),
		UID:                parent.GetUID(),
		Controller:         ptr.To[bool](true),
		BlockOwnerDeletion: ptr.To[bool](true),
	}
}

type ChildUpdateStrategy interface {
	GetMethod(apiGroup, kind string) v1alpha1.ChildUpdateMethod
}

func ManageChildren(
	dynClient *dynamicclientset.Clientset,
	updateStrategy ChildUpdateStrategy,
	parent *unstructured.Unstructured,
	observedChildren, desiredChildren commonv2.UniformObjectMap, ssaOptions *ApplyOptions) error {
	// If some operations fail, keep trying others so, for example,
	// we don't block recovery (create new Pod) on a failed delete.
	var errs []error

	// Delete observed, owned objects that are not desired.
	for key, objects := range observedChildren {
		client, err := dynClient.Kind(key.GroupVersion().String(), key.Kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if err := deleteChildren(client, parent, objects, desiredChildren[key]); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	// Create or update desired objects.
	for key, objects := range desiredChildren {
		client, err := dynClient.Kind(key.GroupVersion().String(), key.Kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := updateChildren(client, updateStrategy, parent, observedChildren[key], objects, ssaOptions); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

func deleteChildren(client *dynamicclientset.ResourceClient, parent *unstructured.Unstructured, observed, desired map[string]*unstructured.Unstructured) error {
	var errs []error
	for name, obj := range observed {
		if obj.GetDeletionTimestamp() != nil {
			// Skip objects that are already pending deletion.
			continue
		}

		if desired == nil || desired[name] == nil {
			// This observed object wasn't listed as desired.
			logging.Logger.Info("Deleting child", "parent", parent, "child", obj)
			uid := obj.GetUID()
			// Explicitly request deletion propagation, which is what users expect,
			// since some objects default to orphaning for backwards compatibility.
			propagation := metav1.DeletePropagationBackground
			err := client.Namespace(obj.GetNamespace()).Delete(
				context.TODO(),
				obj.GetName(),
				metav1.DeleteOptions{
					Preconditions:     &metav1.Preconditions{UID: &uid},
					PropagationPolicy: &propagation,
				},
			)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Swallow the error since there's no point retrying if the child is gone.
					logging.Logger.Info("Failed to delete child, child object has been deleted", "parent", parent, "child", obj)
				} else {
					errs = append(errs, fmt.Errorf("can't delete %v: %w", describeObject(obj), err))
				}
				continue
			}

			lastUpdateName := lastUpdateCacheKey(client, obj)
			cacheLock.Lock()
			delete(lastUpdatedCache, lastUpdateName)
			cacheLock.Unlock()
		}
	}
	return utilerrors.NewAggregate(errs)
}

type lastUpdate struct {
	hash               uint64
	resourcegeneration int64
}

var (
	lastUpdatedCache = make(map[string]*lastUpdate)
	cacheLock        = &sync.RWMutex{}
)

func lastUpdateCacheKey(client *dynamicclientset.ResourceClient, obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s/%s", client.Group, client.Kind, obj.GetNamespace(), obj.GetName())
}

type ApplyOperations struct {
	client         *dynamicclientset.ResourceClient
	updateStrategy ChildUpdateStrategy

	parent *unstructured.Unstructured

	observed *unstructured.Unstructured
	desired  *unstructured.Unstructured

	ssaOptions *ApplyOptions
}

var operationHandlers = map[string]func(*ApplyOperations) error{
	string(ApplyStrategyServerSideApply): updateChildrenWithServerSideApply,
	string(ApplyStrategyDynamicApply):    updateChildrenWithDynamicApply,
}

func updateChildren(client *dynamicclientset.ResourceClient, updateStrategy ChildUpdateStrategy, parent *unstructured.Unstructured, observed, desired map[string]*unstructured.Unstructured, ssaOptions *ApplyOptions) error {
	var errs []error

	for name, obj := range desired {
		operation := &ApplyOperations{
			client:         client,
			updateStrategy: updateStrategy,
			parent:         parent,
			observed:       observed[name],
			desired:        obj,
			ssaOptions:     ssaOptions,
		}

		if handler, ok := operationHandlers[string(ssaOptions.Strategy)]; ok {
			if err := handler(operation); err != nil {
				errs = append(errs, err)
			}
		} else {
			errs = append(errs, fmt.Errorf("invalid apply strategy: unknown strategy %q", ssaOptions.Strategy))
		}
	}
	return utilerrors.NewAggregate(errs)
}

func updateChildrenWithServerSideApply(operation *ApplyOperations) error {
	// We always claim everything we create/update.
	controllerRef := MakeControllerRef(operation.parent)
	ownerRefs := operation.desired.GetOwnerReferences()
	hasControllerRef := false
	for _, ref := range ownerRefs {
		if ref.UID == controllerRef.UID {
			hasControllerRef = true
			break
		}
	}
	if !hasControllerRef {
		ownerRefs = append(ownerRefs, *controllerRef)
		operation.desired.SetOwnerReferences(ownerRefs)
	}

	// Remove metadata fields that are known to be read-only, system fields,
	// so that they don't cause cache misses or SSA conflicts.
	for _, fieldName := range objectMetaSystemFields {
		unstructured.RemoveNestedField(operation.desired.Object, "metadata", fieldName)
	}
	// Remove status because we don't currently support a parent changing status of
	// its children.
	unstructured.RemoveNestedField(operation.desired.Object, "status")

	// prevent setting last applied values in the new object
	nullifyLastAppliedAnnotation(operation.desired)

	data, err := json.Marshal(operation.desired)
	if err != nil {
		return err
	}

	hash := xxhash.Sum64(data)
	lastUpdateCacheName := lastUpdateCacheKey(operation.client, operation.desired)
	if oldObj := operation.observed; oldObj != nil {
		if oldObj.GetDeletionTimestamp() != nil {
			logging.Logger.Info("Not updating", "parent", operation.parent, "child", operation.desired, "reason", "Pending deletion of child object")
			return nil
		}

		cacheLock.RLock()
		lastUpdated, ok := lastUpdatedCache[lastUpdateCacheName]
		cacheLock.RUnlock()

		if ok && lastUpdated.hash == hash && lastUpdated.resourcegeneration == oldObj.GetGeneration() {
			logging.Logger.Info("Skipping update, no changes detected", "name", lastUpdateCacheName)
			return nil
		}

		if ok {
			logging.Logger.Info("Detected changes, updating", "name", lastUpdateCacheName)
		} else {
			logging.Logger.Info("No cache entry found, updating", "name", lastUpdateCacheName)
		}

		// Check the update strategy for this child kind.
		switch method := operation.updateStrategy.GetMethod(operation.client.Group, operation.client.Kind); method {
		case v1alpha1.ChildUpdateOnDelete, "":
			// This means we don't try to update anything unless it gets deleted
			// by someone else (we won't delete it ourselves).
			logging.Logger.V(5).Info("Not updating", "parent", operation.parent, "child", operation.desired, "reason", "OnDelete update strategy selected")
			return nil
		case v1alpha1.ChildUpdateRecreate, v1alpha1.ChildUpdateRollingRecreate:
			// Delete the object (now) and recreate it (on the next sync).
			logging.Logger.Info("Deleting for update", "parent", operation.parent, "child", operation.desired, "reason", "Recreate update strategy selected")
			uid := oldObj.GetUID()
			// Explicitly request deletion propagation, which is what users expect,
			// since some objects default to orphaning for backwards compatibility.
			propagation := metav1.DeletePropagationBackground
			err := operation.client.Namespace(operation.desired.GetNamespace()).Delete(
				context.TODO(),
				operation.desired.GetName(),
				metav1.DeleteOptions{
					Preconditions:     &metav1.Preconditions{UID: &uid},
					PropagationPolicy: &propagation,
				},
			)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Swallow the error since there's no point retrying if the child is gone.
					logging.Logger.Info("Failed to delete child, child object has been deleted", "parent", operation.parent, "child", operation.desired)
				} else {
					return err
				}
			}

			return nil // skip the rest of the function, since we'll recreate the object on the next sync
		case v1alpha1.ChildUpdateInPlace, v1alpha1.ChildUpdateRollingInPlace:
			// check if observed object hast last applied annotation
			_, hasLastApplied := oldObj.GetAnnotations()[dynamicapply.LastAppliedAnnotation]
			if hasLastApplied {
				// if observed object has has last applied annotation we need to remove it
				// from the remote object to avoid conflicts
				annotationNameForJsonPatch := strings.ReplaceAll(strings.ReplaceAll(dynamicapply.LastAppliedAnnotation, "~", "~0"), "/", "~1")
				patch := fmt.Sprintf(`[{"op": "remove", "path": "/metadata/annotations/%s"}]`, annotationNameForJsonPatch)
				_, err := operation.client.Namespace(operation.desired.GetNamespace()).Patch(context.TODO(), operation.desired.GetName(), types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						// Swallow the error since there's no point retrying if the child is gone.
						logging.Logger.Info("Failed to remove last applied annotation, child object has been deleted", "parent", operation.parent, "child", operation.desired)
					} else {
						logging.Logger.Error(err, "Failed to remove last applied annotation from observed object", "parent", operation.parent, "child", operation.desired)
						return err
					}
					return nil
				}
			}
		default:
			return fmt.Errorf("invalid update strategy for %v: unknown method %q", operation.client.Kind, method)
		}
	}

	// create or update the object using server-side apply
	patched, err := operation.client.Namespace(operation.desired.GetNamespace()).Patch(context.TODO(), operation.desired.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: operation.ssaOptions.FieldManager,
		Force:        ptr.To(true),
	})

	if err != nil {
		switch {
		case apierrors.IsConflict(err):
			// it is possible that the object was modified after this sync was started, ignore conflict since we will reconcile again
			logging.Logger.Info("Failed to apply server-side apply due to outdated resourceVersion", "parent", operation.parent, "child", operation.desired)
		default:
			logging.Logger.Error(err, "Failed to apply server-side apply", "parent", operation.parent, "child", operation.desired)
			return err
		}
		return nil
	}

	cacheLock.Lock()
	lastUpdatedCache[lastUpdateCacheName] = &lastUpdate{
		hash:               hash,
		resourcegeneration: patched.GetGeneration(),
	}

	logging.Logger.Info("Cache updated", "name", lastUpdateCacheName)

	cacheLock.Unlock()
	return nil
}

func updateChildrenWithDynamicApply(operation *ApplyOperations) error {
	if oldObj := operation.observed; oldObj != nil {
		// Update
		newObj, err := ApplyUpdate(oldObj, operation.desired)
		if err != nil {
			return err
		}

		// Attempt an update, if the 3-way merge resulted in any changes.
		if DeepEqual(newObj.UnstructuredContent(), oldObj.UnstructuredContent()) {
			// Nothing changed.
			return nil
		}

		if logging.Logger.V(5).Enabled() {
			mergePatch, err := JsonMergePatch(oldObj, newObj)
			if err != nil {
				logging.Logger.V(5).Error(err, "Cannot create merge patch to visualize diff")
			} else {
				rawMergePatch := json.RawMessage(mergePatch)
				logging.Logger.V(5).Info("Diff between observed and desired", "mergePatch", rawMergePatch)
			}
		}

		// Leave it alone if it's pending deletion.
		if oldObj.GetDeletionTimestamp() != nil {
			logging.Logger.Info("Not updating", "parent", operation.parent, "child", operation.desired, "reason", "Pending deletion of child object")
			return nil
		}

		// Check the update strategy for this child kind.
		switch method := operation.updateStrategy.GetMethod(operation.client.Group, operation.client.Kind); method {
		case v1alpha1.ChildUpdateOnDelete, "":
			// This means we don't try to update anything unless it gets deleted
			// by someone else (we won't delete it ourselves).
			logging.Logger.V(5).Info("Not updating", "parent", operation.parent, "child", operation.desired, "reason", "OnDelete update strategy selected")
			return nil
		case v1alpha1.ChildUpdateRecreate, v1alpha1.ChildUpdateRollingRecreate:
			// Delete the object (now) and recreate it (on the next sync).
			logging.Logger.Info("Deleting for update", "parent", operation.parent, "child", operation.desired, "reason", "Recreate update strategy selected")
			uid := oldObj.GetUID()
			// Explicitly request deletion propagation, which is what users expect,
			// since some objects default to orphaning for backwards compatibility.
			propagation := metav1.DeletePropagationBackground
			err := operation.client.Namespace(operation.desired.GetNamespace()).Delete(
				context.TODO(),
				operation.desired.GetName(),
				metav1.DeleteOptions{
					Preconditions:     &metav1.Preconditions{UID: &uid},
					PropagationPolicy: &propagation,
				},
			)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Swallow the error since there's no point retrying if the child is gone.
					logging.Logger.Info("Failed to delete child, child object has been deleted", "parent", operation.parent, "child", operation.desired)
				} else {
					return err
				}
				return nil
			}
		case v1alpha1.ChildUpdateInPlace, v1alpha1.ChildUpdateRollingInPlace:
			// Update the object in-place.
			logging.Logger.Info("Updating", "parent", operation.parent, "child", operation.desired, "reason", "InPlace update strategy selected")
			if _, err := operation.client.Namespace(operation.desired.GetNamespace()).Update(context.TODO(), newObj, metav1.UpdateOptions{}); err != nil {
				switch {
				case apierrors.IsNotFound(err):
					// Swallow the error since there's no point retrying if the child is gone.
					logging.Logger.Info("Failed to update child, child object has been deleted", "parent", operation.parent, "child", operation.desired)
				case apierrors.IsConflict(err):
					// it is possible that the object was modified after this sync was started, ignore conflict since we will reconcile again
					logging.Logger.Info("Failed to update child due to outdated resourceVersion", "parent", operation.parent, "child", operation.desired)
				default:
					return err
				}
				return nil
			}
		default:
			return fmt.Errorf("invalid update strategy for %v: unknown method %q", operation.client.Kind, method)

		}
	} else {
		// Create
		logging.Logger.Info("Creating", "parent", operation.parent, "child", operation.desired)

		// The controller should return a partial object containing only the
		// fields it cares about. We save this partial object so we can do
		// a 3-way merge upon update, in the style of "kubectl apply".
		//
		// Make sure this happens before we add anything else to the object.
		if err := dynamicapply.SetLastApplied(operation.desired, operation.desired.UnstructuredContent()); err != nil {
			return err
		}

		// We always claim everything we create.
		controllerRef := MakeControllerRef(operation.parent)
		ownerRefs := operation.desired.GetOwnerReferences()
		ownerRefs = append(ownerRefs, *controllerRef)
		operation.desired.SetOwnerReferences(ownerRefs)

		if _, err := operation.client.Namespace(operation.desired.GetNamespace()).Create(context.TODO(), operation.desired, metav1.CreateOptions{}); err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Swallow the error since there's no point retrying if the child already exists
				logging.Logger.Info("Failed to create child, child object already exists", "parent", operation.parent, "child", operation.desired)
			} else {
				return err
			}
		}
	}

	return nil
}
