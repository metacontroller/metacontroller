/*
Copyright 2026 Metacontroller authors.

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

// Package cleanuprecord has test helpers for the cleanup record of the parent
// finalizer cleanup.
package cleanuprecord

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"metacontroller/pkg/controller/common/finalizer"
	"metacontroller/pkg/internal/testutils/common"
)

// State is the cleanup record.
type State struct {
	Version                   int     `json:"version"`
	Active                    []Entry `json:"active"`
	RemoveParentFinalizerFrom []Entry `json:"removeParentFinalizerFrom"`
}

// Entry is one resource of the cleanup record.
type Entry struct {
	APIVersion string `json:"apiVersion"`
	Resource   string `json:"resource"`
}

// JSON returns the annotation value of the record, with version 1.
func JSON(t *testing.T, state State) string {
	t.Helper()
	state.Version = 1
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	return string(raw)
}

// ForTestResources returns the annotation value of a record with resources of
// common.TestAPIVersion. An empty active gives an empty active list.
func ForTestResources(t *testing.T, active string, retired ...string) string {
	t.Helper()
	state := State{}
	if active != "" {
		state.Active = []Entry{{APIVersion: common.TestAPIVersion, Resource: active}}
	}
	for _, resource := range retired {
		state.RemoveParentFinalizerFrom = append(state.RemoveParentFinalizerFrom, Entry{APIVersion: common.TestAPIVersion, Resource: resource})
	}
	return JSON(t, state)
}

// Read returns the record of the controller object. The test fails if the
// object has no record.
func Read(t *testing.T, controller client.Object) State {
	t.Helper()
	raw := controller.GetAnnotations()[finalizer.CleanupRecordAnnotation]
	require.NotEmpty(t, raw)
	var state State
	require.NoError(t, json.Unmarshal([]byte(raw), &state))
	return state
}

// Resources returns the resource names of the entries, or nil when there are
// no entries.
func Resources(entries []Entry) []string {
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Resource)
	}
	return names
}

// NewParent returns a parent object of common.TestAPIVersion with the given
// finalizers.
func NewParent(kind, name string, finalizers ...string) *unstructured.Unstructured {
	parent := common.NewUnstructured(common.TestAPIVersion, kind, common.TestNamespace, name)
	parent.SetFinalizers(finalizers)
	return parent
}

// CleanCount returns the number of the named objects that have no finalizer.
func CleanCount(t *testing.T, fakeClient *dynamicfake.FakeDynamicClient, gvr schema.GroupVersionResource, names ...string) int {
	t.Helper()
	clean := 0
	for _, name := range names {
		obj, err := fakeClient.Resource(gvr).Namespace(common.TestNamespace).Get(context.TODO(), name, metav1.GetOptions{})
		require.NoError(t, err)
		if len(obj.GetFinalizers()) == 0 {
			clean++
		}
	}
	return clean
}

// RequireEvent fails the test when the recorder has no event that contains
// text. The recorder must be a record.FakeRecorder. It reads the events that
// come before the match.
func RequireEvent(t *testing.T, recorder record.EventRecorder, text string) {
	t.Helper()
	fake, ok := recorder.(*record.FakeRecorder)
	require.True(t, ok, "the recorder is not a FakeRecorder")
	for {
		select {
		case event := <-fake.Events:
			if strings.Contains(event, text) {
				return
			}
		default:
			t.Fatalf("no event contains %q", text)
		}
	}
}
