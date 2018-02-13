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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	internalclient "k8s.io/metacontroller/client/generated/clientset/internalclientset"
	internalinformers "k8s.io/metacontroller/client/generated/informer/externalversions"
	"k8s.io/metacontroller/controller/composite"
	"k8s.io/metacontroller/controller/initializer"
	dynamicclientset "k8s.io/metacontroller/dynamic/clientset"
	dynamicdiscovery "k8s.io/metacontroller/dynamic/discovery"
)

var (
	discoveryInterval = flag.Duration("discovery-interval", 30*time.Second, "How often to refresh discovery cache to pick up newly-installed resources")
	informerResync    = flag.Duration("informer-resync", 5*time.Minute, "Default resync period for shared informer caches")
)

func startResyncLoop(dynClient *dynamicclientset.Clientset, mcInformerFactory internalinformers.SharedInformerFactory) (cancel func()) {
	stop := make(chan struct{})
	done := make(chan struct{})

	ccLister := mcInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Lister()
	icLister := mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Lister()

	go func() {
		defer close(done)

		// This interval isn't configurable because polling is going away soon.
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// Sync all CompositeController objects.
				if err := composite.SyncAll(dynClient, ccLister); err != nil {
					glog.Errorf("can't sync CompositeControllers: %v", err)
				}

				// Sync all InitializerController objects.
				if err := initializer.SyncAll(dynClient, icLister); err != nil {
					glog.Errorf("can't sync InitializerControllers: %v", err)
				}
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

func main() {
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}

	// Periodically refresh discovery to pick up newly-installed resources.
	dc := discovery.NewDiscoveryClientForConfigOrDie(config)
	resources := dynamicdiscovery.NewResourceMap(dc)
	stopDiscoveryRefresh := resources.Start(*discoveryInterval)
	defer stopDiscoveryRefresh()

	// Create informerfactory for metacontroller api objects.
	mcClient, err := internalclient.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Can't create client for api %s: %v", v1alpha1.SchemeGroupVersion, err)
	}
	mcInformerFactory := internalinformers.NewSharedInformerFactory(mcClient, *informerResync)

	// Initialize informers.
	mcInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Informer()
	mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Informer()
	mcInformerFactory.Start(nil)

	// Wait for the caches to be synced before starting the loop.
	glog.V(2).Info("Waiting for discovery and informer caches to sync")
	if ok := cache.WaitForCacheSync(nil,
		resources.HasSynced,
		mcInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Informer().HasSynced,
		mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Informer().HasSynced,
	); !ok {
		glog.Fatal("Failed to wait for caches to sync")
	}
	glog.V(2).Info("Discovery and informer caches synced")

	// Create dynamic clientset (factory for dynamic clients).
	dynClient := dynamicclientset.New(config, resources)

	// Start polling in the background.
	// TODO(kube-metacontroller#8): Replace with shared, dynamic informers.
	stopResyncLoop := startResyncLoop(dynClient, mcInformerFactory)
	defer stopResyncLoop()

	// On SIGTERM, execute deferred functions to shut down gracefully.
	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigchan
	glog.Infof("Received %q signal. Shutting down...", sig)
}
