package etag_cache

import (
	"strings"
	"time"

	"zgo.at/zcache"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Cache struct {
	cache *zcache.Cache
}

type CacheEntry struct {
	Etag     string
	Response []byte
}

func NewCache(expiration, cleanup *int32) *Cache {
	var exp, cl time.Duration
	if expiration != nil {
		exp = time.Second * time.Duration(*expiration)
	}
	if cleanup != nil {
		cl = time.Second * time.Duration(*cleanup)
	}
	return &Cache{
		cache: zcache.New(exp, cl),
	}
}

func (c *Cache) Get(key string) (*CacheEntry, bool) {
	if c == nil {
		return nil, false
	}
	if val, ok := c.cache.Get(key); ok {
		return val.(*CacheEntry), true
	}
	return nil, false
}

func (c *Cache) Set(key string, val *CacheEntry) {
	c.cache.SetDefault(key, val)
}

func GetKeyFromObject(obj *unstructured.Unstructured) string {
	return strings.Join([]string{
		obj.GetKind(),
		obj.GetName(),
		obj.GetNamespace(),
	}, "/")
}
