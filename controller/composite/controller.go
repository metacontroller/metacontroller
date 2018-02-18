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

package composite

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/diff"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	mcclientset "k8s.io/metacontroller/client/generated/clientset/internalclientset"
	mclisters "k8s.io/metacontroller/client/generated/lister/metacontroller/v1alpha1"
	dynamicapply "k8s.io/metacontroller/dynamic/apply"
	dynamicclientset "k8s.io/metacontroller/dynamic/clientset"
	dynamiccontrollerref "k8s.io/metacontroller/dynamic/controllerref"
	dynamicdiscovery "k8s.io/metacontroller/dynamic/discovery"
	k8s "k8s.io/metacontroller/third_party/kubernetes"
)

type parentController struct {
	cc *v1alpha1.CompositeController

	resources      *dynamicdiscovery.ResourceMap
	mcClient       mcclientset.Interface
	dynClient      *dynamicclientset.Clientset
	parentClient   *dynamicclientset.ResourceClient
	parentResource *dynamicdiscovery.APIResource

	revisionLister mclisters.ControllerRevisionLister

	stopCh, doneCh chan struct{}

	updateStrategy updateStrategyMap
}

func newParentController(resources *dynamicdiscovery.ResourceMap, dynClient *dynamicclientset.Clientset, mcClient mcclientset.Interface, revisionLister mclisters.ControllerRevisionLister, cc *v1alpha1.CompositeController) (*parentController, error) {
	// Make a dynamic client for the parent resource.
	parentClient, err := dynClient.Resource(cc.Spec.ParentResource.APIVersion, cc.Spec.ParentResource.Resource, "")
	if err != nil {
		return nil, err
	}

	pc := &parentController{
		cc:             cc,
		resources:      resources,
		mcClient:       mcClient,
		dynClient:      dynClient,
		parentClient:   parentClient,
		parentResource: parentClient.APIResource(),
		revisionLister: revisionLister,
	}

	pc.updateStrategy, err = makeUpdateStrategyMap(resources, cc)
	if err != nil {
		return nil, err
	}

	return pc, nil
}

func (pc *parentController) Start() {
	pc.stopCh = make(chan struct{})
	pc.doneCh = make(chan struct{})

	go func() {
		defer close(pc.doneCh)

		glog.Infof("Starting %v CompositeController", pc.parentResource.Kind)
		defer glog.Infof("Shutting down %v CompositeController", pc.parentResource.Kind)

		// Wait for dynamic client to populate discovery cache.
		if !k8s.WaitForCacheSync(pc.parentResource.Kind, pc.stopCh, pc.dynClient.HasSynced) {
			return
		}

		// Start polling in the background.
		// This interval isn't configurable because polling is going away soon.
		// TODO(kube-metacontroller#8): Replace with shared, dynamic informers.
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-pc.stopCh:
				return
			case <-ticker.C:
				if err := pc.syncAll(); err != nil {
					utilruntime.HandleError(fmt.Errorf("can't sync %v: %v", pc.parentResource.Kind, err))
				}
			}
		}
	}()
}

func (pc *parentController) Stop() {
	close(pc.stopCh)
	<-pc.doneCh
}

func (pc *parentController) syncAll() error {
	// Sync all objects of the parent type, in all namespaces.
	obj, err := pc.parentClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("can't list %vs: %v", pc.parentResource.Kind, err)
	}
	list := obj.(*unstructured.UnstructuredList)
	for i := range list.Items {
		parent := &list.Items[i]
		if err := pc.syncParentObject(parent); err != nil {
			glog.Errorf("can't sync %v %v/%v: %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
			continue
		}
	}

	return nil
}

