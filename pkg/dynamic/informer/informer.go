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
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/tools/cache"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
)

// orphanIndexSentinel is the fixed key under which all controller-ownerless
// objects are stored in the OrphanIndex.
const orphanIndexSentinel = "true"

// OwnerUIDIndex is the name of the informer index that maps controller-owner
// UID → owned objects. It is registered for every child-resource informer so
// that claimChildren can retrieve already-owned objects in O(k) instead of
// scanning the whole cache.
const OwnerUIDIndex = "ownerUID"

// ownerUIDIndexFunc returns the controller-owner UIDs for an object (i.e. the
// UID of every owner reference where Controller==true).
func ownerUIDIndexFunc(obj interface{}) ([]string, error) {
	object, ok := obj.(metav1.Object)
	if !ok {
		return nil, nil
	}
	var uids []string
	for _, ref := range object.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			uids = append(uids, string(ref.UID))
		}
	}
	return uids, nil
}

// OrphanIndex is the name of the informer index that contains all objects that
// have no controller owner reference. All such objects are stored under the
// single key orphanIndexSentinel ("true").
//
// Indexing orphans allows claimChildren to limit its adoption scan to the
// (typically small) orphan set, rather than scanning the full object cache.
const OrphanIndex = "orphan"

// orphanIndexFunc returns [orphanIndexSentinel] if the object has no
// controller owner, so it appears in the OrphanIndex.
func orphanIndexFunc(obj interface{}) ([]string, error) {
	object, ok := obj.(metav1.Object)
	if !ok {
		return nil, nil
	}
	for _, ref := range object.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			// Has a controller owner — not an orphan.
			return nil, nil
		}
	}
	return []string{orphanIndexSentinel}, nil
}

// SharedIndexInformer is an extension of the standard interface of the same
// name, adding the ability to remove event handlers that you added.
type SharedIndexInformer interface {
	cache.SharedIndexInformer

	// RemoveEventHandlers removes all event handlers added through this instance
	// of SharedIndexInformer. It does not affect other handlers on the same
	// underlying shared informer. It waits until each handler stops processing
	// events.
	//
	// This is necessary because the underlying shared informer may continue
	// running if others are using it, so you should remove the handlers you added
	// when you're no longer interested in receiving events.
	RemoveEventHandlers()
}

// ResourceInformer represents a "subscription" to a shared informer and lister.
//
// Users of this package shouldn't create ResourceInformers directly.
// The SharedInformerFactory returns a new ResourceInformer for each request,
// but multiple ResourceInformers may share the same underlying informer if they
// are for the same apiVersion and resource.
//
// When you're done with a ResourceInformer, you should call Close() on it.
// Once all ResourceInformers for a shared informer are closed, the shared
// informer is stopped.
type ResourceInformer struct {
	sharedResourceInformer *sharedResourceInformer
	informerWrapper        *informerWrapper
}

func newResourceInformer(sri *sharedResourceInformer) *ResourceInformer {
	return &ResourceInformer{
		sharedResourceInformer: sri,
		informerWrapper:        newInformerWrapper(sri),
	}
}

// Informer returns an interface to the dynamic, shared informer.
// If you add any event handlers with this interface, you should arrange to call
// RemoveEventHandlers() when you want to stop receiving events.
func (ri *ResourceInformer) Informer() SharedIndexInformer {
	return ri.informerWrapper
}

// Lister returns a shared, dynamic lister that's analogous to the static
// listers generated for static types.
func (ri *ResourceInformer) Lister() dynamiclister.Lister {
	return ri.sharedResourceInformer.lister
}

// ListByOwnerUID returns all cached objects whose controller-owner UID equals
// the given uid. It uses the OwnerUIDIndex for an O(k) lookup instead of a
// full cache scan, where k is the number of objects owned by that controller.
//
// We require filtering by namespace as well, since the same UID could be reused in different
// namespaces and we don't want to return objects from other namespaces, to prevent possible
// security issues.
func (ri *ResourceInformer) ListByOwnerUID(uid types.UID, namespace string) ([]*unstructured.Unstructured, error) {
	return ri.byIndexAndNamespace(OwnerUIDIndex, string(uid), namespace)
}

// ListOrphans returns cached objects that have no controller owner reference
// and that match the given namespace and label selector.
//
// It uses the OrphanIndex for an O(k_orphan) lookup — only orphaned objects
// are examined, not the full cache. namespace may be empty to match all
// namespaces.
func (ri *ResourceInformer) ListOrphans(namespace string) ([]*unstructured.Unstructured, error) {
	return ri.byIndexAndNamespace(OrphanIndex, orphanIndexSentinel, namespace)
}

