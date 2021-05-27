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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"metacontroller.io/pkg/apis/metacontroller.io/v1alpha1"
	"metacontroller.io/pkg/controller/common"
	"metacontroller.io/pkg/hooks"
)

// SyncHookRequest is the object sent as JSON to the sync hook.
type SyncHookRequest struct {
	Controller  *v1alpha1.DecoratorController `json:"controller"`
	Object      *unstructured.Unstructured    `json:"object"`
	Attachments common.ChildMap               `json:"attachments"`
	Related     common.ChildMap               `json:"related"`
	Finalizing  bool                          `json:"finalizing"`
}

// SyncHookResponse is the expected format of the JSON response from the sync hook.
type SyncHookResponse struct {
	Labels      map[string]*string           `json:"labels"`
	Annotations map[string]*string           `json:"annotations"`
	Status      map[string]interface{}       `json:"status"`
	Attachments []*unstructured.Unstructured `json:"attachments"`

	ResyncAfterSeconds float64 `json:"resyncAfterSeconds"`

	// Finalized is only used by the finalize hook.
	Finalized bool `json:"finalized"`
}

func (c *decoratorController) callSyncHook(request *SyncHookRequest) (*SyncHookResponse, error) {
	if c.dc.Spec.Hooks == nil {
		return nil, fmt.Errorf("no hooks defined")
	}

	var response SyncHookResponse

	// First check if we should instead call the finalize hook,
	// which has the same API as the sync hook except that it's
	// called while the object is pending deletion.
	//
	// In addition to finalizing when the object is deleted, we also finalize
	// when the object no longer matches our decorator selector.
	// This allows the decorator to clean up after itself if the object has been
	// updated to disable the functionality added by the decorator.
	if c.dc.Spec.Hooks.Finalize != nil &&
		(request.Object.GetDeletionTimestamp() != nil || !c.parentSelector.Matches(request.Object)) {
		// Finalize
		request.Finalizing = true
		if err := hooks.Call(c.dc.Spec.Hooks.Finalize, request, &response); err != nil {
			return nil, fmt.Errorf("finalize hook failed: %v", err)
		}
	} else {
		// Sync
		request.Finalizing = false
		if c.dc.Spec.Hooks.Sync == nil {
			return nil, fmt.Errorf("sync hook not defined")
		}

		if err := hooks.Call(c.dc.Spec.Hooks.Sync, request, &response); err != nil {
			return nil, fmt.Errorf("sync hook failed: %v", err)
		}
	}

	return &response, nil
}
