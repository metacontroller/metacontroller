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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"metacontroller.io/controller/decorator"
	"metacontroller.io/options"

	"k8s.io/client-go/discovery"
	"metacontroller.io/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller.io/client/generated/clientset/internalclientset"
	mcinformers "metacontroller.io/client/generated/informer/externalversions"
	"metacontroller.io/controller/composite"
	dynamicclientset "metacontroller.io/dynamic/clientset"
	dynamicdiscovery "metacontroller.io/dynamic/discovery"
	dynamicinformer "metacontroller.io/dynamic/informer"
	"metacontroller.io/events"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	v1alpha1.AddToScheme(scheme)
}

type controller interface {
	Start()
	Stop()
}

func Start(options options.Options) (stop func(), err error) {
	// Periodically refresh discovery to pick up newly-installed resources.
	dc := discovery.NewDiscoveryClientForConfigOrDie(options.Config)
	resources := dynamicdiscovery.NewResourceMap(dc)
	// We don't care about stopping this cleanly since it has no external effects.
	resources.Start(options.DiscoveryInterval)

	// Create informer factory for metacontroller API objects.
	mcClient, err := mcclientset.NewForConfig(options.Config)
	if err != nil {
		return nil, fmt.Errorf("can't create client for api %s: %v", v1alpha1.SchemeGroupVersion, err)
	}
	mcInformerFactory := mcinformers.NewSharedInformerFactory(mcClient, options.InformerRelist)

	// Create dynamic clientset (factory for dynamic clients).
	dynClient, err := dynamicclientset.New(options.Config, resources)
	if err != nil {
		return nil, err
	}
	// Create dynamic informer factory (for sharing dynamic informers).
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, options.InformerRelist)

	// Start metacontrollers (controllers that spawn controllers).
	// Each one requests the informers it needs from the factory.
	broadcaster, err := events.NewBroadcaster(options.Config, options.CorrelatorOptions)
	if err != nil {
		return nil, err
	}
	recorder := broadcaster.NewRecorder(scheme, corev1.EventSource{Component: "metacontroller"})
	controllers := []controller{
		composite.NewMetacontroller(resources, dynClient, dynInformers, mcInformerFactory, mcClient, options.Workers, recorder),
		decorator.NewMetacontroller(resources, dynClient, dynInformers, mcInformerFactory, options.Workers, recorder),
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
		time.Sleep(1 * time.Second)
		broadcaster.Shutdown()
	}, nil
}
