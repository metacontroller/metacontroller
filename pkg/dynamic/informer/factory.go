/*
Copyright 2018 Google Inc.

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

package informer

import (
	"fmt"
	"metacontroller/pkg/logging"
	"sync"
	"time"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
)

// SharedInformerFactory is a factory for requesting dynamic informers from a
// shared pool. It's analogous to the static SharedInformerFactory generated for
// static types.
type SharedInformerFactory struct {
	clientset     *dynamicclientset.Clientset
	defaultResync time.Duration

	mutex           sync.Mutex
	refCount        map[string]int
	sharedInformers map[string]*sharedResourceInformer
}

// NewSharedInformerFactory creates a new factory for shared, dynamic informers.
// Usually there is only one of these for the whole process, created in main().
func NewSharedInformerFactory(clientset *dynamicclientset.Clientset, defaultResync time.Duration) *SharedInformerFactory {
	return &SharedInformerFactory{
		clientset:       clientset,
		defaultResync:   defaultResync,
		refCount:        make(map[string]int),
		sharedInformers: make(map[string]*sharedResourceInformer),
	}
}

// Resource returns a dynamic informer and lister for the given resource.
// These are shared with any other controllers in the same process that request
// the same resource.
//
// If this function returns successfully, the caller should ensure they call
// Close() on the returned ResourceInformer when they no longer need it.
// Shared informers that become unused will be stopped to minimize our load on
// the API server.
func (f *SharedInformerFactory) Resource(apiVersion, resource string) (*ResourceInformer, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Return existing informer if there is one.
	key := resourceKey(apiVersion, resource)
	if sharedInformer, ok := f.sharedInformers[key]; ok {
		count := f.refCount[key] + 1
		f.refCount[key] = count
		logging.Logger.V(4).Info("Subscribed to shared informer", "resource", resource, "api_version", apiVersion, "total_subscribers", count)
		return newResourceInformer(sharedInformer), nil
	}

	// Create one if it doesn't exist.
	client, err := f.clientset.Resource(apiVersion, resource)
	if err != nil {
		return nil, fmt.Errorf("can't create client for %v shared informer: %w", key, err)
	}
	stopCh := make(chan struct{})

	// closeFn is called by users of the shared informer (via Close()) to indicate
	// they no longer need it. We do all incrementing/decrementing of the ref
	// count in the factory while holding the factory mutex, so that removing
	// shared informers is serialized along with creating them.
	closeFn := func() {
		f.mutex.Lock()
		defer f.mutex.Unlock()

		count := f.refCount[key] - 1
		logging.Logger.V(4).Info("Unsubscribed from shared informer", "resource", resource, "api_version", apiVersion, "total_subscribers", count)

		if count > 0 {
			// Others are still using it.
			f.refCount[key] = count
			return
		}

		// We're the last ones using it.
		logging.Logger.V(4).Info("Stopping shared informer (no more subscribers)", "resource", resource, "api_version", apiVersion, "total_subscribers", count)
		close(stopCh)
		delete(f.refCount, key)
		delete(f.sharedInformers, key)
	}

	logging.Logger.V(4).Info("Starting shared informer", "resource", resource, "api_version", apiVersion)
	sharedInformer, err := newSharedResourceInformer(client, f.defaultResync, closeFn)
	if err != nil {
		return nil, fmt.Errorf("can't create client for %v shared informer: %w", key, err)
	}
	f.sharedInformers[key] = sharedInformer
	f.refCount[key] = 1

	// Start the new informer immediately.
	// Users should check HasSynced() before using it.
	go sharedInformer.informer.Run(stopCh)

	return newResourceInformer(sharedInformer), nil
}

func resourceKey(apiVersion, resource string) string {
	return fmt.Sprintf("%s.%s", resource, apiVersion)
}
