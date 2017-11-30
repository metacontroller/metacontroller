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
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

func resyncAll(config *rest.Config) error {
	// Discover all supported resources.
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("can't create discovery client: %v", err)
	}
	resources, err := dc.ServerResources()
	if err != nil {
		return fmt.Errorf("can't discover resources: %v", err)
	}
	clientset := newDynamicClientset(config, newResourceMap(resources))

	// Sync all CompositeController objects.
	if err := syncAllCompositeControllers(clientset); err != nil {
		glog.Errorf("can't sync CompositeControllers: %v", err)
	}

	// Sync all InitializerController objects.
	if err := syncAllInitializerControllers(clientset); err != nil {
		glog.Errorf("can't sync InitializerControllers: %v", err)
	}

	return nil
}

func main() {
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	for {
		if err := resyncAll(config); err != nil {
			glog.Errorf("sync: %v", err)
		}

		time.Sleep(5 * time.Second)
	}
}
