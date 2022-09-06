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
	"io"
	"metacontroller/pkg/cache"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/logging"
	"metacontroller/pkg/metrics"
	"net/http"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"

	k8sjson "k8s.io/apimachinery/pkg/util/json"
	kjson "sigs.k8s.io/json"
)

const (
	headerIfNoneMatch = "If-None-Match"
	headerETag        = "ETag"
)

type WebhookRequest interface {
	GetRootObject() *unstructured.Unstructured
}

type HttpClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

// WebhookExecutor executes a call to a webhook
type WebhookExecutor interface {
	Call(request WebhookRequest, response interface{}) error
}

// NewWebhookExecutor returns new WebhookExecutor
func NewWebhookExecutor(
	webhook *v1alpha1.Webhook,
	controllerName string,
	controllerType common.ControllerType,
	hookType common.HookType) (WebhookExecutor, error) {
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
	var abstract webhookAbstract
	if isEtagEnabled(webhook) {
		var defaultExpiration, cleanupInterval time.Duration
		if webhook.Etag.CacheTimeoutSeconds != nil {
			defaultExpiration = time.Second * time.Duration(*webhook.Etag.CacheTimeoutSeconds)
		}
		if webhook.Etag.CacheTimeoutSeconds != nil {
			cleanupInterval = time.Second * time.Duration(*webhook.Etag.CacheCleanupSeconds)
		}
		abstract = &webhookExecutorEtag{
			etagCache: cache.New[eTagKey, *eTagEntry](defaultExpiration, cleanupInterval)}
	} else {
		abstract = &webhookExecutorPlain{}
	}
	return newWebhookExecutor(
		client,
		url,
		hookType,
		webhook.ResponseUnmarshallMode,
		abstract,
	), nil
}

func newWebhookExecutor(client HttpClientInterface,
	url string,
	hookType common.HookType,
	unmarshallMode *v1alpha1.ResponseUnmarshallMode,
	abstract webhookAbstract) *webhookExecutor {
	return &webhookExecutor{
		client:                 client,
		url:                    url,
		hookType:               hookType.String(),
		webhookAbstract:        abstract,
		responseUnmarshallMode: responseUnmarshallMode(unmarshallMode),
	}
}

func responseUnmarshallMode(mode *v1alpha1.ResponseUnmarshallMode) v1alpha1.ResponseUnmarshallMode {
	if mode == nil {
		return v1alpha1.ResponseUnmarshallModeLoose
	}
	return *mode
}

type webhookAbstract interface {
	enrichHeaders(request *http.Request, webhookRequest WebhookRequest)
	isStatusSupported(request *http.Request, response *http.Response) bool
	adjustResponse(request *http.Request, webhookRequest WebhookRequest, responseBody []byte, response *http.Response) ([]byte, error)
}

type webhookExecutor struct {
	client                 HttpClientInterface
	url                    string
	hookType               string
	webhookAbstract        webhookAbstract
	responseUnmarshallMode v1alpha1.ResponseUnmarshallMode
}

func (w *webhookExecutor) Call(webhookRequest WebhookRequest, webhookResponse interface{}) error {
	// Encode webhookRequest.
	requestBody, err := k8sjson.Marshal(webhookRequest)
	if err != nil {
		return fmt.Errorf("can't marshal request: %w", err)
	}
	if logging.Logger.V(6).Enabled() {
		rawRequest := json.RawMessage(requestBody)
		logging.Logger.V(6).Info("Webhook request", "type", w.hookType, "url", w.url, "body", rawRequest)
	}
	request, err := http.NewRequest("POST", w.url, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	w.webhookAbstract.enrichHeaders(request, webhookRequest)

	response, err := w.client.Do(request)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer response.Body.Close()

	// Read webhookResponse.
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("can't read response body: %w", err)
	}
	if logging.Logger.V(6).Enabled() {
		rawResponse := json.RawMessage(responseBody)
		logging.Logger.V(6).Info("Webhook response", "type", w.hookType, "url", w.url, "body", rawResponse)
	}

	if !w.webhookAbstract.isStatusSupported(request, response) {
		return fmt.Errorf("unsupported status code: %d body: %s", response.StatusCode, responseBody)
	}

	responseBody, err = w.webhookAbstract.adjustResponse(request, webhookRequest, responseBody, response)
	if err != nil {
		return err
	}
	// Decode webhookResponse.
	if strictErrors, err := kjson.UnmarshalStrict(responseBody, webhookResponse); err != nil {
		return fmt.Errorf("can't unmarshal webhookResponse: %w", err)
	} else if w.shouldReportStrictErrors() {
		return fmt.Errorf("strict validation failed for webkookResponse: %w", utilerrors.NewAggregate(strictErrors))
	}
	return nil
}

func (w *webhookExecutor) shouldReportStrictErrors() bool {
	return w.responseUnmarshallMode == v1alpha1.ResponseUnmarshallModeStrict
}

func isEtagEnabled(webhook *v1alpha1.Webhook) bool {
	return webhook.Etag != nil && webhook.Etag.Enabled != nil && *webhook.Etag.Enabled
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
