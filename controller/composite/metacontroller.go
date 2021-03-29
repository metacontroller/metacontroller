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

package composite

import (
	"fmt"
	"sync"

	dynamicclientset "metacontroller.io/dynamic/clientset"
	dynamicdiscovery "metacontroller.io/dynamic/discovery"
	dynamicinformer "metacontroller.io/dynamic/informer"

	"k8s.io/client-go/tools/record"

	v1 "k8s.io/api/core/v1"
	"metacontroller.io/events"

	"k8s.io/klog/v2"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"metacontroller.io/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller.io/client/generated/clientset/internalclientset"
	mclisters "metacontroller.io/client/generated/lister/metacontroller/v1alpha1"
	"metacontroller.io/controller/common"
)

type Metacontroller struct {
	resources     *dynamicdiscovery.ResourceMap
	dynClient     *dynamicclientset.Clientset
	dynInformers  *dynamicinformer.SharedInformerFactory
	eventRecorder record.EventRecorder

	mcClient mcclientset.Interface

	ccLister         mclisters.CompositeControllerLister
	ccInformer       cache.SharedIndexInformer
	revisionLister   mclisters.ControllerRevisionLister
	revisionInformer cache.SharedIndexInformer

	queue             workqueue.RateLimitingInterface
	parentControllers map[string]*parentController

	stopCh, doneCh chan struct{}

	numWorkers int
}

func NewMetacontroller(controllerContext common.ControllerContext, mcClient mcclientset.Interface, numWorkers int) *Metacontroller {
	mc := &Metacontroller{
		resources:     controllerContext.Resources,
		dynClient:     controllerContext.DynClient,
		dynInformers:  controllerContext.DynInformers,
		eventRecorder: controllerContext.EventRecorder,

		mcClient: mcClient,

		ccLister:         controllerContext.McInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Lister(),
		ccInformer:       controllerContext.McInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Informer(),
		revisionLister:   controllerContext.McInformerFactory.Metacontroller().V1alpha1().ControllerRevisions().Lister(),
		revisionInformer: controllerContext.McInformerFactory.Metacontroller().V1alpha1().ControllerRevisions().Informer(),

		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CompositeController"),
		parentControllers: make(map[string]*parentController),

		numWorkers: numWorkers,
	}

	mc.ccInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    mc.enqueueCompositeController,
		UpdateFunc: mc.updateCompositeController,
		DeleteFunc: mc.enqueueCompositeController,
	})

	return mc
}

func (mc *Metacontroller) Start() {
	mc.stopCh = make(chan struct{})
	mc.doneCh = make(chan struct{})

	go func() {
		defer close(mc.doneCh)
		defer utilruntime.HandleCrash()

		klog.InfoS("Starting CompositeController metacontroller")
		defer klog.InfoS("Shutting down CompositeController metacontroller")

		if !cache.WaitForNamedCacheSync("CompositeController", mc.stopCh, mc.ccInformer.HasSynced) {
			return
		}

		// In the metacontroller, we are only responsible for starting/stopping
		// the actual controllers, so a single worker should be enough.
		for mc.processNextWorkItem() {
		}
	}()
}

func (mc *Metacontroller) Stop() {
	// Stop metacontroller first so there's no more changes to controllers.
	close(mc.stopCh)
	mc.queue.ShutDown()
	<-mc.doneCh

	// Stop all controllers.
	var wg sync.WaitGroup
	for _, pc := range mc.parentControllers {
		wg.Add(1)
		go func(pc *parentController) {
			defer wg.Done()
			pc.Stop()
		}(pc)
	}
	wg.Wait()
}

func (mc *Metacontroller) processNextWorkItem() bool {
	key, quit := mc.queue.Get()
	if quit {
		return false
	}
	defer mc.queue.Done(key)

	err := mc.sync(key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to sync CompositeController %q: %v", key, err))
		mc.queue.AddRateLimited(key)
		return true
	}

	mc.queue.Forget(key)
	return true
}

func (mc *Metacontroller) sync(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	klog.V(4).InfoS("Sync CompositeController", "name", name)

	cc, err := mc.ccLister.Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(4).InfoS("CompositeController has been deleted", "name", name)
		// Stop and remove the controller if it exists.
		if pc, ok := mc.parentControllers[name]; ok {
			pc.Stop()
			defer pc.eventRecorder.Eventf(
				pc.cc,
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", pc.cc.Name)
			delete(mc.parentControllers, name)
		}
		return nil
	}

	if err != nil {
		mc.eventRecorder.Eventf(
			cc, v1.EventTypeNormal,
			events.ReasonSyncError,
			"[%s] Sync error - %s", cc.Name, err)
		return err
	}
	parentClient, err := mc.dynClient.Resource(cc.Spec.ParentResource.APIVersion, cc.Spec.ParentResource.Resource)
	if err != nil {
		return err
	}
	if found := parentClient.APIResource.HasSubresource("status"); !found {
		mc.eventRecorder.Eventf(
			cc,
			v1.EventTypeWarning,
			events.ReasonSyncError,
			"[%s] Sync error - ignoring, parent resource %s does not have subresource 'Status' enabled",
			cc.Name,
			parentClient.GroupVersionKind())
		klog.InfoS("Ignoring CompositeController",
			"name", name,
			"reason", "subresource 'Status' not enabled",
			"groupVersionKind", parentClient.GroupVersionKind())
		// returning, as we cannot do anything until 'Status' subresource is added to parent resource
		return nil
	}
	return mc.syncCompositeController(cc)
}

func (mc *Metacontroller) syncCompositeController(cc *v1alpha1.CompositeController) error {
	if pc, ok := mc.parentControllers[cc.Name]; ok {
		// The controller was already started.
		if apiequality.Semantic.DeepEqual(cc.Spec, pc.cc.Spec) {
			// Nothing has changed.
			return nil
		}
		// Stop and remove the controller so it can be recreated.
		pc.Stop()
		mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", cc.Name)
		delete(mc.parentControllers, cc.Name)
	}

	pc, err := newParentController(
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		mc.mcClient,
		mc.revisionLister,
		cc,
		mc.numWorkers)
	if err != nil {
		return err
	}
	pc.Start()
	mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStarted, "Started controller: %s", cc.Name)
	mc.parentControllers[cc.Name] = pc
	return nil
}

func (mc *Metacontroller) enqueueCompositeController(obj interface{}) {
	key, err := common.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	mc.queue.Add(key)
}

func (mc *Metacontroller) updateCompositeController(old, cur interface{}) {
	mc.enqueueCompositeController(cur)
}
