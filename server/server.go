/*
Copyright 2019 Google Inc.

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

package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/0xRLG/ocworkqueue"
	"go.opencensus.io/stats/view"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"

	"metacontroller.io/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller.io/client/generated/clientset/internalclientset"
	mcinformers "metacontroller.io/client/generated/informer/externalversions"
	"metacontroller.io/controller/composite"
	"metacontroller.io/controller/decorator"
	dynamicclientset "metacontroller.io/dynamic/clientset"
	dynamicdiscovery "metacontroller.io/dynamic/discovery"
	dynamicinformer "metacontroller.io/dynamic/informer"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type controller interface {
	Start()
	Stop()
}

func Start(config *rest.Config, discoveryInterval, informerRelist time.Duration) (stop func(), err error) {
	// Periodically refresh discovery to pick up newly-installed resources.
	dc := discovery.NewDiscoveryClientForConfigOrDie(config)
	resources := dynamicdiscovery.NewResourceMap(dc)
	// We don't care about stopping this cleanly since it has no external effects.
	resources.Start(discoveryInterval)

	// Create informer factory for metacontroller API objects.
	mcClient, err := mcclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Can't create client for api %s: %v", v1alpha1.SchemeGroupVersion, err)
	}
	mcInformerFactory := mcinformers.NewSharedInformerFactory(mcClient, informerRelist)

	// Create dynamic clientset (factory for dynamic clients).
	dynClient, err := dynamicclientset.New(config, resources)
	if err != nil {
		return nil, err
	}
	// Create dynamic informer factory (for sharing dynamic informers).
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, informerRelist)

	workqueue.SetProvider(ocworkqueue.MetricsProvider())
	view.Register(ocworkqueue.DefaultViews...)

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

	// Return a function that will stop all controllers.
	return func() {
		var wg sync.WaitGroup
		for _, c := range controllers {
			wg.Add(1)
			go func(c controller) {
				defer wg.Done()
				c.Stop()
			}(c)
		}
		wg.Wait()
	}, nil
}
