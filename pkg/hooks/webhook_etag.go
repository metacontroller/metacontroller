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

package hooks

import (
	"fmt"
	"metacontroller/pkg/cache"
	"metacontroller/pkg/controller/common/api"
	"metacontroller/pkg/logging"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type webhookExecutorEtag struct {
	etagCache *cache.Cache[eTagKey, *eTagEntry]
}

type eTagEntry struct {
	Etag     string
	Response []byte
}

type eTagKey struct {
	kind, namespace, name string
}

func (w *webhookExecutorEtag) getKeyFromObject(obj *unstructured.Unstructured) eTagKey {
	return eTagKey{
		kind:      obj.GetKind(),
		namespace: obj.GetNamespace(),
		name:      obj.GetName(),
	}
}

func (w *webhookExecutorEtag) enrichHeaders(request *http.Request, webhookRequest api.WebhookRequest) {
	cacheKey := w.getKeyFromObject(webhookRequest.GetParent())
	cacheEntry, cacheEntryExists := w.etagCache.Get(cacheKey)
	if cacheEntryExists {
		request.Header.Set(headerIfNoneMatch, cacheEntry.Etag)
		if logging.Logger.V(6).Enabled() {
			logging.Logger.V(6).Info("enriching headers with 'If-None-Match'", "cacheKey", cacheKey, "eTag", cacheEntry.Etag)
		}
	}
}

func (w *webhookExecutorEtag) adjustResponse(
	request *http.Request,
	webhookRequest api.WebhookRequest,
	responseBody []byte,
	response *http.Response) ([]byte, error) {
	cacheKey := w.getKeyFromObject(webhookRequest.GetParent())
	if request.Header.Get(headerIfNoneMatch) != "" && (response.StatusCode == http.StatusNotModified || response.StatusCode == http.StatusPreconditionFailed) {
		logging.Logger.Info("retrieving body from cache", "cacheKey", cacheKey)
		cacheEntry, cacheEntryExists := w.etagCache.Get(cacheKey)
		if !cacheEntryExists {
			return nil, fmt.Errorf("cannot find cached response for cache key: %s", cacheKey)
		}
		return cacheEntry.Response, nil
	}
	eTag := response.Header.Get(headerETag)
	if eTag != "" {
		w.etagCache.Set(cacheKey, &eTagEntry{Response: responseBody, Etag: eTag})
		logging.Logger.Info("updating cache entry", "cacheKey", cacheKey)
	}
	return responseBody, nil
}

func (w *webhookExecutorEtag) isStatusSupported(request *http.Request, response *http.Response) bool {
	switch response.StatusCode {
	case http.StatusOK:
		return true
	case http.StatusNotModified, http.StatusPreconditionFailed: // we only accept 304 and 412 when "If-None-Match" header were set
		return request.Header.Get(headerIfNoneMatch) != ""
	default:
		return false
	}
}
