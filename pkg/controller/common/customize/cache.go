/*
Copyright 2021 Metacontroller authors.

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

package customize

import (
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"zgo.at/zcache"
)

// ResponseCache keeps customize hook responses for particular parent's to avoid unnecessary
// calls.
type ResponseCache struct {
	cache *zcache.Cache
}

// NewResponseCache returns new, empty response cache.
func NewResponseCache() *ResponseCache {
	cache := zcache.New(20*time.Minute, 10*time.Minute)
	return &ResponseCache{
		cache: cache,
	}
}

type customizeResponseCacheEntry struct {
	parentGeneration int64
	cachedResponse   *v1.CustomizeHookResponse
}

// Add adds a given response for given parent and its generation
func (responseCache *ResponseCache) Add(uid types.UID, parentGeneration int64, response *v1.CustomizeHookResponse) {
	responseCacheEntry := customizeResponseCacheEntry{
		parentGeneration: parentGeneration,
		cachedResponse:   response,
	}
	responseCache.cache.Set(toKey(uid), &responseCacheEntry, zcache.DefaultExpiration)
}

// Get returns response from cache or nil when not found
func (responseCache *ResponseCache) Get(uid types.UID, parentGeneration int64) *v1.CustomizeHookResponse {
	value, found := responseCache.cache.Get(toKey(uid))
	if !found {
		return nil
	}
	responseCacheEntry := value.(*customizeResponseCacheEntry)
	if responseCacheEntry.parentGeneration != parentGeneration {
		return nil
	}
	return responseCacheEntry.cachedResponse
}

func toKey(uid types.UID) string {
	return string(uid)
}
