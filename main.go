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
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	mcclientset "k8s.io/metacontroller/client/generated/clientset/internalclientset"
	mcinformers "k8s.io/metacontroller/client/generated/informer/externalversions"
	"k8s.io/metacontroller/controller/composite"
	"k8s.io/metacontroller/controller/decorator"
	dynamicclientset "k8s.io/metacontroller/dynamic/clientset"
	dynamicdiscovery "k8s.io/metacontroller/dynamic/discovery"
	dynamicinformer "k8s.io/metacontroller/dynamic/informer"
)

var (
	discoveryInterval = flag.Duration("discovery-interval", 30*time.Second, "How often to refresh discovery cache to pick up newly-installed resources")
	informerRelist    = flag.Duration("cache-flush-interval", 30*time.Minute, "How often to flush local caches and relist objects from the API server")
)

type controller interface {
	Start()
	Stop()
}

func main() {
	flag.Parse()

	glog.Infof("Discovery cache flush interval: %v", *discoveryInterval)
	glog.Infof("API server object cache flush interval: %v", *informerRelist)

	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}

	// Periodically refresh discovery to pick up newly-installed resources.
	dc := discovery.NewDiscoveryClientForConfigOrDie(config)
	resources := dynamicdiscovery.NewResourceMap(dc)
	// We don't care about stopping this cleanly since it has no external effects.
	resources.Start(*discoveryInterval)

	// Create informerfactory for metacontroller api objects.
	mcClient, err := mcclientset.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Can't create client for api %s: %v", v1alpha1.SchemeGroupVersion, err)
	}
	mcInformerFactory := mcinformers.NewSharedInformerFactory(mcClient, *informerRelist)

	// Create dynamic clientset (factory for dynamic clients).
	dynClient := dynamicclientset.New(config, resources)
	// Create dynamic informer factory (for sharing dynamic informers).
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, *informerRelist)

	// Start metacontrollers (controllers that spawn controllers).
	// Each one requests the informers it needs from the factory.
	controllers := []controller{
		composite.NewMetacontroller(resources, dynClient, dynInformers, mcInformerFactory, mcClient),
		decorator.NewMetacontroller(resources, dynClient, dynInformers, mcInformerFactory),
	}

	// Start all requested informers.
	// We don't care about stopping this cleanly since it has no external effects.
	mcInformerFactory.Start(nil)

	// Start all controllers.
	for _, c := range controllers {
		c.Start()
	}

	// On SIGTERM, stop all controllers gracefully.
	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigchan
	glog.Infof("Received %q signal. Shutting down...", sig)

	var wg sync.WaitGroup
	for _, c := range controllers {
		wg.Add(1)
		go func(c controller) {
			defer wg.Done()
			c.Stop()
		}(c)
	}
	wg.Wait()
}
