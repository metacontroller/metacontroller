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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"

	dynamicdiscovery "k8s.io/metacontroller/dynamic/discovery"
)

type Clientset struct {
	config    rest.Config
	resources *dynamicdiscovery.ResourceMap
	dc        dynamic.Interface
}

func New(config *rest.Config, resources *dynamicdiscovery.ResourceMap) (*Clientset, error) {
	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create dynamic client when creating clientset: %v", err)
	}
	return &Clientset{
		config:    *config,
		resources: resources,
		dc:        dc,
	}, nil
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
	return &ResourceClient{
		NamespaceableResourceInterface: cs.dc.Resource(apiResource.GroupVersionResource()),
		resource:                       apiResource,
	}
}

type ResourceClient struct {
	dynamic.NamespaceableResourceInterface
	resource *dynamicdiscovery.APIResource
}

type NamespacedResourceClient struct {
	dynamic.ResourceInterface
	resource *dynamicdiscovery.APIResource
}

func (rc *ResourceClient) Namespace(namespace string) *NamespacedResourceClient {
	return &NamespacedResourceClient{
		ResourceInterface: rc.NamespaceableResourceInterface.Namespace(namespace),
		resource:          rc.resource,
	}
}

func (rc *ResourceClient) Kind() string {
	return rc.resource.Kind
}

func (rc *ResourceClient) GroupVersion() schema.GroupVersion {
	return rc.resource.GroupVersion()
}

func (rc *ResourceClient) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    rc.resource.GroupVersion().Group,
		Resource: rc.resource.Name,
	}
}

func (rc *ResourceClient) GroupVersionKind() schema.GroupVersionKind {
	return rc.resource.GroupVersion().WithKind(rc.resource.Kind)
}

func (rc *ResourceClient) GroupVersionResource() schema.GroupVersionResource {
	return rc.resource.GroupVersion().WithResource(rc.resource.Name)
}

func (rc *ResourceClient) APIResource() *dynamicdiscovery.APIResource {
	return rc.resource
}

func (rc *ResourceClient) UpdateWithRetries(orig *unstructured.Unstructured, update func(obj *unstructured.Unstructured) bool) (result *unstructured.Unstructured, err error) {
	return updateWithRetries(rc, rc.resource, orig, update)
}

func (nrc *NamespacedResourceClient) UpdateWithRetries(orig *unstructured.Unstructured, update func(obj *unstructured.Unstructured) bool) (result *unstructured.Unstructured, err error) {
	return updateWithRetries(nrc, nrc.resource, orig, update)
}

func updateWithRetries(ri dynamic.ResourceInterface, resource *dynamicdiscovery.APIResource, orig *unstructured.Unstructured, update func(obj *unstructured.Unstructured) bool) (result *unstructured.Unstructured, err error) {
	name := orig.GetName()
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		current, err := ri.Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.GetUID() != orig.GetUID() {
			// The original object was deleted and replaced with a new one.
			groupResource := schema.GroupResource{
				Group:    resource.GroupVersion().Group,
				Resource: resource.Name,
			}
			return apierrors.NewNotFound(groupResource, name)
		}
		if changed := update(current); !changed {
			// There's nothing to do.
			result = current
			return nil
		}
		result, err = ri.Update(current)
		return err
	})
	return result, err
}