// byIndex is a shared helper that retrieves unstructured objects from a named
// index by key.
func (ri *ResourceInformer) byIndexAndNamespace(indexName, key string, namespace string) ([]*unstructured.Unstructured, error) {
	items, err := ri.sharedResourceInformer.informer.GetIndexer().ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result := make([]*unstructured.Unstructured, 0, len(items))
	for _, item := range items {
		u, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		if namespace != "" && u.GetNamespace() != namespace {
			continue
		}
		result = append(result, u)
	}
	return result, nil
}

// Close marks this ResourceInformer as unused, allowing the underlying shared
// informer to be stopped when no users are left.
// You should call this when you no longer need the informer, so the watches
// and relists can be stopped.
//
// Close does not remove event handlers and does not stop a custom resync
// timer. Call RemoveEventHandlers on the informer first.
func (ri *ResourceInformer) Close() {
	// Decrement the reference count for the sharedResourceInformer.
	ri.sharedResourceInformer.close()
}

// sharedResourceInformer is the actual, single informer that's shared by
// multiple ResourceInformer instances.
type sharedResourceInformer struct {
	informer            cache.SharedIndexInformer
	lister              dynamiclister.Lister
	defaultResyncPeriod time.Duration

	close func()
}

func newSharedResourceInformer(client *dynamicclientset.ResourceClient, defaultResyncPeriod time.Duration, close func()) *sharedResourceInformer {
	// Use the client-go dynamic informer. It owns the list/watch loop and
	// replays the current store to a handler that is added later.
	//
	// The custom indexes stay local. They are passed to the informer, so
	// claimChildren keeps its O(k) lookups.
	informer := dynamicinformer.NewFilteredDynamicInformer(
		client.DynamicClient(),
		client.GroupVersionResource(),
		metav1.NamespaceAll,
		defaultResyncPeriod,
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
			OwnerUIDIndex:        ownerUIDIndexFunc,
			OrphanIndex:          orphanIndexFunc,
		},
		nil,
	).Informer()

	return &sharedResourceInformer{
		close:               close,
		informer:            informer,
		defaultResyncPeriod: defaultResyncPeriod,
		lister:              dynamiclister.New(informer.GetIndexer(), client.GroupVersionResource()),
	}
}

// SkipInitialList returns an add function for
// cache.ResourceEventHandlerDetailedFuncs that ignores the objects that the
// informer replays to a new handler. These are the objects of the first list,
// or the objects that are in the cache when the handler is added.
//
// Use it only when the handler does not need the replay. For example, a
// controller that adds each parent to its work queue at start reads the
// children and the related objects from the cache during the sync. A replay
// of these objects would only add the same parents again.
func SkipInitialList(add func(obj interface{})) func(obj interface{}, isInInitialList bool) {
	return func(obj interface{}, isInInitialList bool) {
		if isInInitialList {
			return
		}
		add(obj)
	}
}

// informerWrapper wraps a cache.SharedIndexInformer and adds the ability to
// remove handlers, turning it into the custom SharedIndexInformer interface
// defined in this package.
type informerWrapper struct {
	cache.SharedIndexInformer
	sharedResourceInformer *sharedResourceInformer

	mutex    sync.Mutex
	handlers map[cache.ResourceEventHandlerRegistration]*handlerEntry

	// removal serializes the removal calls. A second call then returns only
	// after the handlers of the first call have stopped. It is a separate
	// lock, so a caller can add a handler while a removal waits.
	removal sync.Mutex
}

func newInformerWrapper(sri *sharedResourceInformer) *informerWrapper {
	return &informerWrapper{
		SharedIndexInformer:    sri.informer,
		sharedResourceInformer: sri,
		handlers:               make(map[cache.ResourceEventHandlerRegistration]*handlerEntry),
	}
}

func (iw *informerWrapper) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return iw.addHandler(handler, cache.HandlerOptions{})
}

func (iw *informerWrapper) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return iw.addHandler(handler, cache.HandlerOptions{ResyncPeriod: &resyncPeriod})
}

// AddEventHandlerWithOptions registers the handler and tracks it. It hides the
// method that this type promotes from cache.SharedIndexInformer. Without this
// method, a caller could register an untracked handler, and
// RemoveEventHandlers would not remove it.
func (iw *informerWrapper) AddEventHandlerWithOptions(handler cache.ResourceEventHandler, options cache.HandlerOptions) (cache.ResourceEventHandlerRegistration, error) {
	return iw.addHandler(handler, options)
}

