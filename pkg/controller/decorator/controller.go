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

package decorator

import (
	"context"
	"fmt"
	commonv1 "metacontroller/pkg/controller/common/api/v1"
	"metacontroller/pkg/hooks"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"

	dynamicinformer "metacontroller/pkg/dynamic/informer"

	"metacontroller/pkg/events"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/common/customize"
	"metacontroller/pkg/controller/common/finalizer"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
)

const (
	decoratorControllerAnnotation = "metacontroller.k8s.io/decorator-controller"
)

type decoratorController struct {
	dc *v1alpha1.DecoratorController

	resources *dynamicdiscovery.ResourceMap

	parentKinds    common.GroupKindMap
	parentSelector *decoratorSelector

	dynClient *dynamicclientset.Clientset

	stopCh, doneCh chan struct{}
	queue          workqueue.RateLimitingInterface

	updateStrategy updateStrategyMap

	parentInformers common.InformerMap
	childInformers  common.InformerMap
	dynInformers    *dynamicinformer.SharedInformerFactory

	numWorkers    int
	eventRecorder record.EventRecorder

	finalizer    *finalizer.Manager
	customize    *customize.Manager
	syncHook     hooks.Hook
	finalizeHook hooks.Hook

	logger logr.Logger
}

func newDecoratorController(
	resources *dynamicdiscovery.ResourceMap,
	dynClient *dynamicclientset.Clientset,
	dynInformers *dynamicinformer.SharedInformerFactory,
	eventRecorder record.EventRecorder,
	dc *v1alpha1.DecoratorController,
	numWorkers int,
	logger logr.Logger,
) (controller *decoratorController, newErr error) {
	if dc.Spec.Hooks == nil {
		return nil, fmt.Errorf("no hooks defined")
	}
	syncHook, err := hooks.NewHook(dc.Spec.Hooks.Sync, dc.Name, common.DecoratorController, common.SyncHook)
	if err != nil {
		return nil, err
	}
	finalizeHook, err := hooks.NewHook(dc.Spec.Hooks.Finalize, dc.Name, common.DecoratorController, common.FinalizeHook)
	if err != nil {
		return nil, err
	}

	c := &decoratorController{
		dc:              dc,
		resources:       resources,
		dynClient:       dynClient,
		parentKinds:     make(common.GroupKindMap),
		parentInformers: make(common.InformerMap),
		childInformers:  make(common.InformerMap),
		dynInformers:    dynInformers,

		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), common.DecoratorController.String()+"-"+dc.Name),
		numWorkers:    numWorkers,
		eventRecorder: eventRecorder,
		finalizer: finalizer.NewManager(
			"metacontroller.io/decoratorcontroller-"+dc.Name,
			dc.Spec.Hooks.Finalize != nil,
		),
		syncHook:     syncHook,
		finalizeHook: finalizeHook,
		logger:       logger.WithName(dc.Name),
	}

	customize, err := customize.NewCustomizeManager(
		dc.Name,
		c.enqueueParentObject,
		dc,
		dynClient,
		dynInformers,
		c.parentInformers,
		c.parentKinds,
		c.logger,
		common.CompositeController,
	)
	if err != nil {
		return nil, err
	}
	c.customize = customize

	c.parentSelector, err = newDecoratorSelector(resources, dc)
	if err != nil {
		return nil, err
	}

	// Keep a list of parent resource info from discovery.
	for _, parent := range dc.Spec.Resources {
		resource := resources.Get(parent.APIVersion, parent.Resource)
		if resource == nil {
			return nil, fmt.Errorf("can't find resource %q in apiVersion %q", parent.Resource, parent.APIVersion)
		}
		c.parentKinds.Set(schema.GroupKind{Group: resource.Group, Kind: resource.Kind}, resource)
	}

	// Remember the update strategy for each child type.
	c.updateStrategy, err = makeUpdateStrategyMap(resources, dc)
	if err != nil {
		return nil, err
	}

	// Create informers for all parent and child resources.
	defer func() {
		if newErr != nil {
			// If newDecoratorController fails, Close() any informers we created
			// since Stop() will never be called.
			for _, informer := range c.childInformers {
				informer.Close()
			}
			for _, informer := range c.parentInformers {
				informer.Close()
			}
		}
	}()

	for _, parent := range dc.Spec.Resources {
		informer, err := dynInformers.Resource(parent.APIVersion, parent.Resource)
		if err != nil {
			return nil, fmt.Errorf("can't create informer for parent resource: %w", err)
		}
		groupVersion, err := schema.ParseGroupVersion(parent.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("can't parse parent resource groupVersion: %w", err)
		}
		c.parentInformers.Set(groupVersion.WithResource(parent.Resource), informer)
	}

	for _, child := range dc.Spec.Attachments {
		informer, err := dynInformers.Resource(child.APIVersion, child.Resource)
		if err != nil {
			return nil, fmt.Errorf("can't create informer for child resource: %w", err)
		}
		groupVersion, err := schema.ParseGroupVersion(child.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("can't parse child resource groupVersion: %w", err)
		}
		c.childInformers.Set(groupVersion.WithResource(child.Resource), informer)
	}

	return c, nil
}

