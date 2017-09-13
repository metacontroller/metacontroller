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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
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

	// Sync all LambdaController objects.
	lcClient, err := clientset.Resource(v1alpha1.SchemeGroupVersion.String(), "lambdacontrollers", "")
	if err != nil {
		return err
	}
	obj, err := lcClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("can't list LambdaControllers: %v", err)
	}
	lcList := obj.(*unstructured.UnstructuredList)

	for i := range lcList.Items {
		data, err := json.Marshal(&lcList.Items[i])
		if err != nil {
			glog.Errorf("can't marshal LambdaController: %v")
			continue
		}
		lc := &v1alpha1.LambdaController{}
		if err := json.Unmarshal(data, lc); err != nil {
			glog.Errorf("can't unmarshal LambdaController: %v", err)
			continue
		}
		if err := syncLambdaController(clientset, lc); err != nil {
			glog.Errorf("syncLambdaController: %v", err)
			continue
		}
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