func (pc *parentController) syncParentObject(parent *unstructured.Unstructured) error {
	// Claim all matching child resources, including orphan/adopt as necessary.
	observedChildren, err := pc.claimChildren(parent)
	if err != nil {
		return err
	}

	// Reconcile ControllerRevisions belonging to this parent.
	// Call the sync hook for each revision, then compute the overall status and
	// desired children, accounting for any rollout in progress.
	parentStatus, desiredChildren, err := pc.syncRevisions(parent, observedChildren)
	if err != nil {
		return err
	}

	// Reconcile child objects belonging to this parent.
	// Remember manage error, but continue to update status regardless.
	var manageErr error
	if parent.GetDeletionTimestamp() == nil {
		// Reconcile children.
		if err := pc.manageChildren(parent, observedChildren, desiredChildren); err != nil {
			manageErr = fmt.Errorf("can't reconcile children for %v %v/%v: %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
		}
	}

	// Update parent status.
	// We'll want to make sure this happens after manageChildren once we support observedGeneration.
	if _, err := pc.updateParentStatus(parent, parentStatus); err != nil {
		return fmt.Errorf("can't update status for %v %v/%v: %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), err)
	}

	return manageErr
}

func (pc *parentController) makeSelector(parent *unstructured.Unstructured, extraMatchLabels map[string]string) (labels.Selector, error) {
	labelSelector := &metav1.LabelSelector{}

	if pc.cc.Spec.GenerateSelector {
		// Select by controller-uid, like Job does.
		// Any selector on the parent is ignored in this case.
		labelSelector = metav1.AddLabelToSelector(labelSelector, "controller-uid", string(parent.GetUID()))
	} else {
		// Get the parent's LabelSelector.
		if err := k8s.GetNestedFieldInto(labelSelector, parent.UnstructuredContent(), "spec", "selector"); err != nil {
			return nil, fmt.Errorf("can't get label selector from %v %v/%v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName())
		}
	}

	for key, value := range extraMatchLabels {
		labelSelector = metav1.AddLabelToSelector(labelSelector, key, value)
	}

	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("can't convert label selector (%#v): %v", labelSelector, err)
	}
	return selector, nil
}

func (pc *parentController) canAdoptFunc(parent *unstructured.Unstructured) func() error {
	nsParentClient := pc.parentClient.WithNamespace(parent.GetNamespace())
	return k8s.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		// Make sure this is always an uncached read.
		fresh, err := nsParentClient.Get(parent.GetName(), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.GetUID() != parent.GetUID() {
			return nil, fmt.Errorf("original %v %v/%v is gone: got uid %v, wanted %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), fresh.GetUID(), parent.GetUID())
		}
		return fresh, nil
	})
}

func (pc *parentController) claimChildren(parent *unstructured.Unstructured) (childMap, error) {
	// Set up values common to all child types.
	parentGVK := pc.parentResource.GroupVersionKind()
	selector, err := pc.makeSelector(parent, nil)
	if err != nil {
		return nil, err
	}
	canAdoptFunc := pc.canAdoptFunc(parent)

	// Claim all child types.
	groups := make(childMap)
	for _, child := range pc.cc.Spec.ChildResources {
		// List all objects of the child kind in the parent object's namespace.
		childClient, err := pc.dynClient.Resource(child.APIVersion, child.Resource, parent.GetNamespace())
		if err != nil {
			return nil, err
		}
		obj, err := childClient.List(metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("can't list %v children: %v", childClient.Kind(), err)
		}
		childList := obj.(*unstructured.UnstructuredList)

		// Handle orphan/adopt and filter by owner+selector.
		crm := dynamiccontrollerref.NewUnstructuredManager(childClient, parent, selector, parentGVK, childClient.GroupVersionKind(), canAdoptFunc)
		children, err := crm.ClaimChildren(childList.Items)
		if err != nil {
			return nil, fmt.Errorf("can't claim %v children: %v", childClient.Kind(), err)
		}

		// Add children to map by name.
		// Note that we limit each parent to only working within its own namespace.
		groupMap := make(map[string]*unstructured.Unstructured)
		for _, child := range children {
			groupMap[child.GetName()] = child
		}
		groups[fmt.Sprintf("%s.%s", childClient.Kind(), child.APIVersion)] = groupMap
	}
	return groups, nil
}

func (pc *parentController) updateParentStatus(parent *unstructured.Unstructured, status map[string]interface{}) (*unstructured.Unstructured, error) {
	// Overwrite .status field of parent object without touching other parts.
	// We can't use Patch() because we need to ensure that the UID matches.
	// TODO(enisoc): Use /status subresource when that exists.
	// TODO(enisoc): Update status.observedGeneration when spec.generation starts working.
	nsParentClient := pc.parentClient.WithNamespace(parent.GetNamespace())
	return nsParentClient.UpdateWithRetries(parent, func(obj *unstructured.Unstructured) bool {
		oldStatus := k8s.GetNestedField(obj.UnstructuredContent(), "status")
		if reflect.DeepEqual(oldStatus, status) {
			// Nothing to do.
			return false
		}
		k8s.SetNestedField(obj.UnstructuredContent(), status, "status")
		return true
	})
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

func (pc *parentController) updateChildren(client *dynamicclientset.ResourceClient, parent *unstructured.Unstructured, observed, desired map[string]*unstructured.Unstructured) error {
	var errs []error
	for name, obj := range desired {
		if oldObj := observed[name]; oldObj != nil {
			// Update
			newObj, err := applyUpdate(oldObj, obj)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			// Attempt an update, if the 3-way merge resulted in any changes.
			if reflect.DeepEqual(newObj.UnstructuredContent(), oldObj.UnstructuredContent()) {
				// Nothing changed.
				continue
			}

			// Leave it alone if it's pending deletion.
			if oldObj.GetDeletionTimestamp() != nil {
				glog.Infof("%v %v/%v: not updating %v %v (pending deletion)", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())
				continue
			}

			// Check the update strategy for this child kind.
			strategy := pc.updateStrategy.get(client.GroupVersion().Group, client.Kind())
			if strategy == nil {
				// No update strategy specified, or OnDelete specified.
				// This means we don't try to update anything unless it gets deleted
				// by someone else (we won't delete it ourselves).
				glog.V(2).Infof("%v %v/%v: not updating %v %v (OnDelete update strategy)", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())
				continue
			}

			switch strategy.Method {
			case v1alpha1.ChildUpdateRecreate, v1alpha1.ChildUpdateRollingRecreate:
				// Delete the object (now) and recreate it (on the next sync).
				glog.Infof("%v %v/%v: deleting %v %v for update", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())
				uid := oldObj.GetUID()
				err := client.Delete(name, &metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{UID: &uid},
				})
				if err != nil {
					errs = append(errs, err)
					continue
				}
			case v1alpha1.ChildUpdateInPlace, v1alpha1.ChildUpdateRollingInPlace:
				// Update the object in-place.
				glog.Infof("%v %v/%v: updating %v %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())
				if glog.V(5) {
					glog.Infof("reflect diff: a=observed, b=desired:\n%s", diff.ObjectReflectDiff(oldObj.UnstructuredContent(), newObj.UnstructuredContent()))
				}
				if _, err := client.Update(newObj); err != nil {
					errs = append(errs, err)
					continue
				}
			default:
				errs = append(errs, fmt.Errorf("invalid update strategy for %v: unknown method %q", client.Kind(), strategy.Method))
				continue
			}
		} else {
			// Create
			glog.Infof("%v %v/%v: creating %v %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), obj.GetKind(), obj.GetName())

			// The controller should return a partial object containing only the
			// fields it cares about. We save this partial object so we can do
			// a 3-way merge upon update, in the style of "kubectl apply".
			//
			// Make sure this happens before we add anything else to the object.
			if err := dynamicapply.SetLastApplied(obj, obj.UnstructuredContent()); err != nil {
				errs = append(errs, err)
				continue
			}

			// For CompositeController, we always claim everything we create.
			controllerRef := makeControllerRef(parent)
			ownerRefs := obj.GetOwnerReferences()
			ownerRefs = append(ownerRefs, *controllerRef)
			obj.SetOwnerReferences(ownerRefs)

			if _, err := client.Create(obj); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (pc *parentController) manageChildren(parent *unstructured.Unstructured, observedChildren, desiredChildren childMap) error {
	// If some operations fail, keep trying others so, for example,
	// we don't block recovery (create new Pod) on a failed delete.
	var errs []error

	// Delete observed, owned objects that are not desired.
	for key, objects := range observedChildren {
		apiVersion, kind := parseChildMapKey(key)
		client, err := pc.dynClient.Kind(apiVersion, kind, parent.GetNamespace())
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
		client, err := pc.dynClient.Kind(apiVersion, kind, parent.GetNamespace())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if pc.cc.Spec.GenerateSelector {
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
		if err := pc.updateChildren(client, parent, observedChildren[key], objects); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

func applyUpdate(orig, update *unstructured.Unstructured) (*unstructured.Unstructured, error) {
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
	dynamicapply.SetLastApplied(newObj, update.UnstructuredContent())
	return newObj, nil
}

func makeControllerRef(parent *unstructured.Unstructured) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion:         parent.GetAPIVersion(),
		Kind:               parent.GetKind(),
		Name:               parent.GetName(),
		UID:                parent.GetUID(),
		Controller:         k8s.BoolPtr(true),
		BlockOwnerDeletion: k8s.BoolPtr(true),
	}
}
