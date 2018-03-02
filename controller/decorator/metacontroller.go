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

	"github.com/golang/glog"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	mcinformers "k8s.io/metacontroller/client/generated/informer/externalversions"
	mclisters "k8s.io/metacontroller/client/generated/lister/metacontroller/v1alpha1"
	"k8s.io/metacontroller/controller/common"
	dynamicclientset "k8s.io/metacontroller/dynamic/clientset"
	dynamicdiscovery "k8s.io/metacontroller/dynamic/discovery"
	dynamicinformer "k8s.io/metacontroller/dynamic/informer"
	k8s "k8s.io/metacontroller/third_party/kubernetes"
)

type Metacontroller struct {
	resources    *dynamicdiscovery.ResourceMap
	dynClient    *dynamicclientset.Clientset
	dynInformers *dynamicinformer.SharedInformerFactory

	dcLister   mclisters.DecoratorControllerLister
	dcInformer cache.SharedIndexInformer

	queue                workqueue.RateLimitingInterface
	decoratorControllers map[string]*decoratorController

	stopCh, doneCh chan struct{}
}

func NewMetacontroller(resources *dynamicdiscovery.ResourceMap, dynClient *dynamicclientset.Clientset, dynInformers *dynamicinformer.SharedInformerFactory, mcInformerFactory mcinformers.SharedInformerFactory) *Metacontroller {
	mc := &Metacontroller{
		resources:    resources,
		dynClient:    dynClient,
		dynInformers: dynInformers,

		dcLister:   mcInformerFactory.Metacontroller().V1alpha1().DecoratorControllers().Lister(),
		dcInformer: mcInformerFactory.Metacontroller().V1alpha1().DecoratorControllers().Informer(),

		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DecoratorController"),
		decoratorControllers: make(map[string]*decoratorController),
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

		glog.Info("Starting DecoratorController metacontroller")
		defer glog.Info("Shutting down DecoratorController metacontroller")

		if !k8s.WaitForCacheSync("DecoratorController", mc.stopCh, mc.dcInformer.HasSynced) {
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

	glog.V(4).Infof("sync DecoratorController %v", name)

	dc, err := mc.dcLister.Get(name)
	if apierrors.IsNotFound(err) {
		glog.V(4).Infof("DecoratorController %v has been deleted", name)
		// Stop and remove the controller if it exists.
		if c, ok := mc.decoratorControllers[name]; ok {
			c.Stop()
			delete(mc.decoratorControllers, name)
		}
		return nil
	}
	if err != nil {
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
		delete(mc.decoratorControllers, dc.Name)
	}

	c, err := newDecoratorController(mc.resources, mc.dynClient, mc.dynInformers, dc)
	if err != nil {
		return err
	}
	c.Start()
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
