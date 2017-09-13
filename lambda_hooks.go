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

func callSyncHook(lc *v1alpha1.LambdaController, request *syncHookRequest) (*syncHookResponse, error) {
	// Encode request.
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("can't marshal sync hook request: %v", err)
	}
	glog.Infof("DEBUG: request body: %s", reqBody)

	// Send request.
	url := fmt.Sprintf("http://%s.%s%s", lc.Spec.ClientConfig.Service.Name, lc.Spec.ClientConfig.Service.Namespace, lc.Spec.Hooks.Sync.Path)
	client := &http.Client{Timeout: hookTimeout}
	resp, err := client.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("http error: %v", err)
	}
	defer resp.Body.Close()

	// Read response.
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read response body: %v", err)
	}
	glog.Infof("DEBUG: response body: %s", respBody)

	// Check status code.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sync hook remote error: %s", respBody)
	}

	// Decode response.
	response := &syncHookResponse{}
	if err := json.Unmarshal(respBody, response); err != nil {
		return nil, fmt.Errorf("can't unmarshal sync hook response: %v", err)
	}
	return response, nil
}
