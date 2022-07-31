/*
 *
 * Copyright 2022. Metacontroller authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package cache

import (
	"time"

	"zgo.at/zcache/v2"
)

// Cache is an generic cache object. Using it instead of `zcache` to hide implementation details.
type Cache[K comparable, V any] struct {
	cache *zcache.Cache[K, V]
}

// New creates a new cache with a given expiration duration and cleanup
// interval.
//
// If the expiration duration is less than 1 (or NoExpiration) the items in the
// cache never expire (by default) and must be deleted manually.
//
// If the cleanup interval is less than 1 expired items are not deleted from the
// cache before calling c.DeleteExpired().
func New[K comparable, V any](defaultExpiration, cleanupInterval time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		cache: zcache.New[K, V](defaultExpiration, cleanupInterval),
	}
}

// Get an item from the cache.
//
// Returns the item or the zero value and a bool indicating whether the key is
// set.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.cache.Get(key)
}

// Set a cache item, replacing any existing item.
func (c *Cache[K, V]) Set(key K, val V) {
	c.cache.Set(key, val)
}

func (c *Cache[K, V]) SetNoExpiration(key K, val V) {
	c.cache.SetWithExpire(key, val, zcache.NoExpiration)
}