func (c *decoratorController) Start() {
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})

	// Install event handlers. DecoratorControllers can be created at any time,
	// so we have to assume the shared informers are already running. We can't
	// add event handlers in newParentController() since c might be incomplete.
	parentHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueueParentObject,
		UpdateFunc: c.updateParentObject,
		DeleteFunc: c.enqueueParentObject,
	}
	var resyncPeriod time.Duration
	if c.dc.Spec.ResyncPeriodSeconds != nil {
		// Use a custom resync period if requested. This only applies to the parent.
		resyncPeriod = time.Duration(*c.dc.Spec.ResyncPeriodSeconds) * time.Second
		// Put a reasonable limit on it.
		if resyncPeriod < time.Second {
			resyncPeriod = time.Second
		}
	}
	for _, informer := range c.parentInformers {
		if resyncPeriod != 0 {
			informer.Informer().AddEventHandlerWithResyncPeriod(parentHandlers, resyncPeriod)
		} else {
			informer.Informer().AddEventHandler(parentHandlers)
		}
	}
	for _, informer := range c.childInformers {
		informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.onChildAdd,
			UpdateFunc: c.onChildUpdate,
			DeleteFunc: c.onChildDelete,
		})
	}

	go func() {
		defer close(c.doneCh)
		defer utilruntime.HandleCrash()

		c.logger.Info("Starting DecoratorController", "controller", c.dc)
		c.eventRecorder.Eventf(c.dc, corev1.EventTypeNormal, events.ReasonStarting, "Starting controller: %s", c.dc.Name)
		defer c.logger.Info("Shutting down DecoratorController", "controller", c.dc)
		defer c.eventRecorder.Eventf(c.dc, corev1.EventTypeNormal, events.ReasonStopping, "Stopping controller: %s", c.dc.Name)

		// Wait for dynamic client and all informers.
		c.logger.Info("Waiting for DecoratorController caches to sync", "controller", c.dc)
		syncFuncs := make([]cache.InformerSynced, 0, 1+len(c.dc.Spec.Resources)+len(c.dc.Spec.Attachments))
		for _, informer := range c.parentInformers {
			syncFuncs = append(syncFuncs, informer.Informer().HasSynced)
		}
		for _, informer := range c.childInformers {
			syncFuncs = append(syncFuncs, informer.Informer().HasSynced)
		}
		if !cache.WaitForNamedCacheSync(c.dc.Name, c.stopCh, syncFuncs...) {
			// We wait forever unless Stop() is called, so this isn't an error.
			c.logger.Info("DecoratorController cache sync never finished", "controller", c.dc)
			return
		}

		var wg sync.WaitGroup
		for i := 0; i < c.numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				wait.Until(c.worker, time.Second, c.stopCh)
			}()
		}
		wg.Wait()
	}()
}

func (c *decoratorController) Stop() {
	close(c.stopCh)
	c.queue.ShutDown()
	<-c.doneCh

	// Remove event handlers and close informers for all child resources.
	for _, informer := range c.childInformers {
		informer.Informer().RemoveEventHandlers()
		informer.Close()
	}
	// Remove event handlers and close informer for all parent resources.
	for _, informer := range c.parentInformers {
		informer.Informer().RemoveEventHandlers()
		informer.Close()
	}
	c.customize.Stop()
}

func (c *decoratorController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *decoratorController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	if err := c.sync(key.(string)); err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to sync %v '%v': %w", c.dc.Name, key, err))
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	return true
}

