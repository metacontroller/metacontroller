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

package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/etag_cache"
	"metacontroller/pkg/logging"
	"metacontroller/pkg/metrics"
	"net/http"
	"time"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"

	k8sjson "k8s.io/apimachinery/pkg/util/json"
)

type WebhookRequest interface {
	GetCacheKey() string
}

type HttpClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

// WebhookExecutor executes a call to a webhook
type WebhookExecutor struct {
	client    HttpClientInterface
	url       string
	hookType  string
	etagCache *etag_cache.Cache
}

// NewWebhookExecutor returns new WebhookExecutor
func NewWebhookExecutor(
	webhook *v1alpha1.Webhook,
	controllerName string,
	controllerType common.ControllerType,
	hookType common.HookType) (*WebhookExecutor, error) {
	if webhook == nil {
		return nil, nil
	}
	url, err := webhookURL(webhook)
	if err != nil {
		return nil, err
	}
	hookTimeout, err := webhookTimeout(webhook)
	if err != nil {
		logging.Logger.Info(err.Error())
	}
	client := &http.Client{Timeout: hookTimeout}
	client, err = metrics.InstrumentClientWithConstLabels(
		controllerName,
		controllerType,
		hookType,
		client,
		url)
	if err != nil {
		return nil, err
	}
	return newWebhookExecutorWithCustomHttpClient(webhook, hookType, client, url)
}

func newWebhookExecutorWithCustomHttpClient(
	webhook *v1alpha1.Webhook,
	hookType common.HookType,
	httpClient HttpClientInterface,
	url string,
) (*WebhookExecutor, error) {
	var eTagCache *etag_cache.Cache = nil
	if webhook.Etag != nil && webhook.Etag.Enabled != nil && *webhook.Etag.Enabled {
		eTagCache = etag_cache.NewCache(webhook.Etag.CacheTimeoutSeconds, webhook.Etag.CacheTimeoutSeconds)
	}

	return &WebhookExecutor{
		client:    httpClient,
		url:       url,
		hookType:  hookType.String(),
		etagCache: eTagCache,
	}, nil
}

func (w *WebhookExecutor) Call(request WebhookRequest, response interface{}) error {
	// Encode request.
	reqBody, err := k8sjson.Marshal(request)
	if err != nil {
		return fmt.Errorf("can't marshal request: %w", err)
	}

	cacheKey := request.GetCacheKey()
	cacheEnabled := w.etagCache != nil
	cacheEntry, cacheEntryExists := w.etagCache.Get(cacheKey)
	eTagValue := ""

	req, err := http.NewRequest("POST", w.url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	if cacheEnabled && cacheEntryExists {
		req.Header.Set("If-None-Match", cacheEntry.Etag)
		eTagValue = cacheEntry.Etag
	}
	if logging.Logger.V(6).Enabled() {
		rawRequest := json.RawMessage(reqBody)
		logging.Logger.V(6).Info("Webhook request", "type", w.hookType, "url", w.url, "etag", eTagValue, "body", rawRequest)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	// Read response.
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can't read response body: %w", err)
	}
	if logging.Logger.V(6).Enabled() {
		rawResponse := json.RawMessage(respBody)
		logging.Logger.V(6).Info("Webhook response", "type", w.hookType, "url", w.url, "etag", eTagValue, "body", rawResponse)
	}

	if cacheEnabled && cacheEntryExists {
		// According to https://datatracker.ietf.org/doc/html/rfc7232
		// When 'If-None-Match' is present and backend responded with 304 it means that object has not changed
		if resp.StatusCode == 304 {
			// TODO: Find a way to deep copy from cacheEntry.Response to response and switch cacheEntry.Response to store decoded response
			if logging.Logger.V(6).Enabled() {
				rawResponse := json.RawMessage(cacheEntry.Response)
				logging.Logger.V(6).Info("Webhook 304 response, reusing cached response", "type", w.hookType, "url", w.url, "etag", eTagValue, "body", rawResponse)
			}
			if err := k8sjson.Unmarshal(cacheEntry.Response, response); err != nil {
				return fmt.Errorf("can't unmarshal response: %w", err)
			}
			return nil
		}
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote error: %s", respBody)
	}

	// Decode response.
	if err := k8sjson.Unmarshal(respBody, response); err != nil {
		return fmt.Errorf("can't unmarshal response: %w", err)
	}

	if cacheEnabled {
		eTag := resp.Header.Get("ETag")
		if eTag != "" {
			w.etagCache.Set(cacheKey, &etag_cache.CacheEntry{Response: respBody, Etag: eTag})
		}
	}
	return nil
}

func webhookURL(webhook *v1alpha1.Webhook) (string, error) {
	if webhook.URL != nil {
		// Full URL overrides everything else.
		return *webhook.URL, nil
	}
	if webhook.Service == nil || webhook.Path == nil {
		return "", fmt.Errorf("invalid webhook config: must specify either full 'url', or both 'service' and 'path'")
	}

	// For now, just use cluster DNS to resolve Services.
	// If necessary, we can use a Lister to get more info about Services.
	if webhook.Service.Name == "" || webhook.Service.Namespace == "" {
		return "", fmt.Errorf("invalid client config: must specify service 'name' and 'namespace'")
	}
	port := int32(80)
	if webhook.Service.Port != nil {
		port = *webhook.Service.Port
	}
	protocol := "http"
	if webhook.Service.Protocol != nil {
		protocol = *webhook.Service.Protocol
	}
	return fmt.Sprintf("%s://%s.%s:%v%s", protocol, webhook.Service.Name, webhook.Service.Namespace, port, *webhook.Path), nil
}

func webhookTimeout(webhook *v1alpha1.Webhook) (time.Duration, error) {
	if webhook.Timeout == nil {
		// Defaults to 10 Seconds to preserve current behavior.
		return 10 * time.Second, nil
	}

	if webhook.Timeout.Duration <= 0 {
		// Defaults to 10 Seconds if invalid.
		return 10 * time.Second, fmt.Errorf("invalid client config: timeout must be a non-zero positive duration. Defaulting to 10 seconds")
	}

	return webhook.Timeout.Duration, nil
}
