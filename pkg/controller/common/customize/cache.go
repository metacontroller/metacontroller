package customize

import "sync"

// ResponseCache keeps customize hook responses for particular parent's to avoid unnecessary
// calls.
type ResponseCache struct {
	cache map[string]customizeResponseCacheEntry
	lock  sync.RWMutex
}

// NewResponseCache returns new, empty response cache.
func NewResponseCache() *ResponseCache {
	return &ResponseCache{
		cache: make(map[string]customizeResponseCacheEntry),
		lock:  sync.RWMutex{},
	}
}

type customizeResponseCacheEntry struct {
	parentGeneration int64
	cachedResponse   *CustomizeHookResponse
}

// Add adds a given response for given parent and its generation
func (responseCache *ResponseCache) Add(name string, parentGeneration int64, response *CustomizeHookResponse) {
	responseCache.lock.Lock()
	defer responseCache.lock.Unlock()
	responseCache.cache[name] = customizeResponseCacheEntry{
		parentGeneration: parentGeneration,
		cachedResponse:   response,
	}
}

// Get returns response from cache or nil when not found
func (responseCache *ResponseCache) Get(name string, parentGeneration int64) *CustomizeHookResponse {
	responseCache.lock.RLock()
	defer responseCache.lock.RUnlock()
	cacheEntry, ok := responseCache.cache[name]
	if !ok || cacheEntry.parentGeneration != parentGeneration {
		return nil
	}

	return cacheEntry.cachedResponse
}
