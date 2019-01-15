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
	"sync"
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"metacontroller.app/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller.app/client/generated/clientset/internalclientset"
	mclisters "metacontroller.app/client/generated/lister/metacontroller/v1alpha1"
	"metacontroller.app/controller/common"
	"metacontroller.app/controller/common/finalizer"
	dynamicclientset "metacontroller.app/dynamic/clientset"
	dynamiccontrollerref "metacontroller.app/dynamic/controllerref"
	dynamicdiscovery "metacontroller.app/dynamic/discovery"
	dynamicinformer "metacontroller.app/dynamic/informer"
	k8s "metacontroller.app/third_party/kubernetes"
)

type parentController struct {
	cc *v1alpha1.CompositeController

	resources      *dynamicdiscovery.ResourceMap
	parentResource *dynamicdiscovery.APIResource

	mcClient       mcclientset.Interface
	dynClient      *dynamicclientset.Clientset
	parentClient   *dynamicclientset.ResourceClient
	parentInformer *dynamicinformer.ResourceInformer

	revisionLister mclisters.ControllerRevisionLister

	stopCh, doneCh chan struct{}
	queue          workqueue.RateLimitingInterface

	updateStrategy updateStrategyMap
	childInformers common.InformerMap

	finalizer *finalizer.Manager
}

func newParentController(resources *dynamicdiscovery.ResourceMap, dynClient *dynamicclientset.Clientset, dynInformers *dynamicinformer.SharedInformerFactory, mcClient mcclientset.Interface, revisionLister mclisters.ControllerRevisionLister, cc *v1alpha1.CompositeController) (pc *parentController, newErr error) {
	// Make a dynamic client for the parent resource.
	parentClient, err := dynClient.Resource(cc.Spec.ParentResource.APIVersion, cc.Spec.ParentResource.Resource)
	if err != nil {
		return nil, err
	}
	parentResource := parentClient.APIResource

	updateStrategy, err := makeUpdateStrategyMap(resources, cc)
	if err != nil {
		return nil, err
	}

	// Create informer for the parent resource.
	parentInformer, err := dynInformers.Resource(cc.Spec.ParentResource.APIVersion, cc.Spec.ParentResource.Resource)
	if err != nil {
		return nil, fmt.Errorf("can't create informer for parent resource: %v", err)
	}

	// Create informers for all child resources.
	childInformers := make(common.InformerMap)
	defer func() {
		if newErr != nil {
			// If newParentController fails, Close() any informers we created
			// since Stop() will never be called.
			for _, childInformer := range childInformers {
				childInformer.Close()
			}
			parentInformer.Close()
		}
	}()
	for _, child := range cc.Spec.ChildResources {
		childInformer, err := dynInformers.Resource(child.APIVersion, child.Resource)
		if err != nil {
			return nil, fmt.Errorf("can't create informer for child resource: %v", err)
		}
		childInformers.Set(child.APIVersion, child.Resource, childInformer)
	}

	pc = &parentController{
		cc:             cc,
		resources:      resources,
		mcClient:       mcClient,
		dynClient:      dynClient,
		childInformers: childInformers,
		parentClient:   parentClient,
		parentInformer: parentInformer,
		parentResource: parentResource,
		revisionLister: revisionLister,
		updateStrategy: updateStrategy,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CompositeController-"+cc.Name),
		finalizer: &finalizer.Manager{
			Name:    "metacontroller.app/compositecontroller-" + cc.Name,
			Enabled: cc.Spec.Hooks.Finalize != nil,
		},
	}

	return pc, nil
}