// addHandler registers the handler with the shared informer.
//
// The informer replays the current store to a new handler. It also sends
// resync events at its own period. When the caller asks for a shorter period,
// a timer repeats the resync from the local cache. The timer is necessary
// because the informer can start before or after a handler is added.
// client-go changes the period of the whole shared informer in the first
// case, and it raises a shorter period to the informer period in the second.
//
// The resync period of the handler is not the client-go ResyncPeriod:
//   - A period that is zero or not shorter than the informer period gives
//     the informer period. Zero does not disable the resync.
//   - The first informer resync for a handler that is added late can come
//     after up to two informer periods. After that, the period is constant.
func (iw *informerWrapper) addHandler(handler cache.ResourceEventHandler, options cache.HandlerOptions) (cache.ResourceEventHandlerRegistration, error) {
	iw.mutex.Lock()
	defer iw.mutex.Unlock()

	resyncPeriod := iw.sharedResourceInformer.defaultResyncPeriod
	if options.ResyncPeriod != nil {
		resyncPeriod = *options.ResyncPeriod
	}

	// Do not pass ResyncPeriod to client-go. Before the informer starts,
	// client-go changes the resync check period of the whole shared informer.
	// After the informer starts, client-go raises a short period to the
	// informer period. The local timer gives the short period in both cases.
	registration, err := iw.SharedIndexInformer.AddEventHandlerWithOptions(handler, cache.HandlerOptions{
		Logger: options.Logger,
	})
	if err != nil {
		return nil, err
	}

	entry := &handlerEntry{handler: handler}
	if resyncPeriod > 0 && resyncPeriod < iw.sharedResourceInformer.defaultResyncPeriod {
		entry.start(resyncPeriod, iw.sharedResourceInformer.lister)
	}

	iw.handlers[registration] = entry

	return registration, nil
}

// RemoveEventHandlers removes all handlers that were added through this
// instance of SharedIndexInformer. It waits until each handler stops
// processing events. A caller can release its own resources after it returns.
//
// Do not call it from a handler callback: client-go deadlocks in that case.
//
// A handler that is being added at the same moment can stay registered. Add
// handlers in a start path and remove them in a stop path, so a new handler
// cannot arrive during removal.
func (iw *informerWrapper) RemoveEventHandlers() {
	iw.removal.Lock()
	defer iw.removal.Unlock()

	iw.mutex.Lock()
	entries := iw.handlers
	iw.handlers = make(map[cache.ResourceEventHandlerRegistration]*handlerEntry)
	iw.mutex.Unlock()

	for registration, entry := range entries {
		entry.stop()
		if err := cache.ShutDownEventHandler(iw.SharedIndexInformer, registration); err != nil {
			utilruntime.HandleError(fmt.Errorf("can't shut down event handler: %w", err))
		}
	}
}

// RemoveEventHandler removes one handler added through this instance and waits
// until it stops processing events. It hides the method that this type
// promotes from cache.SharedIndexInformer. Without this method, a direct
// removal would leave the resync timer running.
//
// It does nothing for a registration that this instance does not track. Such
// a registration is already removed, or it belongs to another user of the
// shared informer. That user must remove it, because only that user can stop
// its resync timer.
//
// Do not call it from a handler callback: client-go deadlocks in that case.
func (iw *informerWrapper) RemoveEventHandler(registration cache.ResourceEventHandlerRegistration) error {
	if registration == nil {
		return fmt.Errorf("can't remove event handler: the registration is nil")
	}

	iw.removal.Lock()
	defer iw.removal.Unlock()

	iw.mutex.Lock()
	entry, tracked := iw.handlers[registration]
	delete(iw.handlers, registration)
	iw.mutex.Unlock()

	if !tracked {
		return nil
	}

	entry.stop()
	return cache.ShutDownEventHandler(iw.SharedIndexInformer, registration)
}

// handlerEntry keeps a handler and its optional resync timer. The registration
// of the handler is the key in informerWrapper.handlers.
type handlerEntry struct {
	handler cache.ResourceEventHandler

	stopCh, doneCh chan struct{}
	stopOnce       sync.Once
}

// start gives this handler its own resync period. The period can be shorter
// than the informer's period.
func (he *handlerEntry) start(resyncPeriod time.Duration, lister dynamiclister.Lister) {
	he.stopCh = make(chan struct{})
	he.doneCh = make(chan struct{})

	go func() {
		defer close(he.doneCh)

		ticker := time.NewTicker(resyncPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-he.stopCh:
				return
			case <-ticker.C:
				he.resync(lister)
			}
		}
	}()
}

// resync sends all the latest cached values to the handler.
//
// Unlike the informer's resync, this does not relist from the server. It only
// replays the local cache.
func (he *handlerEntry) resync(lister dynamiclister.Lister) {
	list, err := lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("can't list for resync: %w", err))
		return
	}

	for _, obj := range list {
		// Pass the same object for old/new to indicate a resync.
		he.handler.OnUpdate(obj, obj)
	}
}

func (he *handlerEntry) stop() {
	he.stopOnce.Do(func() {
		if he.stopCh != nil {
			close(he.stopCh)
			<-he.doneCh
		}
	})
}
