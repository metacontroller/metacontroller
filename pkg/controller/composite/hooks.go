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

package composite

import (
	"fmt"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/api"
	"metacontroller/pkg/controller/composite/api/common"
	v1 "metacontroller/pkg/controller/composite/api/v1"
	v2 "metacontroller/pkg/controller/composite/api/v2"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// hookCallInfo contains information about which hook to call and how
type hookCallInfo struct {
	version      v1alpha1.HookVersion
	isFinalizing bool
	hookType     string
}

func (pc *parentController) callHook(
	parent *unstructured.Unstructured,
	observedChildren, related api.ObjectMap,
) (*v1.CompositeHookResponse, error) {
	// Step 1: Determine which hook to call
	hookInfo := pc.determineHookToCall(parent)
	if hookInfo == nil {
		return nil, nil
	}

	// Step 2: Build and execute the hook request
	request := pc.buildHookRequest(hookInfo, parent, observedChildren, related)
	return pc.executeHook(hookInfo, request, parent)
}

// determineHookToCall decides which hook should be called based on parent state
func (pc *parentController) determineHookToCall(parent *unstructured.Unstructured) *hookCallInfo {
	if pc.shouldCallFinalizeHook(parent) {
		return &hookCallInfo{
			version:      pc.finalizeHook.GetVersion(),
			isFinalizing: true,
			hookType:     "finalize",
		}
	}

	if pc.syncHook.IsEnabled() {
		return &hookCallInfo{
			version:      pc.syncHook.GetVersion(),
			isFinalizing: false,
			hookType:     "sync",
		}
	}

	return nil
}

// shouldCallFinalizeHook determines if finalize hook should be called
func (pc *parentController) shouldCallFinalizeHook(parent *unstructured.Unstructured) bool {
	return pc.finalizeHook.IsEnabled() &&
		(parent.GetDeletionTimestamp() != nil || pc.doNotMatchLabels(parent.GetLabels()))
}

// buildHookRequest creates the appropriate webhook request based on hook version
func (pc *parentController) buildHookRequest(
	hookInfo *hookCallInfo,
	parent *unstructured.Unstructured,
	observedChildren, related api.ObjectMap,
) api.WebhookRequest {
	// Select appropriate builder based on hook version
	var requestBuilder common.WebhookRequestBuilder
	if hookInfo.version == v1alpha1.HookVersionV2 {
		requestBuilder = v2.NewRequestBuilder()
	} else {
		requestBuilder = v1.NewRequestBuilder()
	}

	// Build request with correct format
	requestBuilder = requestBuilder.
		WithController(pc.cc).
		WithParent(parent).
		WithChildren(observedChildren).
		WithRelatedObjects(related)

	if hookInfo.isFinalizing {
		requestBuilder = requestBuilder.IsFinalizing()
	}

	return requestBuilder.Build()
}

// executeHook handles both V1 and V2 hook execution and response conversion
func (pc *parentController) executeHook(
	hookInfo *hookCallInfo,
	request api.WebhookRequest,
	parent *unstructured.Unstructured,
) (*v1.CompositeHookResponse, error) {
	var v1Response *v1.CompositeHookResponse

	if hookInfo.version == v1alpha1.HookVersionV2 {
		var v2Response v2.CompositeHookResponse
		if err := pc.callHookExecutor(hookInfo, request, &v2Response); err != nil {
			return nil, fmt.Errorf("%s hook failed (version=v2): %w", hookInfo.hookType, err)
		}
		v1Response = pc.convertV2ToV1Response(v2Response)
	} else {
		v1Response = &v1.CompositeHookResponse{Children: []*unstructured.Unstructured{}}
		if err := pc.callHookExecutor(hookInfo, request, v1Response); err != nil {
			return nil, fmt.Errorf("%s hook failed (version=v1): %w", hookInfo.hookType, err)
		}
	}

	pc.applyNamespaceDefaults(v1Response.Children, parent)
	return v1Response, nil
}

// callHookExecutor performs the actual hook call (unified logic for both V1 and V2)
func (pc *parentController) callHookExecutor(hookInfo *hookCallInfo, request api.WebhookRequest, response interface{}) error {
	if hookInfo.isFinalizing {
		return pc.finalizeHook.Call(request, response)
	}
	return pc.syncHook.Call(request, response)
}

// convertV2ToV1Response converts V2 response format to V1 for internal consistency
func (pc *parentController) convertV2ToV1Response(v2Response v2.CompositeHookResponse) *v1.CompositeHookResponse {
	return &v1.CompositeHookResponse{
		Status:             v2Response.Status,
		Children:           v2Response.Children,
		ResyncAfterSeconds: v2Response.ResyncAfterSeconds,
		Finalized:          v2Response.Finalized,
	}
}

// applyNamespaceDefaults sets parent namespace on children that don't have one
func (pc *parentController) applyNamespaceDefaults(children []*unstructured.Unstructured, parent *unstructured.Unstructured) {
	for _, child := range children {
		if child != nil && child.GetNamespace() == "" {
			child.SetNamespace(parent.GetNamespace())
		}
	}
}
