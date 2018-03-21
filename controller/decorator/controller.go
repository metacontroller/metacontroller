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
	"fmt"
	"strings"
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

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	"k8s.io/metacontroller/controller/common"
	dynamicclientset "k8s.io/metacontroller/dynamic/clientset"
	dynamicdiscovery "k8s.io/metacontroller/dynamic/discovery"
	dynamicinformer "k8s.io/metacontroller/dynamic/informer"
	k8s "k8s.io/metacontroller/third_party/kubernetes"
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
}

func newDecoratorController(resources *dynamicdiscovery.ResourceMap, dynClient *dynamicclientset.Clientset, dynInformers *dynamicinformer.SharedInformerFactory, dc *v1alpha1.DecoratorController) (controller *decoratorController, newErr error) {
	c := &decoratorController{
		dc:              dc,
		resources:       resources,
		dynClient:       dynClient,
		parentKinds:     make(common.GroupKindMap),
		parentInformers: make(common.InformerMap),
		childInformers:  make(common.InformerMap),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DecoratorController-"+dc.Name),
	}

	var err error

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
		c.parentKinds.Set(resource.Group, resource.Kind, resource)
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
			return nil, fmt.Errorf("can't create informer for parent resource: %v", err)
		}
		c.parentInformers.Set(parent.APIVersion, parent.Resource, informer)
	}

	for _, child := range dc.Spec.Attachments {
		informer, err := dynInformers.Resource(child.APIVersion, child.Resource)
		if err != nil {
			return nil, fmt.Errorf("can't create informer for child resource: %v", err)
		}
		c.childInformers.Set(child.APIVersion, child.Resource, informer)
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

		glog.Infof("Starting DecoratorController %v", c.dc.Name)
		defer glog.Infof("Shutting down DecoratorController %v", c.dc.Name)

		// Wait for dynamic client and all informers.
		glog.Infof("Waiting for DecoratorController %v caches to sync", c.dc.Name)
		syncFuncs := make([]cache.InformerSynced, 0, 1+len(c.dc.Spec.Resources)+len(c.dc.Spec.Attachments))
		for _, informer := range c.parentInformers {
			syncFuncs = append(syncFuncs, informer.Informer().HasSynced)
		}
		for _, informer := range c.childInformers {
			syncFuncs = append(syncFuncs, informer.Informer().HasSynced)
		}
		if !k8s.WaitForCacheSync(c.dc.Name, c.stopCh, syncFuncs...) {
			// We wait forever unless Stop() is called, so this isn't an error.
			glog.Warningf("DecoratorController %v cache sync never finished", c.dc.Name)
			return
		}

		// 5 workers ought to be enough for anyone.
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
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

	err := c.sync(key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to sync %v %q: %v", c.dc.Name, key, err))
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	return true
}

func (c *decoratorController) enqueueParentObject(obj interface{}) {
	// If the parent doesn't match our selector, or if it's being deleted,
	// we don't care about it.
	if parent, ok := obj.(*unstructured.Unstructured); ok {
		if !c.parentSelector.Matches(parent) || parent.GetDeletionTimestamp() != nil {
			return
		}
	}

	key, err := parentQueueKey(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	c.queue.Add(key)
}

func (c *decoratorController) updateParentObject(old, cur interface{}) {
	// TODO(enisoc): Is there any way to avoid resyncing after our own updates?
	c.enqueueParentObject(cur)
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (c *decoratorController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *unstructured.Unstructured {
	// Is the controllerRef pointing to one of the parent resources we care about?
	// Only look at the group and kind; it doesn't matter if the controller uses
	// a different version than we do.
	apiGroup, _ := common.ParseAPIVersion(controllerRef.APIVersion)
	resource := c.parentKinds.Get(apiGroup, controllerRef.Kind)
	if resource == nil {
		// It's not one of the resources we care about.
		return nil
	}
	// Get the lister for this resource.
	informer := c.parentInformers.Get(resource.APIVersion, resource.Name)
	if informer == nil {
		return nil
	}
	// We can't look up by UID, so look up by Name and then verify UID.
	parent, err := informer.Lister().Get(namespace, controllerRef.Name)
	if err != nil {
		return nil
	}
	if parent.GetUID() != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	if !c.parentSelector.Matches(parent) {
		// If the parent doesn't match our selector, we don't care about it.
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
	glog.V(4).Infof("DecoratorController %v: %v %v/%v: child %v %v created or updated", c.dc.Name, parent.GetKind(), parent.GetNamespace(), parent.GetName(), child.GetKind(), child.GetName())
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
	glog.V(4).Infof("DecoratorController %v: %v %v/%v: child %v %v deleted", c.dc.Name, parent.GetKind(), parent.GetNamespace(), parent.GetName(), child.GetKind(), child.GetName())
	c.enqueueParentObject(parent)
}

func (c *decoratorController) sync(key string) error {
	apiVersion, kind, namespace, name, err := splitParentQueueKey(key)
	if err != nil {
		return err
	}

	resource := c.resources.GetKind(apiVersion, kind)
	if resource == nil {
		return fmt.Errorf("can't find kind %q in apiVersion %q", kind, apiVersion)
	}
	informer := c.parentInformers.Get(apiVersion, resource.Name)
	if informer == nil {
		return fmt.Errorf("no informer for resource %q in apiVersion %q", resource.Name, apiVersion)
	}
	parent, err := informer.Lister().Get(namespace, name)
	if apierrors.IsNotFound(err) {
		// Swallow the error since there's no point retrying if the parent is gone.
		glog.V(4).Infof("%v %v/%v has been deleted", kind, namespace, name)
		return nil
	}
	if err != nil {
		return err
	}
	return c.syncParentObject(parent)
}

func (c *decoratorController) syncParentObject(parent *unstructured.Unstructured) error {
	// If it doesn't match our selector, or is being deleted, ignore it.
	if !c.parentSelector.Matches(parent) || parent.GetDeletionTimestamp() != nil {
		return nil
	}

	glog.V(4).Infof("DecoratorController %v: sync %v %v/%v", c.dc.Name, parent.GetKind(), parent.GetNamespace(), parent.GetName())

	// List all children belonging to this parent, of the kinds we care about.
	// This only lists the children we created. Existing children are ignored.
	observedChildren, err := c.getChildren(parent)
	if err != nil {
		return err
	}

	var desiredChildren common.ChildMap

	// Call the sync hook to get the desired annotations and children.
	syncRequest := &syncHookRequest{
		Controller:  c.dc,
		Object:      parent,
		Attachments: observedChildren,
	}
	syncResult, err := callSyncHook(c.dc, syncRequest)
	if err != nil {
		return err
	}
	desiredChildren = common.MakeChildMap(syncResult.Attachments)

	// Set desired labels and annotations on parent.
	// Make a copy since parent is from the cache.
	updatedParent := parent.DeepCopy()
	parentLabels := updatedParent.GetLabels()
	if parentLabels == nil {
		parentLabels = make(map[string]string)
	}
	parentAnnotations := updatedParent.GetAnnotations()
	if parentAnnotations == nil {
		parentAnnotations = make(map[string]string)
	}
	labelsChanged := updateStringMap(parentLabels, syncResult.Labels)
	annotationsChanged := updateStringMap(parentAnnotations, syncResult.Annotations)
	if labelsChanged || annotationsChanged {
		updatedParent.SetLabels(parentLabels)
		updatedParent.SetAnnotations(parentAnnotations)

		parentClient, err := c.dynClient.Kind(parent.GetAPIVersion(), parent.GetKind(), parent.GetNamespace())
		if err != nil {
			return fmt.Errorf("can't update %v %v/%v: %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
		}
		glog.V(4).Infof("DecoratorController %v: updating %v %v/%v", c.dc.Name, parent.GetKind(), parent.GetNamespace(), parent.GetName())
		_, err = parentClient.Update(updatedParent)
		if err != nil {
			return fmt.Errorf("can't update %v %v/%v: %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
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
	var manageErr error
	if parent.GetDeletionTimestamp() == nil {
		// Reconcile children.
		if err := common.ManageChildren(c.dynClient, c.updateStrategy, parent, observedChildren, desiredChildren); err != nil {
			manageErr = fmt.Errorf("can't reconcile children for %v %v/%v: %v", parent.GetKind(), parent.GetNamespace(), parent.GetName(), err)
		}
	}

	return manageErr
}

func (c *decoratorController) getChildren(parent *unstructured.Unstructured) (common.ChildMap, error) {
	parentUID := parent.GetUID()
	namespace := parent.GetNamespace()
	childMap := make(common.ChildMap)

	for _, child := range c.dc.Spec.Attachments {
		// List all objects of the child kind in the parent object's namespace.
		informer := c.childInformers.Get(child.APIVersion, child.Resource)
		if informer == nil {
			return nil, fmt.Errorf("no informer for resource %q in apiVersion %q", child.Resource, child.APIVersion)
		}
		all, err := informer.Lister().ListNamespace(namespace, labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("can't list children for resource %q in apiVersion %q: %v", child.Resource, child.APIVersion, err)
		}

		// Always include the requested groups, even if there are no entries.
		resource := c.resources.Get(child.APIVersion, child.Resource)
		if resource == nil {
			return nil, fmt.Errorf("can't find resource %q in apiVersion %q", child.Resource, child.APIVersion)
		}
		childMap.InitGroup(child.APIVersion, resource.Kind)

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
			// Add children to map by name.
			// Note that we limit each parent to only working within its own namespace.
			childMap.Insert(obj)
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
	return m[updateStrategyMapKey(apiGroup, kind)]
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
