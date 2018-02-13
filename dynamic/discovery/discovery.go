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

package discovery

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
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

type groupVersionEntry struct {
	resources, kinds map[string]*APIResource
}

type ResourceMap struct {
	mutex         sync.RWMutex
	groupVersions map[string]groupVersionEntry

	discoveryClient discovery.DiscoveryInterface
}

func (rm *ResourceMap) Get(apiVersion, resource string) (result *APIResource) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	gv, ok := rm.groupVersions[apiVersion]
	if !ok {
		return nil
	}
	return gv.resources[resource]
}

func (rm *ResourceMap) GetKind(apiVersion, kind string) (result *APIResource) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	gv, ok := rm.groupVersions[apiVersion]
	if !ok {
		return nil
	}
	return gv.kinds[kind]
}

func (rm *ResourceMap) refresh() {
	// Fetch all API Group-Versions and their resources from the server.
	// We do this before acquiring the lock so we don't block readers.
	glog.V(7).Info("Refreshing API discovery info")
	groups, err := rm.discoveryClient.ServerResources()
	if err != nil {
		glog.Errorf("Failed to fetch discovery info: %v", err)
		return
	}

	// Denormalize resource lists into maps for convenient lookup
	// by either Group-Version-Kind or Group-Version-Resource.
	groupVersions := make(map[string]groupVersionEntry, len(groups))
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
		groupVersions[group.GroupVersion] = gv
	}

	// Replace the local cache.
	rm.mutex.Lock()
	rm.groupVersions = groupVersions
	rm.mutex.Unlock()
}

func (rm *ResourceMap) Start(refreshInterval time.Duration) (cancel func()) {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			rm.refresh()

			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

func (rm *ResourceMap) HasSynced() bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.groupVersions != nil
}

func NewResourceMap(discoveryClient discovery.DiscoveryInterface) *ResourceMap {
	return &ResourceMap{
		discoveryClient: discoveryClient,
	}
}