func (c *decoratorController) enqueueParentObject(obj interface{}) {
	// If the parent doesn't match our selector, and it doesn't have our
	// finalizer, we don't care about it.
	if parent, ok := obj.(*unstructured.Unstructured); ok {
		if !c.parentSelector.Matches(parent) && !controllerutil.ContainsFinalizer(parent, c.finalizer.Name) {
			return
		}
	}

	key, err := parentQueueKey(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %w", obj, err))
		return
	}
	c.queue.Add(key)
}

func (c *decoratorController) enqueueParentObjectAfter(obj interface{}, delay time.Duration) {
	key, err := parentQueueKey(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %w", obj, err))
		return
	}
	c.queue.AddAfter(key, delay)
}

func (c *decoratorController) updateParentObject(old, cur interface{}) {
	// TODO(enisoc): Is there any way to avoid resyncing after our own updates?
	c.enqueueParentObject(cur)
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (c *decoratorController) resolveControllerRef(childNamespace string, controllerRef *metav1.OwnerReference) *unstructured.Unstructured {
	// Is the controllerRef pointing to one of the parent resources we care about?
	// Only look at the group and kind; it doesn't matter if the controller uses
	// a different version than we do.
	apiGroup, _ := schema.ParseGroupVersion(controllerRef.APIVersion)
	resource := c.parentKinds.Get(schema.GroupKind{Group: apiGroup.Group, Kind: controllerRef.Kind})
	if resource == nil {
		// It's not one of the resources we care about.
		return nil
	}
	// Get the lister for this resource.
	groupVersion, _ := schema.ParseGroupVersion(resource.APIVersion)
	informer := c.parentInformers.Get(groupVersion.WithResource(resource.Name))
	if informer == nil {
		return nil
	}
	// We can't look up by UID, so look up by Namespace/Name and then verify UID.
	parentNamespace := ""
	if resource.Namespaced {
		// If the parent is namespaced, it must be in the same namespace as the
		// child because controllerRef does not support cross-namespace references
		// (except for namespaced child -> cluster-scoped parent).
		parentNamespace = childNamespace
	}
	parent, err := common.GetObject(informer, parentNamespace, controllerRef.Name)
	if err != nil {
		return nil
	}
	if parent.GetUID() != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	if !c.parentSelector.Matches(parent) && !controllerutil.ContainsFinalizer(parent, c.finalizer.Name) {
		// If the parent doesn't match our selector and doesn't have our finalizer,
		// we don't care about it.
		return nil
	}
	return parent
}

func (c *decoratorController) onChildAdd(obj interface{}) {
	child := obj.(*unstructured.Unstructured)

	if child.GetDeletionTimestamp() != nil {
		c.onChildDelete(child)
		return
	}

	// If it has no ControllerRef, we don't care.
	// DecoratorController doesn't do adoption since there are no child selectors.
	controllerRef := metav1.GetControllerOf(child)
	if controllerRef == nil {
		return
	}

	parent := c.resolveControllerRef(child.GetNamespace(), controllerRef)
	if parent == nil {
		// The controllerRef isn't a parent we know about.
		return
	}
	c.logger.V(4).Info("Child created or updated", "controller", c.dc, "parent", parent, "child", child)
	c.enqueueParentObject(parent)
}

func (c *decoratorController) onChildUpdate(old, cur interface{}) {
	oldChild := old.(*unstructured.Unstructured)
	curChild := cur.(*unstructured.Unstructured)

	// Don't sync if it's a no-op update (probably a relist/resync).
	// We don't care about resyncs for children; we rely on the parent resync.
	if oldChild.GetResourceVersion() == curChild.GetResourceVersion() {
		return
	}

	// Other than that, we treat updates the same as creates.
	// Level-triggered controllers shouldn't care what the old state was.
	c.onChildAdd(cur)
}

func (c *decoratorController) onChildDelete(obj interface{}) {
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
	parent := c.resolveControllerRef(child.GetNamespace(), controllerRef)
	if parent == nil {
		// The controllerRef isn't a parent we know about.
		return
	}
	c.logger.V(4).Info("DecoratorController child deleted", "controller", c.dc, "parent", parent, "child", child)
	c.enqueueParentObject(parent)
}

func (c *decoratorController) sync(key string) error {
	apiVersion, kind, namespace, name, err := splitParentQueueKey(key)
	if err != nil {
		return err
	}

	c.logger.V(4).Info("Sync", "object", klog.KRef(namespace, name))

	resource := c.resources.GetKind(apiVersion, kind)
	if resource == nil {
		return fmt.Errorf("can't find kind %q in apiVersion %q", kind, apiVersion)
	}

	groupVersion, _ := schema.ParseGroupVersion(apiVersion)
	informer := c.parentInformers.Get(groupVersion.WithResource(resource.Name))
	if informer == nil {
		return fmt.Errorf("no informer for resource %q in apiVersion %q", resource.Name, apiVersion)
	}
	parent, err := common.GetObject(informer, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Swallow the error since there's no point retrying if the parent is gone.
			c.logger.V(4).Info("Parent object has been deleted", "kind", kind, "object", klog.KRef(namespace, name))
			return nil
		} else {
			return err
		}
	}
	err = c.syncParentObject(parent)
	if err != nil {
		c.eventRecorder.Eventf(
			parent,
			corev1.EventTypeWarning,
			events.ReasonSyncError,
			"Sync error: %s", err.Error())
	}
	return err
}

