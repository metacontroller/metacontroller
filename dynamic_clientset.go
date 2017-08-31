package main

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/metacontroller/util/retry"
)

type dynamicClientset struct {
	config    rest.Config
	resources resourceDiscovery
}

func newDynamicClientset(config *rest.Config, resources resourceDiscovery) *dynamicClientset {
	return &dynamicClientset{
		config:    *config,
		resources: resources,
	}
}

func (cs *dynamicClientset) Resource(apiVersion, resource, namespace string) (*dynamicResourceClient, error) {
	// Look up the requested resource in discovery.
	apiResource := cs.resources.Get(apiVersion, resource)
	if apiResource == nil {
		return nil, fmt.Errorf("discovery: can't find resource %s in apiVersion %s", resource, apiVersion)
	}
	return cs.resource(apiResource, namespace)
}

func (cs *dynamicClientset) Kind(apiVersion, kind, namespace string) (*dynamicResourceClient, error) {
	// Look up the requested resource in discovery.
	apiResource := cs.resources.GetKind(apiVersion, kind)
	if apiResource == nil {
		return nil, fmt.Errorf("discovery: can't find kind %s in apiVersion %s", kind, apiVersion)
	}
	return cs.resource(apiResource, namespace)
}

func (cs *dynamicClientset) resource(apiResource *APIResource, namespace string) (*dynamicResourceClient, error) {
	// Create dynamic client for this apiVersion/resource.
	gv := apiResource.GroupVersion()
	config := cs.config
	config.GroupVersion = &gv
	if gv.Group != "" {
		config.APIPath = "/apis"
	}
	dc, err := dynamic.NewClient(&config)
	if err != nil {
		return nil, fmt.Errorf("can't create dynamic client for resource %v in apiVersion %v: %v", apiResource.Name, apiResource.APIVersion, err)
	}
	return &dynamicResourceClient{ResourceInterface: dc.Resource(&apiResource.APIResource, namespace), gv: gv, resource: apiResource}, nil
}

type dynamicResourceClient struct {
	dynamic.ResourceInterface

	gv       schema.GroupVersion
	resource *APIResource
}

func (rc *dynamicResourceClient) Kind() string {
	return rc.resource.Kind
}

func (rc *dynamicResourceClient) GroupVersion() schema.GroupVersion {
	return rc.gv
}

func (rc *dynamicResourceClient) GroupVersionKind() schema.GroupVersionKind {
	return rc.gv.WithKind(rc.resource.Kind)
}

func (rc *dynamicResourceClient) APIResource() *APIResource {
	return rc.resource
}

func (rc *dynamicResourceClient) UpdateWithRetries(orig *unstructured.Unstructured, update func(obj *unstructured.Unstructured) bool) error {
	name := orig.GetName()
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		current, err := rc.Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.GetUID() != orig.GetUID() {
			return newUIDError("can't update %v %v/%v: original object is gone: got uid %v, want %v", rc.resource.Kind, orig.GetNamespace(), orig.GetName(), current.GetUID(), orig.GetUID())
		}
		if changed := update(current); !changed {
			// There's nothing to do.
			return nil
		}
		_, err = rc.Update(current)
		return err
	})
}
