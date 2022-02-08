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

package clientset

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"

	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
)

type Clientset struct {
	config    rest.Config
	resources *dynamicdiscovery.ResourceMap
	dc        dynamic.Interface
}

func NewClientset(config *rest.Config, resources *dynamicdiscovery.ResourceMap, dc dynamic.Interface) *Clientset {
	return &Clientset{
		config:    *config,
		resources: resources,
		dc:        dc,
	}
}

func New(config *rest.Config, resources *dynamicdiscovery.ResourceMap) (*Clientset, error) {
	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create dynamic client when creating clientset: %w", err)
	}
	return NewClientset(config, resources, dc), nil
}

func (cs *Clientset) HasSynced() bool {
	return cs.resources.HasSynced()
}

func (cs *Clientset) Resource(apiVersion, resource string) (*ResourceClient, error) {
	// Look up the requested resource in discovery.
	apiResource := cs.resources.Get(apiVersion, resource)
	if apiResource == nil {
		return nil, fmt.Errorf("discovery: can't find resource %s in apiVersion %s", resource, apiVersion)
	}
	return cs.resource(apiResource), nil
}

func (cs *Clientset) Kind(apiVersion, kind string) (*ResourceClient, error) {
	// Look up the requested resource in discovery.
	apiResource := cs.resources.GetKind(apiVersion, kind)
	if apiResource == nil {
		return nil, fmt.Errorf("discovery: can't find kind %s in apiVersion %s", kind, apiVersion)
	}
	return cs.resource(apiResource), nil
}

func (cs *Clientset) resource(apiResource *dynamicdiscovery.APIResource) *ResourceClient {
	client := cs.dc.Resource(apiResource.GroupVersionResource())
	return &ResourceClient{
		ResourceInterface: client,
		APIResource:       apiResource,
		rootClient:        client,
	}
}

// ResourceClient is a combination of APIResource and a dynamic Client.
//
// Passing this around makes it easier to write code that deals with arbitrary
// resource types and often needs to know the API discovery details.
// This wrapper also adds convenience functions that are useful for any client.
//
// It can be used on either namespaced or cluster-scoped resources.
// Call Namespace() to return a client that's scoped down to a given namespace.
type ResourceClient struct {
	dynamic.ResourceInterface
	*dynamicdiscovery.APIResource

	rootClient dynamic.NamespaceableResourceInterface
}

// Namespace returns a copy of the ResourceClient with the client namespace set.
//
// This can be chained to set the namespace to something else.
// Pass "" to return a client with the namespace cleared.
// If the resource is cluster-scoped, this is a no-op.
func (rc *ResourceClient) Namespace(namespace string) *ResourceClient {
	// Ignore the namespace if the resource is cluster-scoped.
	if !rc.Namespaced {
		return rc
	}
	// Reset to cluster-scoped if provided namespace is empty.
	ri := dynamic.ResourceInterface(rc.rootClient)
	if namespace != "" {
		ri = rc.rootClient.Namespace(namespace)
	}
	return &ResourceClient{
		ResourceInterface: ri,
		APIResource:       rc.APIResource,
		rootClient:        rc.rootClient,
	}
}

// AtomicUpdate performs an atomic read-modify-write loop, retrying on
// optimistic concurrency conflicts.
//
// It only uses the identity (name/namespace/uid) of the provided 'orig' object,
// not the contents. The object passed to the update() func will be from a live
// GET against the API server.
//
// This should only be used for unconditional writes, as in, "I want to make
// this change right now regardless of what else may have changed since I last
// read the object."
//
// The update() func should modify the passed object and return true to go ahead
// with the update, or false if no update is required.
func AtomicUpdate(cl client.Client, orig *unstructured.Unstructured, update func(obj *unstructured.Unstructured) bool) (result *unstructured.Unstructured, err error) {
	name := orig.GetName()

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		current := &unstructured.Unstructured{}
		current.SetAPIVersion(orig.GetAPIVersion())
		current.SetKind(orig.GetKind())
		err := cl.Get(context.TODO(), client.ObjectKeyFromObject(orig), current)
		if err != nil {
			return err
		}
		if current.GetUID() != orig.GetUID() {
			// The original object was deleted and replaced with a new one.
			groupVersion, err := schema.ParseGroupVersion(orig.GetAPIVersion())
			if err != nil {
				return err
			}
			return apierrors.NewNotFound(schema.GroupResource{Group: groupVersion.Group, Resource: orig.GetKind()}, name)
		}
		result = current
		if changed := update(current); !changed {
			// There's nothing to do.
			return nil
		}
		return cl.Update(context.TODO(), current)
	})
	return result, err
}

// AtomicStatusUpdate is similar to AtomicUpdate, except that it updates status.
func AtomicStatusUpdate(cl client.Client, orig *unstructured.Unstructured, update func(obj *unstructured.Unstructured) bool) (result *unstructured.Unstructured, err error) {
	name := orig.GetName()

	// We should call GetStatus (if it HasSubresource) to respect subresource
	// RBAC rules, but the dynamic client does not support this yet.
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		current := &unstructured.Unstructured{}
		current.SetAPIVersion(orig.GetAPIVersion())
		current.SetKind(orig.GetKind())
		err := cl.Get(context.TODO(), client.ObjectKeyFromObject(orig), current)
		if err != nil {
			return err
		}
		if current.GetUID() != orig.GetUID() {
			// The original object was deleted and replaced with a new one.
			groupVersion, err := schema.ParseGroupVersion(orig.GetAPIVersion())
			if err != nil {
				return err
			}
			return apierrors.NewNotFound(schema.GroupResource{Group: groupVersion.Group, Resource: orig.GetKind()}, name)
		}
		// we operate on pointer and current gets updated
		result = current
		if changed := update(current); !changed {
			// nothing to do
			return nil
		}
		return cl.Status().Update(context.TODO(), current)
	})
	return result, err
}
