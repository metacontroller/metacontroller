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

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/metacontroller/apis/metacontroller/v1alpha1"
)

const (
	hookTimeout = 10 * time.Second
)

type childMap map[string]map[string]*unstructured.Unstructured

type syncHookRequest struct {
	Parent   *unstructured.Unstructured `json:"parent"`
	Children childMap                   `json:"children"`
}

type syncHookResponse struct {
	Status   map[string]interface{}       `json:"status"`
	Children []*unstructured.Unstructured `json:"children"`
}

type initHookRequest struct {
	Object *unstructured.Unstructured `json:"object"`
}

type initHookResponse struct {
	Object *unstructured.Unstructured `json:"object"`
	Result *metav1.Status             `json:"result,omitempty"`
}

func callSyncHook(cc *v1alpha1.CompositeController, request *syncHookRequest) (*syncHookResponse, error) {
	url := fmt.Sprintf("http://%s.%s%s", cc.Spec.ClientConfig.Service.Name, cc.Spec.ClientConfig.Service.Namespace, cc.Spec.Hooks.Sync.Path)
	var response syncHookResponse
	if err := callHook(url, request, &response); err != nil {
		return nil, fmt.Errorf("sync hook failed: %v", err)
	}
	return &response, nil
}

func callInitHook(ic *v1alpha1.InitializerController, request *initHookRequest) (*initHookResponse, error) {
	url := fmt.Sprintf("http://%s.%s%s", ic.Spec.ClientConfig.Service.Name, ic.Spec.ClientConfig.Service.Namespace, ic.Spec.Hooks.Init.Path)
	var response initHookResponse
	if err := callHook(url, request, &response); err != nil {
		return nil, fmt.Errorf("init hook failed: %v", err)
	}
	return &response, nil
}

func callHook(url string, request interface{}, response interface{}) error {
	// Encode request.
	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("can't marshal request: %v", err)
	}
	glog.V(6).Infof("DEBUG: request body: %s", reqBody)

	// Send request.
	client := &http.Client{Timeout: hookTimeout}
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
	glog.V(6).Infof("DEBUG: response body: %s", respBody)

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