func (c *decoratorController) syncParentObject(parent *unstructured.Unstructured) error {
	// If it doesn't match our selector, and it doesn't have our finalizer, ignore it.
	if !c.parentSelector.Matches(parent) && !controllerutil.ContainsFinalizer(parent, c.finalizer.Name) {
		return nil
	}

	c.logger.V(4).Info("DecoratorController sync", "controller", c.dc, "parent", parent)

	parentClient, err := c.dynClient.Kind(parent.GetAPIVersion(), parent.GetKind())
	if err != nil {
		return fmt.Errorf("can't get client for %v %v/%v: %w", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
	}

	// Before taking any other action, add our finalizer (if desired).
	// This ensures we have a chance to clean up after any action we later take.
	updatedParent, err := c.finalizer.SyncObject(parentClient, parent)
	if err != nil {
		// If we fail to do this, abort before doing anything else and requeue.
		return fmt.Errorf("can't sync finalizer for %v %v/%v: %w", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
	}
	parent = updatedParent

	// Check the finalizer again in case we just removed it.
	if !c.parentSelector.Matches(parent) && !controllerutil.ContainsFinalizer(parent, c.finalizer.Name) {
		return nil
	}

	// List all children belonging to this parent, of the kinds we care about.
	// This only lists the children we created. Existing children are ignored.
	observedChildren, err := c.getChildren(parent)
	if err != nil {
		return err
	}

	relatedObjects, err := c.customize.GetRelatedObjects(parent)
	if err != nil {
		return err
	}

	// Call the sync hook to get the desired annotations and children.
	syncResult, err := c.callHook(parent, observedChildren, relatedObjects)
	if err != nil {
		return err
	}
	desiredChildren := commonv1.MakeRelativeObjectMap(parent, syncResult.Attachments)

	_, err = common.UpdateConfigMapDC(c.dc, c.dynClient, parent, desiredChildren)
	if err != nil {
		return err
	}

	// Enqueue a delayed resync, if requested.
	if syncResult.ResyncAfterSeconds > 0 {
		c.enqueueParentObjectAfter(parent, time.Duration(syncResult.ResyncAfterSeconds*float64(time.Second)))
	}

	// Set desired labels and annotations on parent.
	// Also remove finalizer if requested.
	// Make a copy since parent is from the cache.
	updatedParent = parent.DeepCopy()
	parentLabels := updatedParent.GetLabels()
	if parentLabels == nil {
		parentLabels = make(map[string]string)
	}
	parentAnnotations := updatedParent.GetAnnotations()
	if parentAnnotations == nil {
		parentAnnotations = make(map[string]string)
	}
	parentStatus, _, err := unstructured.NestedMap(updatedParent.Object, "status")
	if err != nil {
		return err
	}
	if syncResult.Status == nil {
		// A null .status in the sync response means leave it unchanged.
		syncResult.Status = parentStatus
	}

	labelsChanged := updateStringMap(parentLabels, syncResult.Labels)
	annotationsChanged := updateStringMap(parentAnnotations, syncResult.Annotations)
	statusChanged := !common.DeepEqual(parentStatus, syncResult.Status)

	// Only do the update if something changed.
	if labelsChanged || annotationsChanged || statusChanged ||
		(syncResult.Finalized && controllerutil.ContainsFinalizer(parent, c.finalizer.Name)) {
		updatedParent.SetLabels(parentLabels)
		updatedParent.SetAnnotations(parentAnnotations)
		if err := unstructured.SetNestedField(updatedParent.Object, syncResult.Status, "status"); err != nil {
			return err
		}

		if statusChanged && parentClient.HasSubresource("status") {
			// The regular Update below will ignore changes to .status so we do it separately.
			result, err := parentClient.Namespace(parent.GetNamespace()).UpdateStatus(context.TODO(), updatedParent, metav1.UpdateOptions{})
			if err != nil {
				switch {
				case apierrors.IsNotFound(err):
					// Swallow the error since there's no point retrying if the child is gone.
					c.logger.V(4).Info("DecoratorController Failed to sync status, parent object has been deleted", "controller", c.dc, "parent", parent)
					return nil
				case apierrors.IsConflict(err):
					// it is possible that the object was modified after this sync was started, ignore conflict since we will reconcile again
					c.logger.V(4).Info("DecoratorController ignoring update status due to outdated resourceVersion", "controller", c.dc, "parent", parent)
					return nil
				default:
					return fmt.Errorf("can't update status: %w", err)
				}
			}
			// The Update below needs to use the latest ResourceVersion.
			updatedParent.SetResourceVersion(result.GetResourceVersion())
		}

		if syncResult.Finalized {
			controllerutil.RemoveFinalizer(updatedParent, c.finalizer.Name)
		}

		c.logger.V(4).Info("DecoratorController updating", "controller", c.dc, "parent", parent)
		_, err = parentClient.Namespace(parent.GetNamespace()).Update(context.TODO(), updatedParent, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Swallow the error since there's no point retrying if the parent is gone.
				c.logger.V(4).Info("DecoratorController Failed to sync, parent object has been deleted", "controller", c.dc, "parent", parent)
				return nil
			} else if apierrors.IsConflict(err) {
				// it is possible that the object was modified after this sync was started, ignore conflict since we will reconcile again
				c.logger.V(4).Info("DecoratorController ignoring update due to outdated resourceVersion", "controller", c.dc, "parent", parent)
				return nil
			}
			return fmt.Errorf("can't update %v %v/%v: %w", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
		}
	}

	// Add an annotation to all desired children to remember that they were
	// created by this decorator.
	for _, group := range desiredChildren {
		for _, child := range group {
			ann := child.GetAnnotations()
			if ann[decoratorControllerAnnotation] == c.dc.Name {
				continue
			}
			if ann == nil {
				ann = make(map[string]string)
			}
			ann[decoratorControllerAnnotation] = c.dc.Name
			child.SetAnnotations(ann)
		}
	}

	// Reconcile child objects belonging to this parent.
	// Remember manage error, but continue to update status regardless.
	//
	// We only manage children if the parent is "alive" (not pending deletion),
	// or if it's pending deletion and we have a `finalize` hook.
	var manageErr error
	if parent.GetDeletionTimestamp() == nil || c.finalizer.ShouldFinalize(parent) {
		// Reconcile children.
		if err := common.ManageChildren(c.dynClient, c.updateStrategy, parent, observedChildren, desiredChildren); err != nil {
			manageErr = fmt.Errorf("can't reconcile children for %v %v/%v: %w", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
		}
	}

	return manageErr
}

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func (c *decoratorController) getChildren(parent *unstructured.Unstructured) (commonv1.RelativeObjectMap, error) {
	parentUID := parent.GetUID()
	parentNamespace := parent.GetNamespace()
	childMap := make(commonv1.RelativeObjectMap)

	gvks, err := common.GetGvkFromConfigMapDC(c.dc, c.dynClient)
	if err != nil {
		return nil, err
	}
	for _, child := range c.dc.Spec.Attachments {
		gr, err := schema.ParseGroupVersion(child.APIVersion)
		if err != nil {
			return nil, err
		}
		childClient, err := c.dynClient.Resource(child.APIVersion, child.Resource)
		if err != nil {
			return nil, err
		}
		res := commonv1.GroupVersionKind{
			schema.GroupVersionKind{
				Group:   gr.Group,
				Version: gr.Version,
				Kind:    childClient.Kind,
			},
		}

		if !Contains(gvks, res) {
			gvks = append(gvks, res)
		}
	}

	for _, child := range gvks {
		// List all objects of the child kind in the parent object's namespace,
		// or in all namespaces if the parent is cluster-scoped.
		groupVersion := child.GroupVersion()
		childClient, err := c.dynClient.Kind(groupVersion.String(), child.Kind)
		if err != nil {
			return nil, err
		}
		resource := childClient.GroupVersionResource()
		informer := c.childInformers.Get(resource)

		if informer == nil {
			informer, err = c.dynInformers.Resource(groupVersion.String(), resource.Resource)
			if err != nil {
				return nil, fmt.Errorf("can't create informer for child resource: %w", err)
			}
			c.childInformers.Set(groupVersion.WithResource(resource.Resource), informer)
		}
		var all []*unstructured.Unstructured
		if parentNamespace != "" {
			all, err = informer.Lister().Namespace(parentNamespace).List(labels.Everything())
		} else {
			all, err = informer.Lister().List(labels.Everything())
		}
		if err != nil {
			return nil, fmt.Errorf("can't list children for resource %q in apiVersion %q: %w", resource.Resource, groupVersion.String(), err)
		}

		// Always include the requested groups, even if there are no entries.
		apiResource := c.resources.Get(groupVersion.String(), resource.Resource)
		if apiResource == nil {
			return nil, fmt.Errorf("can't find resource %q in apiVersion %q", resource.Resource, groupVersion.String())
		}
		childMap.InitGroup(apiResource.GroupVersionKind())

		// Take only the objects that belong to this parent,
		// and that were created by this decorator.
		for _, obj := range all {
			controllerRef := metav1.GetControllerOf(obj)
			if controllerRef == nil || controllerRef.UID != parentUID {
				continue
			}
			if obj.GetAnnotations()[decoratorControllerAnnotation] != c.dc.Name {
				continue
			}
			childMap.Insert(parent, obj)
		}
	}
	return childMap, nil
}

type updateStrategyMap map[string]*v1alpha1.DecoratorControllerAttachmentUpdateStrategy

func (m updateStrategyMap) GetMethod(apiGroup, kind string) v1alpha1.ChildUpdateMethod {
	strategy := m.get(apiGroup, kind)
	if strategy == nil || strategy.Method == "" {
		return v1alpha1.ChildUpdateOnDelete
	}
	return strategy.Method
}

func (m updateStrategyMap) get(apiGroup, kind string) *v1alpha1.DecoratorControllerAttachmentUpdateStrategy {
	t := m[updateStrategyMapKey(apiGroup, kind)]
	if t == nil {
		return m[""]
	}
	return t
}

func updateStrategyMapKey(apiGroup, kind string) string {
	return fmt.Sprintf("%s.%s", kind, apiGroup)
}

func makeUpdateStrategyMap(resources *dynamicdiscovery.ResourceMap, dc *v1alpha1.DecoratorController) (updateStrategyMap, error) {
	m := make(updateStrategyMap)
	for _, child := range dc.Spec.Attachments {
		if child.UpdateStrategy != nil && child.UpdateStrategy.Method != v1alpha1.ChildUpdateOnDelete {
			// Map resource name to kind name.
			resource := resources.Get(child.APIVersion, child.Resource)
			if resource == nil {
				return nil, fmt.Errorf("can't find child resource %q in %v", child.Resource, child.APIVersion)
			}
			// Ignore API version.
			apiGroup, _ := common.ParseAPIVersion(child.APIVersion)
			key := updateStrategyMapKey(apiGroup, resource.Kind)
			m[key] = child.UpdateStrategy
		}
	}
	m[""] = &v1alpha1.DecoratorControllerAttachmentUpdateStrategy{
		Method: dc.Spec.DefaultUpdateStrategy.Method,
	}
	return m, nil
}

func parentQueueKey(obj interface{}) (string, error) {
	switch o := obj.(type) {
	case cache.DeletedFinalStateUnknown:
		return o.Key, nil
	case cache.ExplicitKey:
		return string(o), nil
	case *unstructured.Unstructured:
		return fmt.Sprintf("%s:%s:%s:%s", o.GetAPIVersion(), o.GetKind(), o.GetNamespace(), o.GetName()), nil
	default:
		return "", fmt.Errorf("can't get key for object of type %T; expected *unstructured.Unstructured", obj)
	}
}

func splitParentQueueKey(key string) (apiVersion, kind, namespace, name string, err error) {
	parts := strings.SplitN(key, ":", 4)
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("invalid parent key: %q", key)
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}

func updateStringMap(dest map[string]string, updates map[string]*string) (changed bool) {
	for k, v := range updates {
		if v == nil {
			// nil/null means delete the key
			if _, exists := dest[k]; exists {
				delete(dest, k)
				changed = true
			}
			continue
		}
		// Add/Update the key.
		oldValue, exists := dest[k]
		if !exists || oldValue != *v {
			dest[k] = *v
			changed = true
		}
	}
	return changed
}
