/*
Copyright 2018 Google Inc.

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

package decorator

import (
	"fmt"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/api"
	"metacontroller/pkg/controller/decorator/api/common"
	v1 "metacontroller/pkg/controller/decorator/api/v1"
	v2 "metacontroller/pkg/controller/decorator/api/v2"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// decoratorHookCallInfo contains information about which decorator hook to call and how
type decoratorHookCallInfo struct {
	version      v1alpha1.HookVersion
	isFinalizing bool
	hookType     string
}

func (c *decoratorController) callHook(
	parent *unstructured.Unstructured,
	observedChildren,
	related api.ObjectMap,
) (*v1.DecoratorHookResponse, error) {
	if c.dc.Spec.Hooks == nil {
		return nil, fmt.Errorf("no hooks defined")
	}

	// Step 1: Determine which hook to call
	hookInfo := c.determineHookToCall(parent)
	if hookInfo == nil {
		return nil, fmt.Errorf("no enabled hooks found")
	}

	// Step 2: Build and execute the hook request
	request := c.buildHookRequest(hookInfo, parent, observedChildren, related)
	return c.executeHook(hookInfo, request, parent)
}

// determineHookToCall decides which hook should be called based on parent state
func (c *decoratorController) determineHookToCall(parent *unstructured.Unstructured) *decoratorHookCallInfo {
	if c.shouldCallFinalizeHook(parent) {
		return &decoratorHookCallInfo{
			version:      c.finalizeHook.GetVersion(),
			isFinalizing: true,
			hookType:     "finalize",
		}
	}

	if c.syncHook.IsEnabled() {
		return &decoratorHookCallInfo{
			version:      c.syncHook.GetVersion(),
			isFinalizing: false,
			hookType:     "sync",
		}
	}

	return nil
}

// shouldCallFinalizeHook determines if finalize hook should be called
func (c *decoratorController) shouldCallFinalizeHook(parent *unstructured.Unstructured) bool {
	return c.finalizeHook.IsEnabled() &&
		(parent.GetDeletionTimestamp() != nil || !c.parentSelector.Matches(parent))
}

// buildHookRequest creates the appropriate webhook request based on hook version
func (c *decoratorController) buildHookRequest(
	hookInfo *decoratorHookCallInfo,
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
		WithController(c.dc).
		WithParent(parent).
		WithChildren(observedChildren).
		WithRelatedObjects(related)

	if hookInfo.isFinalizing {
		requestBuilder = requestBuilder.IsFinalizing()
	}

	return requestBuilder.Build()
}

// executeHook handles both V1 and V2 hook execution and response conversion
func (c *decoratorController) executeHook(
	hookInfo *decoratorHookCallInfo,
	request api.WebhookRequest,
	parent *unstructured.Unstructured,
) (*v1.DecoratorHookResponse, error) {
	var v1Response *v1.DecoratorHookResponse

	if hookInfo.version == v1alpha1.HookVersionV2 {
		var v2Response v2.DecoratorHookResponse
		if err := c.callHookExecutor(hookInfo, request, &v2Response); err != nil {
			return nil, fmt.Errorf("%s hook failed (version=v2): %w", hookInfo.hookType, err)
		}
		v1Response = c.convertV2ToV1Response(v2Response)
	} else {
		v1Response = &v1.DecoratorHookResponse{Attachments: []*unstructured.Unstructured{}}
		if err := c.callHookExecutor(hookInfo, request, v1Response); err != nil {
			return nil, fmt.Errorf("%s hook failed (version=v1): %w", hookInfo.hookType, err)
		}
	}

	c.applyNamespaceDefaults(v1Response.Attachments, parent)
	return v1Response, nil
}

// callHookExecutor performs the actual hook call (unified logic for both V1 and V2)
func (c *decoratorController) callHookExecutor(hookInfo *decoratorHookCallInfo, request api.WebhookRequest, response interface{}) error {
	if hookInfo.isFinalizing {
		return c.finalizeHook.Call(request, response)
	}
	return c.syncHook.Call(request, response)
}

// convertV2ToV1Response converts V2 response format to V1 for internal consistency
func (c *decoratorController) convertV2ToV1Response(v2Response v2.DecoratorHookResponse) *v1.DecoratorHookResponse {
	return &v1.DecoratorHookResponse{
		Labels:             v2Response.Labels,
		Annotations:        v2Response.Annotations,
		Status:             v2Response.Status,
		Attachments:        v2Response.Attachments,
		ResyncAfterSeconds: v2Response.ResyncAfterSeconds,
		Finalized:          v2Response.Finalized,
	}
}

// applyNamespaceDefaults sets parent namespace on attachments that don't have one
func (c *decoratorController) applyNamespaceDefaults(attachments []*unstructured.Unstructured, parent *unstructured.Unstructured) {
	for _, attachment := range attachments {
		if attachment != nil && attachment.GetNamespace() == "" {
			attachment.SetNamespace(parent.GetNamespace())
		}
	}
}
