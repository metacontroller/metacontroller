package main

import (
	"fmt"

	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type APIResource struct {
	metav1.APIResource
	APIVersion string
}

func (r *APIResource) GroupVersion() schema.GroupVersion {
	gv, err := schema.ParseGroupVersion(r.APIVersion)
	if err != nil {
		// This shouldn't happen because we get this value from discovery.
		panic(fmt.Sprintf("API discovery returned invalid group/version %q: %v", r.APIVersion, err))
	}
	return gv
}

func (r *APIResource) GroupVersionKind() schema.GroupVersionKind {
	return r.GroupVersion().WithKind(r.Kind)
}

type resourceDiscovery interface {
	Get(groupVersion, resource string) *APIResource
	GetKind(groupVersion, kind string) *APIResource
}

type groupVersionEntry struct {
	resources, kinds map[string]*APIResource
}

type resourceMap map[string]groupVersionEntry

func (r resourceMap) Get(apiVersion, resource string) *APIResource {
	if gv, ok := r[apiVersion]; ok {
		return gv.resources[resource]
	}
	return nil
}

func (r resourceMap) GetKind(apiVersion, kind string) *APIResource {
	if gv, ok := r[apiVersion]; ok {
		return gv.kinds[kind]
	}
	return nil
}

func newResourceMap(groups []*metav1.APIResourceList) resourceMap {
	r := make(resourceMap, len(groups))
	for _, group := range groups {
		gv := groupVersionEntry{
			resources: make(map[string]*APIResource, len(group.APIResources)),
			kinds:     make(map[string]*APIResource, len(group.APIResources)),
		}
		for i := range group.APIResources {
			apiResource := &APIResource{
				APIResource: group.APIResources[i],
				APIVersion:  group.GroupVersion,
			}
			gv.resources[apiResource.Name] = apiResource
			// Remember how to map back from Kind to resource.
			// This is different from what RESTMapper provides because we already know
			// the full GroupVersionKind and just need the resource name.
			// Make sure we don't choose a subresource like "pods/status".
			if !strings.ContainsRune(apiResource.Name, '/') {
				gv.kinds[apiResource.Kind] = apiResource
			}
		}
		r[group.GroupVersion] = gv
	}
	return r
}