func (pc *parentController) Start() {
	pc.stopCh = make(chan struct{})
	pc.doneCh = make(chan struct{})

	// Install event handlers. CompositeControllers can be created at any time,
	// so we have to assume the shared informers are already running. We can't
	// add event handlers in newParentController() since pc might be incomplete.
	parentHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.enqueueParentObject,
		UpdateFunc: pc.updateParentObject,
		DeleteFunc: pc.enqueueParentObject,
	}
	if pc.cc.Spec.ResyncPeriodSeconds != nil {
		// Use a custom resync period if requested. This only applies to the parent.
		resyncPeriod := time.Duration(*pc.cc.Spec.ResyncPeriodSeconds) * time.Second
		// Put a reasonable limit on it.
		if resyncPeriod < time.Second {
			resyncPeriod = time.Second
		}
		pc.parentInformer.Informer().AddEventHandlerWithResyncPeriod(parentHandlers, resyncPeriod)
	} else {
		pc.parentInformer.Informer().AddEventHandler(parentHandlers)
	}
	for _, childInformer := range pc.childInformers {
		childInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    pc.onChildAdd,
			UpdateFunc: pc.onChildUpdate,
			DeleteFunc: pc.onChildDelete,
		})
	}

	go func() {
		defer close(pc.doneCh)
		defer utilruntime.HandleCrash()

		glog.Infof("Starting %v CompositeController", pc.parentResource.Kind)
		defer glog.Infof("Shutting down %v CompositeController", pc.parentResource.Kind)

		// Wait for dynamic client and all informers.
		glog.Infof("Waiting for %v CompositeController caches to sync", pc.parentResource.Kind)
		syncFuncs := make([]cache.InformerSynced, 0, 2+len(pc.cc.Spec.ChildResources))
		syncFuncs = append(syncFuncs, pc.dynClient.HasSynced, pc.parentInformer.Informer().HasSynced)
		for _, childInformer := range pc.childInformers {
			syncFuncs = append(syncFuncs, childInformer.Informer().HasSynced)
		}
		if !k8s.WaitForCacheSync(pc.parentResource.Kind, pc.stopCh, syncFuncs...) {
			// We wait forever unless Stop() is called, so this isn't an error.
			glog.Warningf("%v CompositeController cache sync never finished", pc.parentResource.Kind)
			return
		}

		// 5 workers ought to be enough for anyone.
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				wait.Until(pc.worker, time.Second, pc.stopCh)
			}()
		}
		wg.Wait()
	}()
}

func (pc *parentController) Stop() {
	close(pc.stopCh)
	pc.queue.ShutDown()
	<-pc.doneCh

	// Remove event handlers and close informers for all child resources.
	for _, informer := range pc.childInformers {
		informer.Informer().RemoveEventHandlers()
		informer.Close()
	}
	// Remove event handlers and close informer for the parent resource.
	pc.parentInformer.Informer().RemoveEventHandlers()
	pc.parentInformer.Close()
}

func (pc *parentController) worker() {
	for pc.processNextWorkItem() {
	}
}

func (pc *parentController) processNextWorkItem() bool {
	key, quit := pc.queue.Get()
	if quit {
		return false
	}
	defer pc.queue.Done(key)

	err := pc.sync(key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to sync %v %q: %v", pc.parentResource.Kind, key, err))
		pc.queue.AddRateLimited(key)
		return true
	}

	pc.queue.Forget(key)
	return true
}

func (pc *parentController) enqueueParentObject(obj interface{}) {
	key, err := common.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	pc.queue.Add(key)
}

func (pc *parentController) updateParentObject(old, cur interface{}) {
	// Ignore updates where the ResourceVersion changed (not a resync)
	// but the spec hasn't changed (e.g. our own status updates).
	oldParent := old.(*unstructured.Unstructured)
	curParent := cur.(*unstructured.Unstructured)
	if curParent.GetDeletionTimestamp() == nil &&
		oldParent.GetDeletionTimestamp() == nil &&
		curParent.GetResourceVersion() != oldParent.GetResourceVersion() {
		oldSpec := k8s.GetNestedField(oldParent.UnstructuredContent(), "spec")
		curSpec := k8s.GetNestedField(curParent.UnstructuredContent(), "spec")
		if reflect.DeepEqual(oldSpec, curSpec) {
			return
		}
	}

	pc.enqueueParentObject(cur)
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (pc *parentController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *unstructured.Unstructured {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong APIGroup or Kind.
	if apiGroup, _ := common.ParseAPIVersion(controllerRef.APIVersion); apiGroup != pc.parentResource.Group {
		return nil
	}
	if controllerRef.Kind != pc.parentResource.Kind {
		return nil
	}
	parent, err := pc.parentInformer.Lister().Get(namespace, controllerRef.Name)
	if err != nil {
		return nil
	}
	if parent.GetUID() != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return parent
}

func (pc *parentController) onChildAdd(obj interface{}) {
	child := obj.(*unstructured.Unstructured)

	if child.GetDeletionTimestamp() != nil {
		pc.onChildDelete(child)
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(child); controllerRef != nil {
		parent := pc.resolveControllerRef(child.GetNamespace(), controllerRef)
		if parent == nil {
			// The controllerRef isn't a parent we know about.
			return
		}
		glog.V(4).Infof("%v %v/%v: child %v %v created or updated", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), child.GetKind(), child.GetName())
		pc.enqueueParentObject(parent)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching parents and sync
	// them to see if anyone wants to adopt it.
	parents := pc.findPotentialParents(child)
	if len(parents) == 0 {
		return
	}
	glog.V(4).Infof("%v: orphan child %v %s created or updated", pc.parentResource.Kind, child.GetKind(), child.GetName())
	for _, parent := range parents {
		pc.enqueueParentObject(parent)
	}
}

func (pc *parentController) onChildUpdate(old, cur interface{}) {
	oldChild := old.(*unstructured.Unstructured)
	curChild := cur.(*unstructured.Unstructured)

	// Don't sync if it's a no-op update (probably a relist/resync).
	// We don't care about resyncs for children; we rely on the parent resync.
	if oldChild.GetResourceVersion() == curChild.GetResourceVersion() {
		return
	}

	// Other than that, we treat updates the same as creates.
	// Level-triggered controllers shouldn't care what the old state was.
	pc.onChildAdd(cur)
}

func (pc *parentController) onChildDelete(obj interface{}) {
	child, ok := obj.(*unstructured.Unstructured)

	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		child, ok = tombstone.Obj.(*unstructured.Unstructured)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not *unstructured.Unstructured %#v", obj))
			return
		}
	}

	// If it's an orphan, there's nothing to do because we never adopt orphans
	// that are being deleted.
	controllerRef := metav1.GetControllerOf(child)
	if controllerRef == nil {
		return
	}

	// Sync the parent of this child (if it's ours).
	parent := pc.resolveControllerRef(child.GetNamespace(), controllerRef)
	if parent == nil {
		// The controllerRef isn't a parent we know about.
		return
	}
	glog.V(4).Infof("%v %v/%v: child %v %v deleted", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), child.GetKind(), child.GetName())
	pc.enqueueParentObject(parent)
}

