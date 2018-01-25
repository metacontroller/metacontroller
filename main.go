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

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func resyncAll(config *rest.Config, mcInformerFactory internalinformers.SharedInformerFactory) error {
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
	ccLister := mcInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Lister()
	if err := syncAllCompositeControllers(dynClient, ccLister); err != nil {
		glog.Errorf("can't sync CompositeControllers: %v", err)
	}

	// Sync all InitializerController objects.
	icLister := mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Lister()
	if err := syncAllInitializerControllers(dynClient, icLister); err != nil {
		glog.Errorf("can't sync InitializerControllers: %v", err)
	}

	return nil
}

func main() {
	flag.Parse()

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

	// Initialize informers.
	mcInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Informer()
	mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Informer()
	mcInformerFactory.Start(nil)

	// Wait for the caches to be synced before starting the loop.
	glog.V(2).Info("Waiting for informers caches to sync")
	if ok := cache.WaitForCacheSync(nil,
		mcInformerFactory.Metacontroller().V1alpha1().CompositeControllers().Informer().HasSynced,
		mcInformerFactory.Metacontroller().V1alpha1().InitializerControllers().Informer().HasSynced,
	); !ok {
		glog.Fatal("Failed to wait for caches to sync")
	}
	glog.V(2).Info("Informers caches synced")

	// Close 'stop' to begin shutdown. Wait for 'done' before terminating.
	stop := make(chan struct{})
	done := make(chan struct{})

	// Start polling in the background.
	// TODO(kube-metacontroller#8): Replace with shared, dynamic informers.
	go func() {
		defer close(done)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := resyncAll(config, mcInformerFactory); err != nil {
					glog.Errorf("sync: %v", err)
				}
			}
		}
	}()

	// On SIGTERM, wait for a complete resync attempt to finish gracefully.
	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigchan
	glog.Infof("Received %q signal. Shutting down...", sig)
	close(stop)
	<-done
}
