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
	mclisters "metacontroller.io/client/generated/lister/metacontroller/v1alpha1"
	"metacontroller.io/controller/common"
)

type Metacontroller struct {
	resources    *dynamicdiscovery.ResourceMap
	dynClient    *dynamicclientset.Clientset
	dynInformers *dynamicinformer.SharedInformerFactory

	eventRecorder record.EventRecorder

	dcLister   mclisters.DecoratorControllerLister
	dcInformer cache.SharedIndexInformer

	queue                workqueue.RateLimitingInterface
	decoratorControllers map[string]*decoratorController

	stopCh, doneCh chan struct{}

	numWorkers int
}

func NewMetacontroller(controllerContext common.ControllerContext, numWorkers int) *Metacontroller {
	mc := &Metacontroller{
		resources:     controllerContext.Resources,
		dynClient:     controllerContext.DynClient,
		dynInformers:  controllerContext.DynInformers,
		eventRecorder: controllerContext.EventRecorder,

		dcLister:   controllerContext.McInformerFactory.Metacontroller().V1alpha1().DecoratorControllers().Lister(),
		dcInformer: controllerContext.McInformerFactory.Metacontroller().V1alpha1().DecoratorControllers().Informer(),

		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DecoratorController"),
		decoratorControllers: make(map[string]*decoratorController),

		numWorkers: numWorkers,
	}

	mc.dcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    mc.enqueueDecoratorController,
		UpdateFunc: mc.updateDecoratorController,
		DeleteFunc: mc.enqueueDecoratorController,
	})

	return mc
}

func (mc *Metacontroller) Start() {
	mc.stopCh = make(chan struct{})
	mc.doneCh = make(chan struct{})

	go func() {
		defer close(mc.doneCh)
		defer utilruntime.HandleCrash()

		klog.InfoS("Starting DecoratorController metacontroller")
		defer klog.InfoS("Shutting down DecoratorController metacontroller")

		if !cache.WaitForNamedCacheSync("DecoratorController", mc.stopCh, mc.dcInformer.HasSynced) {
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
	for _, c := range mc.decoratorControllers {
		wg.Add(1)
		go func(c *decoratorController) {
			defer wg.Done()
			c.Stop()
		}(c)
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
		utilruntime.HandleError(fmt.Errorf("failed to sync DecoratorController %q: %v", key, err))
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

	klog.V(4).InfoS("Sync DecoratorController", "name", name)

	dc, err := mc.dcLister.Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(4).InfoS("DecoratorController has been deleted", "name", name)
		// Stop and remove the controller if it exists.
		if c, ok := mc.decoratorControllers[name]; ok {
			c.Stop()
			defer c.eventRecorder.Eventf(
				c.dc,
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", c.dc.Name)
			delete(mc.decoratorControllers, name)
		}
		return nil
	}
	if err != nil {
		mc.eventRecorder.Eventf(
			dc,
			v1.EventTypeNormal,
			events.ReasonSyncError,
			"[%s] sync error - %s", dc.Name, err)
		return err
	}
	return mc.syncDecoratorController(dc)
}

func (mc *Metacontroller) syncDecoratorController(dc *v1alpha1.DecoratorController) error {
	if c, ok := mc.decoratorControllers[dc.Name]; ok {
		// The controller was already started.
		if apiequality.Semantic.DeepEqual(dc.Spec, c.dc.Spec) {
			// Nothing has changed.
			return nil
		}
		// Stop and remove the controller so it can be recreated.
		c.Stop()
		mc.eventRecorder.Eventf(dc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", dc.Name)
		delete(mc.decoratorControllers, dc.Name)
	}

	c, err := newDecoratorController(
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		dc,
		mc.numWorkers,
	)
	if err != nil {
		return err
	}
	c.Start()
	mc.eventRecorder.Eventf(dc, v1.EventTypeNormal, events.ReasonStarted, "Started controller: %s", dc.Name)
	mc.decoratorControllers[dc.Name] = c
	return nil
}

func (mc *Metacontroller) enqueueDecoratorController(obj interface{}) {
	key, err := common.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	mc.queue.Add(key)
}

func (mc *Metacontroller) updateDecoratorController(old, cur interface{}) {
	mc.enqueueDecoratorController(cur)
}