func (pc *parentController) findPotentialParents(child *unstructured.Unstructured) []*unstructured.Unstructured {
	childLabels := labels.Set(child.GetLabels())

	parents, err := pc.parentInformer.Lister().ListNamespace(child.GetNamespace(), labels.Everything())
	if err != nil {
		return nil
	}

	var matchingParents []*unstructured.Unstructured
	for _, parent := range parents {
		selector, err := pc.makeSelector(parent, nil)
		if err != nil || selector.Empty() {
			continue
		}
		if selector.Matches(childLabels) {
			matchingParents = append(matchingParents, parent)
		}
	}
	return matchingParents
}

func (pc *parentController) sync(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	glog.V(4).Infof("sync %v %v/%v", pc.parentResource.Kind, namespace, name)

	parent, err := pc.parentInformer.Lister().Get(namespace, name)
	if apierrors.IsNotFound(err) {
		// Swallow the error since there's no point retrying if the parent is gone.
		glog.V(4).Infof("%v %v/%v has been deleted", pc.parentResource.Kind, namespace, name)
		return nil
	}
	if err != nil {
		return err
	}
	return pc.syncParentObject(parent)
}

func (pc *parentController) syncParentObject(parent *unstructured.Unstructured) error {
	// Before taking any other action, add our finalizer (if desired).
	// This ensures we have a chance to clean up after any action we later take.
	updatedParent, err := pc.finalizer.SyncObject(pc.parentClient, parent)
	if err != nil {
		// If we fail to do this, abort before doing anything else and requeue.
		return fmt.Errorf("can't sync finalizer for %v %v/%v: %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
	}
	parent = updatedParent

	// Claim all matching child resources, including orphan/adopt as necessary.
	observedChildren, err := pc.claimChildren(parent)
	if err != nil {
		return err
	}

	// Reconcile ControllerRevisions belonging to this parent.
	// Call the sync hook for each revision, then compute the overall status and
	// desired children, accounting for any rollout in progress.
	parentStatus, desiredChildren, finalized, err := pc.syncRevisions(parent, observedChildren)
	if err != nil {
		return err
	}

	// If all revisions agree that they've finished finalizing,
	// remove our finalizer.
	if finalized {
		updatedParent, err := pc.parentClient.Namespace(parent.GetNamespace()).RemoveFinalizer(parent, pc.finalizer.Name)
		if err != nil {
			return fmt.Errorf("can't remove finalizer for %v %v/%v: %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
		}
		parent = updatedParent
	}

	// Enforce invariants between parent selector and child labels.
	selector, err := pc.makeSelector(parent, nil)
	if err != nil {
		return err
	}
	for _, group := range desiredChildren {
		for _, obj := range group {
			// We don't use GetLabels() because that swallows conversion errors.
			objLabels, _, err := unstructured.NestedStringMap(obj.UnstructuredContent(), "metadata", "labels")
			if err != nil {
				return fmt.Errorf("invalid labels on desired child %v %v/%v: %v", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
			}
			// If selector generation is enabled, add the controller-uid label to all
			// desired children so they match the generated selector.
			if pc.cc.Spec.GenerateSelector != nil && *pc.cc.Spec.GenerateSelector {
				if objLabels == nil {
					objLabels = make(map[string]string, 1)
				}
				if _, ok := objLabels["controller-uid"]; !ok {
					objLabels["controller-uid"] = string(parent.GetUID())
					obj.SetLabels(objLabels)
				}
			}
			// Make sure all desired children match the parent's selector.
			// We consider it user error to try to create children that would be
			// immediately orphaned.
			if !selector.Matches(labels.Set(objLabels)) {
				return fmt.Errorf("labels on desired child %v %v/%v don't match parent selector", obj.GetKind(), obj.GetNamespace(), obj.GetName())
			}
		}
	}

	// Reconcile child objects belonging to this parent.
	// Remember manage error, but continue to update status regardless.
	//
	// We only manage children if the parent is "alive" (not pending deletion),
	// or if it's pending deletion and we have a `finalize` hook.
	var manageErr error
	if parent.GetDeletionTimestamp() == nil || pc.finalizer.ShouldFinalize(parent) {
		// Reconcile children.
		if err := common.ManageChildren(pc.dynClient, pc.updateStrategy, parent, observedChildren, desiredChildren); err != nil {
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

	if pc.cc.Spec.GenerateSelector != nil && *pc.cc.Spec.GenerateSelector {
		// Select by controller-uid, like Job does.
		// Any selector on the parent is ignored in this case.
		labelSelector = metav1.AddLabelToSelector(labelSelector, "controller-uid", string(parent.GetUID()))
	} else {
		// Get the parent's LabelSelector.
		if err := k8s.GetNestedFieldInto(labelSelector, parent.UnstructuredContent(), "spec", "selector"); err != nil {
			return nil, fmt.Errorf("can't get label selector from %v %v/%v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName())
		}
		// An empty selector doesn't make sense for a CompositeController parent.
		// This is likely user error, and could be dangerous (selecting everything).
		if len(labelSelector.MatchLabels) == 0 && len(labelSelector.MatchExpressions) == 0 {
			return nil, fmt.Errorf(".spec.selector must have either matchLabels, matchExpressions, or both")
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
	return k8s.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		// Make sure this is always an uncached read.
		fresh, err := pc.parentClient.Namespace(parent.GetNamespace()).Get(parent.GetName(), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.GetUID() != parent.GetUID() {
			return nil, fmt.Errorf("original %v %v/%v is gone: got uid %v, wanted %v", pc.parentResource.Kind, parent.GetNamespace(), parent.GetName(), fresh.GetUID(), parent.GetUID())
		}
		return fresh, nil
	})
}

func (pc *parentController) claimChildren(parent *unstructured.Unstructured) (common.ChildMap, error) {
	// Set up values common to all child types.
	namespace := parent.GetNamespace()
	parentGVK := pc.parentResource.GroupVersionKind()
	selector, err := pc.makeSelector(parent, nil)
	if err != nil {
		return nil, err
	}
	canAdoptFunc := pc.canAdoptFunc(parent)

	// Claim all child types.
	childMap := make(common.ChildMap)
	for _, child := range pc.cc.Spec.ChildResources {
		// List all objects of the child kind in the parent object's namespace.
		childClient, err := pc.dynClient.Resource(child.APIVersion, child.Resource)
		if err != nil {
			return nil, err
		}
		informer := pc.childInformers.Get(child.APIVersion, child.Resource)
		if informer == nil {
			return nil, fmt.Errorf("no informer for resource %q in apiVersion %q", child.Resource, child.APIVersion)
		}
		all, err := informer.Lister().ListNamespace(namespace, labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("can't list %v children: %v", childClient.Kind, err)
		}

		// Always include the requested groups, even if there are no entries.
		childMap.InitGroup(child.APIVersion, childClient.Kind)

		// Handle orphan/adopt and filter by owner+selector.
		crm := dynamiccontrollerref.NewUnstructuredManager(childClient, parent, selector, parentGVK, childClient.GroupVersionKind(), canAdoptFunc)
		children, err := crm.ClaimChildren(all)
		if err != nil {
			return nil, fmt.Errorf("can't claim %v children: %v", childClient.Kind, err)
		}

		// Add children to map by name.
		// Note that we limit each parent to only working within its own namespace.
		for _, obj := range children {
			childMap.Insert(parent, obj)
		}
	}
	return childMap, nil
}

func (pc *parentController) updateParentStatus(parent *unstructured.Unstructured, status map[string]interface{}) (*unstructured.Unstructured, error) {
	// Overwrite .status field of parent object without touching other parts.
	// We can't use Patch() because we need to ensure that the UID matches.
	return pc.parentClient.Namespace(parent.GetNamespace()).AtomicStatusUpdate(parent, func(obj *unstructured.Unstructured) bool {
		oldStatus := k8s.GetNestedField(obj.UnstructuredContent(), "status")
		if reflect.DeepEqual(oldStatus, status) {
			// Nothing to do.
			return false
		}

		k8s.SetNestedField(obj.UnstructuredContent(), status, "status")
		k8s.SetNestedField(obj.UnstructuredContent(), parent.GetGeneration(), "status", "observedGeneration")
		return true
	})
}
