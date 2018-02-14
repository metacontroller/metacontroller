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

package initializer

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
	k8s "k8s.io/metacontroller/third_party/kubernetes"
)

type Metacontroller struct {
	dynClient  *dynamicclientset.Clientset
	icLister   mclisters.InitializerControllerLister
	icInformer cache.SharedIndexInformer

	queue                  workqueue.RateLimitingInterface
	initializerControllers map[string]*initializerController

	stopCh, doneCh chan struct{}
}

func NewMetacontroller(dynClient *dynamicclientset.Clientset, mcInformerFactory mcinformers.SharedInformerFactory) *Metacontroller {
	mc := &Metacontroller{
		dynClient:  dynClient,
		icLister:   mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Lister(),
		icInformer: mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Informer(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "InitializerController"),
		initializerControllers: make(map[string]*initializerController),
	}

	mc.icInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    mc.enqueueInitializerController,
		UpdateFunc: mc.updateInitializerController,
		DeleteFunc: mc.enqueueInitializerController,
	})

	return mc
}

func (mc *Metacontroller) Start() {
	mc.stopCh = make(chan struct{})
	mc.doneCh = make(chan struct{})

	go func() {
		defer close(mc.doneCh)
		defer utilruntime.HandleCrash()

		glog.Info("Starting InitializerController metacontroller")
		defer glog.Info("Shutting down InitializerController metacontroller")

		if !k8s.WaitForCacheSync("InitializerController", mc.stopCh, mc.icInformer.HasSynced) {
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
	for _, c := range mc.initializerControllers {
		wg.Add(1)
		go func(c *initializerController) {
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
		utilruntime.HandleError(fmt.Errorf("failed to sync InitializerController %q: %v", key, err))
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

	glog.V(4).Infof("sync InitializerController %v", name)

	cc, err := mc.icLister.Get(name)
	if apierrors.IsNotFound(err) {
		glog.V(4).Infof("InitializerController %v has been deleted", name)
		// Stop and remove the controller if it exists.
		if c, ok := mc.initializerControllers[name]; ok {
			c.Stop()
			delete(mc.initializerControllers, name)
		}
		return nil
	}
	if err != nil {
		return err
	}
	return mc.syncInitializerController(cc)
}

func (mc *Metacontroller) syncInitializerController(ic *v1alpha1.InitializerController) error {
	if c, ok := mc.initializerControllers[ic.Name]; ok {
		// The controller was already started.
		if apiequality.Semantic.DeepEqual(ic.Spec, c.ic.Spec) {
			// Nothing has changed.
			return nil
		}
		// Stop and remove the controller so it can be recreated.
		c.Stop()
		delete(mc.initializerControllers, ic.Name)
	}

	c, err := newInitializerController(mc.dynClient, ic)
	if err != nil {
		return err
	}
	c.Start()
	mc.initializerControllers[ic.Name] = c
	return nil
}

func (mc *Metacontroller) enqueueInitializerController(obj interface{}) {
	key, err := common.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	mc.queue.Add(key)
}

func (mc *Metacontroller) updateInitializerController(old, cur interface{}) {
	mc.enqueueInitializerController(cur)
}
