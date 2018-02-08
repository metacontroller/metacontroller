/*
Copyright 2017 Google Inc.

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

package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	"k8s.io/metacontroller/apply"
	internallisters "k8s.io/metacontroller/client/generated/lister/metacontroller/v1alpha1"
	k8s "k8s.io/metacontroller/third_party/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/diff"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func syncAllCompositeControllers(dynClient *dynamicClientset, ccLister internallisters.CompositeControllerLister) error {
	ccList, err := ccLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("can't list CompositeControllers: %v", err)
	}

	for _, cc := range ccList {
		if err := syncCompositeController(dynClient, cc); err != nil {
			glog.Errorf("syncCompositeController: %v", err)
			continue
		}
	}
	return nil
}

func syncCompositeController(clientset *dynamicClientset, cc *v1alpha1.CompositeController) error {
	// Sync all objects of the parent type, in all namespaces.
	parentClient, err := clientset.Resource(cc.Spec.ParentResource.APIVersion, cc.Spec.ParentResource.Resource, "")
	if err != nil {
		return err
	}
	obj, err := parentClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("can't list %vs: %v", parentClient.Kind(), err)
	}
	list := obj.(*unstructured.UnstructuredList)
	for i := range list.Items {
		parent := &list.Items[i]
		if err := syncParentResource(clientset, cc, parentClient.APIResource(), parent); err != nil {
			glog.Errorf("can't sync %v %v/%v: %v", parentClient.Kind(), parent.GetNamespace(), parent.GetName(), err)
			continue
		}
	}

	return nil
}

func syncParentResource(clientset *dynamicClientset, cc *v1alpha1.CompositeController, parentResource *APIResource, parent *unstructured.Unstructured) error {
	labelSelector := &metav1.LabelSelector{}
	if cc.Spec.GenerateSelector {
		// Select by controller-uid, like Job does.
		// Any selector on the parent is ignored in this case.
		labelSelector = metav1.AddLabelToSelector(labelSelector, "controller-uid", string(parent.GetUID()))
	} else {
		// Get the parent's LabelSelector.
		if err := k8s.GetNestedFieldInto(&labelSelector, parent.UnstructuredContent(), "spec", "selector"); err != nil {
			return fmt.Errorf("can't get label selector from %v %v/%v", parentResource.Kind, parent.GetNamespace(), parent.GetName())
		}
	}

	// Claim all matching child resources, including orphan/adopt as necessary.
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return fmt.Errorf("can't convert label selector (%#v): %v", labelSelector, err)
	}
	children, err := claimChildren(clientset, cc, parentResource, parent, selector)
	if err != nil {
		return fmt.Errorf("can't claim children: %v", err)
	}

	// Call the sync hook for this parent.
	syncRequest := &syncHookRequest{
		Controller: cc,
		Parent:     parent,
		Children:   children,
	}
	syncResult, err := callSyncHook(cc, syncRequest)
	if err != nil {
		return fmt.Errorf("sync hook failed for %v %v/%v: %v", parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
	}

	// Remember manage error, but continue to update status regardless.
	var manageErr error
	if parent.GetDeletionTimestamp() == nil {
		// Reconcile children.
		if err := manageChildren(clientset, cc, parent, children, makeChildMap(syncResult.Children)); err != nil {
			manageErr = fmt.Errorf("can't reconcile children for %v %v/%v: %v", parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
		}
	}

	// Update parent status.
	// We'll want to make sure this happens after manageChildren once we support observedGeneration.
	if err := updateParentStatus(clientset, cc, parentResource, parent, syncResult.Status); err != nil {
		return fmt.Errorf("can't update status for %v %v/%v: %v", parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
	}

	return manageErr
}

func claimChildren(clientset *dynamicClientset, cc *v1alpha1.CompositeController, parentResource *APIResource, parent *unstructured.Unstructured, selector labels.Selector) (childMap, error) {
	// Set up values common to all child types.
	parentGVK := parentResource.GroupVersionKind()
	parentClient, err := clientset.Resource(parentResource.APIVersion, parentResource.Name, parent.GetNamespace())
	if err != nil {
		return nil, err
	}
	canAdoptFunc := k8s.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		// Make sure this is always an uncached read.
		fresh, err := parentClient.Get(parent.GetName(), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.GetUID() != parent.GetUID() {
			return nil, fmt.Errorf("original %v %v/%v is gone: got uid %v, wanted %v", parentResource.Kind, parent.GetNamespace(), parent.GetName(), fresh.GetUID(), parent.GetUID())
		}
		return fresh, nil
	})

	// Claim all child types.
	groups := make(childMap)
	for _, group := range cc.Spec.ChildResources {
		// Within each group/version, there can be multiple resources requested.
		for _, resourceName := range group.Resources {
			// List all objects of the child kind in the parent object's namespace.
			childClient, err := clientset.Resource(group.APIVersion, resourceName, parent.GetNamespace())
			if err != nil {
				return nil, err
			}
			obj, err := childClient.List(metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("can't list %v children: %v", childClient.Kind(), err)
			}
			childList := obj.(*unstructured.UnstructuredList)

			// Handle orphan/adopt and filter by owner+selector.
			crm := newDynamicControllerRefManager(childClient, parent, selector, parentGVK, childClient.GroupVersionKind(), canAdoptFunc)
			children, err := crm.claimChildren(childList.Items)
			if err != nil {
				return nil, fmt.Errorf("can't claim %v children: %v", childClient.Kind(), err)
			}

			// Add children to map by name.
			// Note that we limit each parent to only working within its own namespace.
			groupMap := make(map[string]*unstructured.Unstructured)
			for _, child := range children {
				groupMap[child.GetName()] = child
			}
			groups[fmt.Sprintf("%s.%s", childClient.Kind(), group.APIVersion)] = groupMap
		}
	}
	return groups, nil
}

func updateParentStatus(clientset *dynamicClientset, cc *v1alpha1.CompositeController, parentResource *APIResource, parent *unstructured.Unstructured, status map[string]interface{}) error {
	parentClient, err := clientset.Resource(parentResource.APIVersion, parentResource.Name, parent.GetNamespace())
	if err != nil {
		return err
	}
	// Overwrite .status field of parent object without touching other parts.
	// We can't use Patch() because we need to ensure that the UID matches.
	// TODO(enisoc): Use /status subresource when that exists.
	// TODO(enisoc): Update status.observedGeneration when spec.generation starts working.
	return parentClient.UpdateWithRetries(parent, func(obj *unstructured.Unstructured) bool {
		oldStatus := k8s.GetNestedField(obj.UnstructuredContent(), "status")
		if reflect.DeepEqual(oldStatus, status) {
			// Nothing to do.
			return false
		}
		k8s.SetNestedField(obj.UnstructuredContent(), status, "status")
		return true
	})
}

func deleteChildren(client *dynamicResourceClient, parent *unstructured.Unstructured, observed, desired map[string]*unstructured.Unstructured) error {
	var errs []error
	for name, obj := range observed {
		if obj.GetDeletionTimestamp() != nil {
			// Skip objects that are already pending deletion.
			continue
		}
		if desired == nil || desired[name] == nil {
			// This observed object wasn't listed as desired.
			glog.Infof("%v %v/%v: deleting %v %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())
			uid := obj.GetUID()
			err := client.Delete(name, &metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{UID: &uid},
			})
			if err != nil {
				errs = append(errs, fmt.Errorf("can't delete %v %v/%v: %v", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err))
				continue
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func updateChildren(client *dynamicResourceClient, parent *unstructured.Unstructured, observed, desired map[string]*unstructured.Unstructured) error {
	var errs []error
	for name, obj := range desired {
		if oldObj := observed[name]; oldObj != nil {
			// Update
			var newObj *unstructured.Unstructured

			// The controller only returns a partial object.
			// We compute the full updated object in the style of "kubectl apply".
			lastApplied, err := apply.GetLastApplied(oldObj)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			newObj = &unstructured.Unstructured{}
			newObj.Object, err = apply.Merge(oldObj.UnstructuredContent(), lastApplied, obj.UnstructuredContent())
			if err != nil {
				errs = append(errs, err)
				continue
			}
			apply.SetLastApplied(newObj, obj.UnstructuredContent())

			// Attempt an update, if the 3-way merge resulted in any changes.
			if !reflect.DeepEqual(newObj.UnstructuredContent(), oldObj.UnstructuredContent()) {
				glog.Infof("%v %v/%v: updating %v %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())
				if glog.V(5) {
					glog.Infof("reflect diff: a=observed, b=desired:\n%s", diff.ObjectReflectDiff(oldObj.UnstructuredContent(), newObj.UnstructuredContent()))
				}
				if _, err := client.Update(newObj); err != nil {
					errs = append(errs, err)
					continue
				}
			}
		} else {
			// Create
			glog.Infof("%v %v/%v: creating %v %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())

			// The controller should return a partial object containing only the
			// fields it cares about. We save this partial object so we can do
			// a 3-way merge upon update, in the style of "kubectl apply".
			//
			// Make sure this happens before we add anything else to the object.
			if err := apply.SetLastApplied(obj, obj.UnstructuredContent()); err != nil {
				errs = append(errs, err)
				continue
			}

			// For CompositeController, we always claim everything we create.
			controllerRef := map[string]interface{}{
				"apiVersion":         parent.GetAPIVersion(),
				"kind":               parent.GetKind(),
				"name":               parent.GetName(),
				"uid":                string(parent.GetUID()),
				"controller":         true,
				"blockOwnerDeletion": true,
			}
			ownerRefs, _ := k8s.GetNestedField(obj.UnstructuredContent(), "metadata", "ownerReferences").([]interface{})
			ownerRefs = append(ownerRefs, controllerRef)
			k8s.SetNestedField(obj.UnstructuredContent(), ownerRefs, "metadata", "ownerReferences")

			if _, err := client.Create(obj); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func manageChildren(clientset *dynamicClientset, cc *v1alpha1.CompositeController, parent *unstructured.Unstructured, observedChildren childMap, desiredChildren childMap) error {
	// If some operations fail, keep trying others so, for example,
	// we don't block recovery (create new Pod) on a failed delete.
	var errs []error

	// Delete observed, owned objects that are not desired.
	for key, objects := range observedChildren {
		apiVersion, kind := parseChildMapKey(key)
		client, err := clientset.Kind(apiVersion, kind, parent.GetNamespace())
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
		apiVersion, kind := parseChildMapKey(key)
		client, err := clientset.Kind(apiVersion, kind, parent.GetNamespace())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if cc.Spec.GenerateSelector {
			// Add the controller-uid label if there's none.
			for _, obj := range objects {
				labels := obj.GetLabels()
				if labels == nil {
					labels = make(map[string]string, 1)
				}
				if _, ok := labels["controller-uid"]; ok {
					continue
				}
				labels["controller-uid"] = string(parent.GetUID())
				obj.SetLabels(labels)
			}
		}
		if err := updateChildren(client, parent, observedChildren[key], objects); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

func makeChildMap(list []*unstructured.Unstructured) childMap {
	children := make(childMap)
	for _, child := range list {
		apiVersion := k8s.GetNestedString(child.UnstructuredContent(), "apiVersion")
		kind := k8s.GetNestedString(child.UnstructuredContent(), "kind")
		key := fmt.Sprintf("%s.%s", kind, apiVersion)

		if children[key] == nil {
			children[key] = make(map[string]*unstructured.Unstructured)
		}
		children[key][child.GetName()] = child
	}
	return children
}

func parseChildMapKey(key string) (apiVersion, kind string) {
	parts := strings.SplitN(key, ".", 2)
	return parts[1], parts[0]
}
