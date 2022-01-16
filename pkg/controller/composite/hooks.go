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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
)

// SyncHookRequest is the object sent as JSON to the sync and finalize hooks.
type SyncHookRequest struct {
	Controller *v1alpha1.CompositeController `json:"controller"`
	Parent     *unstructured.Unstructured    `json:"parent"`
	Children   common.RelativeObjectMap      `json:"children"`
	Related    common.RelativeObjectMap      `json:"related"`
	Finalizing bool                          `json:"finalizing"`
}

// SyncHookResponse is the expected format of the JSON response from the sync and finalize hooks.
type SyncHookResponse struct {
	Status   map[string]interface{}       `json:"status"`
	Children []*unstructured.Unstructured `json:"children"`

	ResyncAfterSeconds float64 `json:"resyncAfterSeconds"`

	// Finalized is only used by the finalize hook.
	Finalized bool `json:"finalized"`
}

func (pc *parentController) callHook(request *SyncHookRequest) (*SyncHookResponse, error) {
	response := SyncHookResponse{Children: []*unstructured.Unstructured{}}
	// First check if we should instead call the finalize hook,
	// which has the same API as the sync hook except that it's
	// called while the object is pending deletion.
	if request.Parent.GetDeletionTimestamp() != nil && pc.finalizeHook.IsEnabled() {
		// Finalize
		request.Finalizing = true
		if err := pc.finalizeHook.Execute(request, &response); err != nil {
			return nil, fmt.Errorf("finalize hook failed: %w", err)
		}
	} else {
		// Sync
		request.Finalizing = false
		if err := pc.syncHook.Execute(request, &response); err != nil {
			return nil, fmt.Errorf("sync hook failed: %w", err)
		}
	}

	return &response, nil
}
