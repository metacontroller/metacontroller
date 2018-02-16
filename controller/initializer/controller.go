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

package initializer

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
	dynamicclientset "k8s.io/metacontroller/dynamic/clientset"
	k8s "k8s.io/metacontroller/third_party/kubernetes"
)

type initializerController struct {
	ic        *v1alpha1.InitializerController
	dynClient *dynamicclientset.Clientset

	stopCh, doneCh chan struct{}
}

func newInitializerController(dynClient *dynamicclientset.Clientset, ic *v1alpha1.InitializerController) (*initializerController, error) {
	return &initializerController{
		ic:        ic,
		dynClient: dynClient,
	}, nil
}

func (c *initializerController) Start() {
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})

	go func() {
		defer close(c.doneCh)

		glog.Infof("Starting %v InitializerController", c.ic.Name)
		defer glog.Infof("Shutting down %v InitializerController", c.ic.Name)

		// Wait for dynamic client to populate discovery cache.
		if !k8s.WaitForCacheSync(c.ic.Name+"-initializer-controller", c.stopCh, c.dynClient.HasSynced) {
			return
		}

		// Start polling in the background.
		// This interval isn't configurable because polling is going away soon.
		// TODO(kube-metacontroller#8): Replace with shared, dynamic informers.
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				if err := c.sync(); err != nil {
					utilruntime.HandleError(fmt.Errorf("can't sync %v initializer controller: %v", c.ic.Name, err))
				}
			}
		}
	}()
}

func (c *initializerController) Stop() {
	close(c.stopCh)
	<-c.doneCh
}

func (c *initializerController) sync() error {
	var errs []error
	// Find all uninitialized objects of the requested kinds.
	for _, rule := range c.ic.Spec.UninitializedResources {
		if err := c.initializeResource(rule.APIVersion, rule.Resource); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (c *initializerController) initializeResource(apiVersion, resourceName string) error {
	// List all objects of the given kind in all namespaces.
	client, err := c.dynClient.Resource(apiVersion, resourceName, "")
	if err != nil {
		return err
	}
	obj, err := client.List(metav1.ListOptions{IncludeUninitialized: true})
	if err != nil {
		return fmt.Errorf("can't list uninitialized %v objects: %v", client.Kind(), err)
	}
	list := obj.(*unstructured.UnstructuredList)

	var errs []error
	for i := range list.Items {
		uninitialized := &list.Items[i]

		// Check if this initializer is next in the pending list.
		pending := k8s.GetNestedArray(uninitialized.UnstructuredContent(), "metadata", "initializers", "pending")
		if len(pending) < 1 {
			continue
		}
		first, ok := pending[0].(map[string]interface{})
		if !ok {
			continue
		}
		if k8s.GetNestedString(first, "name") == c.ic.Spec.InitializerName {
			resp, err := callInitHook(c.ic, &initHookRequest{Object: uninitialized})
			if err != nil {
				// TODO(enisoc): Add this as an event on the uninitialized object?
				errs = append(errs, fmt.Errorf("can't initialize %v %v/%v: %v", uninitialized.GetKind(), uninitialized.GetNamespace(), uninitialized.GetName(), err))
				continue
			}
			initialized := resp.Object

			// Remove this initializer from pending.
			pending = pending[1:]
			if len(pending) == 0 {
				// This is a workaround for a bug in 1.7.x, which does not allow setting 'pending'
				// to an empty list.
				k8s.DeleteNestedField(initialized.UnstructuredContent(), "metadata", "initializers")
			} else {
				k8s.SetNestedField(initialized.UnstructuredContent(), pending, "metadata", "initializers", "pending")
			}

			// Set initializer result if provided.
			if resp.Result != nil {
				k8s.SetNestedField(initialized.UnstructuredContent(), resp.Result, "metadata", "initializers", "result")
			}

			glog.Infof("InitializerController %v: updating %v %v/%v", c.ic.Name, initialized.GetKind(), initialized.GetNamespace(), initialized.GetName())
			if _, err := client.WithNamespace(initialized.GetNamespace()).Update(initialized); err != nil {
				errs = append(errs, fmt.Errorf("can't update %v %v/%v: %v", initialized.GetKind(), initialized.GetNamespace(), initialized.GetName(), err))
				continue
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}
