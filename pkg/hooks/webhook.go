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
	"metacontroller/pkg/logging"
	"metacontroller/pkg/metrics"
	"net/http"
	"time"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"

	k8sjson "k8s.io/apimachinery/pkg/util/json"
)

// WebhookExecutor executes a call to a webhook
type WebhookExecutor struct {
	client   *http.Client
	url      string
	hookType string
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
	return &WebhookExecutor{
		client:   client,
		url:      url,
		hookType: hookType.String(),
	}, nil
}

func (w *WebhookExecutor) Execute(request interface{}, response interface{}) error {
	// Encode request.
	reqBody, err := k8sjson.Marshal(request)
	if err != nil {
		return fmt.Errorf("can't marshal request: %w", err)
	}
	if logging.Logger.V(6).Enabled() {
		rawRequest := json.RawMessage(reqBody)
		logging.Logger.Info("Webhook request", "type", w.hookType, "url", w.url, "body", rawRequest)
	}
	resp, err := w.client.Post(w.url, "application/json", bytes.NewReader(reqBody))
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
		logging.Logger.V(6).Info("Webhook response", "type", w.hookType, "url", w.url, "body", rawResponse)
	}

	// Check status code.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote error: %s", respBody)
	}

	// Decode response.
	if err := k8sjson.Unmarshal(respBody, response); err != nil {
		return fmt.Errorf("can't unmarshal response: %w", err)
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
