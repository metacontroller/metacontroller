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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	internalclient "k8s.io/metacontroller/client/generated/clientset/internalclientset"
	internalinformers "k8s.io/metacontroller/client/generated/informer/externalversions"
	internallisters "k8s.io/metacontroller/client/generated/lister/metacontroller/v1alpha1"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// metaController manages metacontroller API objects.
type metaController struct {
	// listers
	ccLister internallisters.CompositeControllerLister
	icLister internallisters.InitializerControllerLister

	// informer synced functions
	ccSynced, icSynced cache.InformerSynced
}

// New creates an instance of metaController.
func New(mcInformerFactory internalinformers.SharedInformerFactory) *metaController {
	// obtain reference to shared index informers
	ccInformerInterface := mcInformerFactory.Metacontroller().V1alpha1().CompositeControllers()
	icInformerInterface := mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers()

	// create controller
	return &metaController{
		ccInformerInterface.Lister(),
		icInformerInterface.Lister(),
		ccInformerInterface.Informer().HasSynced,
		icInformerInterface.Informer().HasSynced,
	}
}

func resyncAll(config *rest.Config, mc *metaController) error {
	// Discover all supported resources.
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("can't create discovery client: %v", err)
	}
	resources, err := dc.ServerResources()
	if err != nil {
		return fmt.Errorf("can't discover resources: %v", err)
	}
	dynClient := newDynamicClientset(config, newResourceMap(resources))

	// Sync all CompositeController objects.
	if err := syncAllCompositeControllers(dynClient, mc); err != nil {
		glog.Errorf("can't sync CompositeControllers: %v", err)
	}

	// Sync all InitializerController objects.
	if err := syncAllInitializerControllers(dynClient, mc); err != nil {
		glog.Errorf("can't sync InitializerControllers: %v", err)
	}

	return nil
}

func main() {
	flag.Parse()

	// Create OS signal channels.
	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigs
		glog.V(2).Info("Stopping informers")
		close(stop)
		os.Exit(0)
	}()

	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}

	// Create informerfactory for metacontroller api objects.
	mcClient, err := internalclient.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Can't create client for api %s: %v", v1alpha1.SchemeGroupVersion, err)
	}
	mcInformerFactory := internalinformers.NewSharedInformerFactory(mcClient, 5*time.Minute)

	// Create metacontroller.
	mc := New(mcInformerFactory)

	// Initialize requested informers.
	mcInformerFactory.Start(stop)

	// Wait for the caches to be synced before starting the loop.
	glog.V(2).Info("Waiting for informers caches to sync")
	if ok := cache.WaitForCacheSync(stop, mc.ccSynced, mc.icSynced); !ok {
		glog.Fatal("Failed to wait for caches to sync")
	}
	glog.V(2).Info("Informers caches synced")

	for {
		if err := resyncAll(config, mc); err != nil {
			glog.Errorf("sync: %v", err)
		}

		time.Sleep(5 * time.Second)
	}
}
