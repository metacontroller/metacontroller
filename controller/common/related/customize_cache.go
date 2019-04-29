package related

import (
	"metacontroller.app/controller/common"
)

type CustomizeResponseCache map[string]customizeResponseCacheEntry

type customizeResponseCacheEntry struct {
	parentGeneration int64
	cachedResponse   *common.CustomizeHookResponse
}

func (crc CustomizeResponseCache) Add(name string, parentGeneration int64, response *common.CustomizeHookResponse) {
	crc[name] = customizeResponseCacheEntry{
		parentGeneration: parentGeneration,
		cachedResponse:   response,
	}
}

func (crc CustomizeResponseCache) Get(name string, parentGeneration int64) *common.CustomizeHookResponse {
	cacheEntry, ok := crc[name]
	if !ok || cacheEntry.parentGeneration != parentGeneration {
		return nil
	}

	return cacheEntry.cachedResponse
}
