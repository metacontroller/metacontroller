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
	commonv1 "metacontroller/pkg/controller/common/api/v1"
	v1 "metacontroller/pkg/controller/decorator/api/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (c *decoratorController) callHook(
	parent *unstructured.Unstructured,
	observedChildren, related commonv1.RelativeObjectMap,
) (*v1.DecoratorHookResponse, error) {
	if c.dc.Spec.Hooks == nil {
		return nil, fmt.Errorf("no hooks defined")
	}

	requestBuilder := v1.NewRequestBuilder().
		WithController(c.dc).
		WithParet(parent).
		WithAttachments(observedChildren).
		WithRelatedObjects(related)

	response := v1.DecoratorHookResponse{Attachments: []*unstructured.Unstructured{}}

	// First check if we should instead call the finalize hook,
	// which has the same API as the sync hook except that it's
	// called while the object is pending deletion.
	//
	// In addition to finalizing when the object is deleted, we also finalize
	// when the object no longer matches our decorator selector.
	// This allows the decorator to clean up after itself if the object has been
	// updated to disable the functionality added by the decorator.
	if c.finalizeHook.IsEnabled() &&
		(parent.GetDeletionTimestamp() != nil || !c.parentSelector.Matches(parent)) {
		// Finalize
		if err := c.finalizeHook.Call(requestBuilder.IsFinalizing().Build(), &response); err != nil {
			return nil, fmt.Errorf("finalize hook failed: %w", err)
		}
	} else {
		// Sync
		if err := c.syncHook.Call(requestBuilder.Build(), &response); err != nil {
			return nil, fmt.Errorf("sync hook failed: %w", err)
		}
	}

	return &response, nil
}
