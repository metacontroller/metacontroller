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
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"

	"metacontroller.io/pkg/apis/metacontroller.io/v1alpha1"
)

func callWebhook(webhook *v1alpha1.Webhook, request interface{}, response interface{}) error {
	url, err := webhookURL(webhook)
	if err != nil {
		return err
	}
	hookTimeout, err := webhookTimeout(webhook)
	if err != nil {
		klog.InfoS(err.Error())
	}
	// Encode request.
	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("can't marshal request: %v", err)
	}
	if klog.V(6).Enabled() {
		klog.InfoS("Webhook request", "url", url, "body", string(reqBody))
	}

	// Send request.
	client := &http.Client{Timeout: hookTimeout}
	klog.V(6).InfoS("Webhook timeout", "timeout", hookTimeout)
	resp, err := client.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("http error: %v", err)
	}
	defer resp.Body.Close()

	// Read response.
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can't read response body: %v", err)
	}
	klog.V(6).InfoS("Webhook response", "url", url, "body", string(respBody))

	// Check status code.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote error: %s", respBody)
	}

	// Decode response.
	if err := json.Unmarshal(respBody, response); err != nil {
		return fmt.Errorf("can't unmarshal response: %v", err)
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
