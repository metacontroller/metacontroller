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
	"k8s.io/apimachinery/pkg/runtime"

	"metacontroller.app/apis/metacontroller/v1alpha1"
	"metacontroller.app/controller/common"
	"metacontroller.app/hooks"
)

type syncHookRequest struct {
	Controller runtime.Object             `json:"controller"`
	Parent     *unstructured.Unstructured `json:"parent"`
	Children   common.ChildMap            `json:"children"`
	Finalizing bool                       `json:"finalizing"`
}

type syncHookResponse struct {
	Status   map[string]interface{}       `json:"status"`
	Children []*unstructured.Unstructured `json:"children"`

	// Finalized is only used by the finalize hook.
	Finalized bool `json:"finalized"`
}

func callSyncHook(cc *v1alpha1.CompositeController, request *syncHookRequest) (*syncHookResponse, error) {
	if cc.Spec.Hooks == nil {
		return nil, fmt.Errorf("no hooks defined")
	}

	var response syncHookResponse

	// First check if we should instead call the finalize hook,
	// which has the same API as the sync hook except that it's
	// called while the object is pending deletion.
	if request.Parent.GetDeletionTimestamp() != nil && cc.Spec.Hooks.Finalize != nil {
		// Finalize
		request.Finalizing = true
		if err := hooks.Call(cc.Spec.Hooks.Finalize, request, &response); err != nil {
			return nil, fmt.Errorf("finalize hook failed: %v", err)
		}
	} else {
		// Sync
		request.Finalizing = false
		if cc.Spec.Hooks.Sync == nil {
			return nil, fmt.Errorf("sync hook not defined")
		}

		if err := hooks.Call(cc.Spec.Hooks.Sync, request, &response); err != nil {
			return nil, fmt.Errorf("sync hook failed: %v", err)
		}
	}

	return &response, nil
}
