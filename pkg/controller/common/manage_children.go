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
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"

	"k8s.io/utils/pointer"

	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	dynamicapply "metacontroller/pkg/dynamic/apply"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
)

func ApplyUpdate(orig, update *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// The controller only returns a partial object.
	// We compute the full updated object in the style of "kubectl apply".
	lastApplied, err := dynamicapply.GetLastApplied(orig)
	if err != nil {
		return nil, err
	}
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
		klog.ErrorS(err, "failed to set lastApplied")
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
		Controller:         pointer.BoolPtr(true),
		BlockOwnerDeletion: pointer.BoolPtr(true),
	}
}

type ChildUpdateStrategy interface {
	GetMethod(apiGroup, kind string) v1alpha1.ChildUpdateMethod
}

func ManageChildren(dynClient *dynamicclientset.Clientset, updateStrategy ChildUpdateStrategy, parent *unstructured.Unstructured, observedChildren, desiredChildren ChildMap) error {
	// If some operations fail, keep trying others so, for example,
	// we don't block recovery (create new Pod) on a failed delete.
	var errs []error

	// Delete observed, owned objects that are not desired.
	for key, objects := range observedChildren {
		apiVersion, kind := ParseChildMapKey(key)
		client, err := dynClient.Kind(apiVersion, kind)
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
		apiVersion, kind := ParseChildMapKey(key)
		client, err := dynClient.Kind(apiVersion, kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := updateChildren(client, updateStrategy, parent, observedChildren[key], objects); err != nil {
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
			klog.InfoS("Deleting child", "parent", klog.KObj(parent), "child", klog.KObj(obj))
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
				errs = append(errs, fmt.Errorf("can't delete %v: %w", describeObject(obj), err))
				continue
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func updateChildren(client *dynamicclientset.ResourceClient, updateStrategy ChildUpdateStrategy, parent *unstructured.Unstructured, observed, desired map[string]*unstructured.Unstructured) error {
	var errs []error
	for name, obj := range desired {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = parent.GetNamespace()
		}
		if oldObj := observed[name]; oldObj != nil {
			// Update
			newObj, err := ApplyUpdate(oldObj, obj)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			// Attempt an update, if the 3-way merge resulted in any changes.
			if reflect.DeepEqual(newObj.UnstructuredContent(), oldObj.UnstructuredContent()) {
				// Nothing changed.
				continue
			}
			if klog.V(5).Enabled() {
				klog.InfoS("Reflect diff: a=observed, b=desired", "diff", cmp.Diff(oldObj.UnstructuredContent(), newObj.UnstructuredContent()))
			}

			// Leave it alone if it's pending deletion.
			if oldObj.GetDeletionTimestamp() != nil {
				klog.InfoS("Not updating", "parent", klog.KObj(parent), "child", klog.KObj(obj), "reason", "Pending deletion of child object")
				continue
			}

			// Check the update strategy for this child kind.
			switch method := updateStrategy.GetMethod(client.Group, client.Kind); method {
			case v1alpha1.ChildUpdateOnDelete, "":
				// This means we don't try to update anything unless it gets deleted
				// by someone else (we won't delete it ourselves).
				klog.V(5).InfoS("Not updating", "parent", klog.KObj(parent), "child", klog.KObj(obj), "reason", "OnDelete update strategy selected")
				continue
			case v1alpha1.ChildUpdateRecreate, v1alpha1.ChildUpdateRollingRecreate:
				// Delete the object (now) and recreate it (on the next sync).
				klog.InfoS("Deleting for update", "parent", klog.KObj(parent), "child", klog.KObj(obj), "reason", "Recreate update strategy selected")
				uid := oldObj.GetUID()
				// Explicitly request deletion propagation, which is what users expect,
				// since some objects default to orphaning for backwards compatibility.
				propagation := metav1.DeletePropagationBackground
				err := client.Namespace(ns).Delete(
					context.TODO(),
					obj.GetName(),
					metav1.DeleteOptions{
						Preconditions:     &metav1.Preconditions{UID: &uid},
						PropagationPolicy: &propagation,
					},
				)
				if err != nil {
					errs = append(errs, err)
					continue
				}
			case v1alpha1.ChildUpdateInPlace, v1alpha1.ChildUpdateRollingInPlace:
				// Update the object in-place.
				klog.InfoS("Updating", "parent", klog.KObj(parent), "child", klog.KObj(obj), "reason", "Recreate update strategy selected")
				if _, err := client.Namespace(ns).Update(context.TODO(), newObj, metav1.UpdateOptions{}); err != nil {
					errs = append(errs, err)
					continue
				}
			default:
				errs = append(errs, fmt.Errorf("invalid update strategy for %v: unknown method %q", client.Kind, method))
				continue
			}
		} else {
			// Create
			klog.InfoS("Creating", "parent", klog.KObj(parent), "child", klog.KObj(obj))

			// The controller should return a partial object containing only the
			// fields it cares about. We save this partial object so we can do
			// a 3-way merge upon update, in the style of "kubectl apply".
			//
			// Make sure this happens before we add anything else to the object.
			if err := dynamicapply.SetLastApplied(obj, obj.UnstructuredContent()); err != nil {
				errs = append(errs, err)
				continue
			}

			// We always claim everything we create.
			controllerRef := MakeControllerRef(parent)
			ownerRefs := obj.GetOwnerReferences()
			ownerRefs = append(ownerRefs, *controllerRef)
			obj.SetOwnerReferences(ownerRefs)

			if _, err := client.Namespace(ns).Create(context.TODO(), obj, metav1.CreateOptions{}); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}
