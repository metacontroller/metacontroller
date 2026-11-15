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

package finalizer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func annotatedController(annotations map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAnnotations(annotations)
	return u
}

func TestNormalizeCleanupState_sortsAndDeduplicatesByGroupResource(t *testing.T) {
	state := &cleanupState{
		Active: []ParentResource{
			{APIVersion: "z.io/v1", Resource: "widgets"},
			{APIVersion: "apps/v1", Resource: "deployments"},
			{APIVersion: "apps/v2", Resource: "deployments"},
		},
	}

	normalizeCleanupState(state)

	assert.Equal(t, []ParentResource{
		{APIVersion: "apps/v1", Resource: "deployments"},
		{APIVersion: "z.io/v1", Resource: "widgets"},
	}, state.Active)
}

func TestReadCleanupState_whenEntryIsIncomplete_returnsError(t *testing.T) {
	controller := annotatedController(map[string]string{CleanupRecordAnnotation: `{"version":1,"active":[{"apiVersion":"","resource":"x"}]}`})

	_, err := readCleanupState(controller)

	require.ErrorIs(t, err, ErrInvalidCleanupRecord)
}

func TestCheckAnnotationSize_countsOtherKeysButNotTheOldRecord(t *testing.T) {
	annotations := map[string]string{"k": strings.Repeat("v", 100)}
	free := validation.TotalAnnotationSizeLimitB - len("k") - 100 - len(CleanupRecordAnnotation)

	require.NoError(t, checkAnnotationSize(annotatedController(annotations), free))
	err := checkAnnotationSize(annotatedController(annotations), free+1)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInvalidCleanupRecord)

	// The new value replaces the old record, so the old record does not count.
	annotations[CleanupRecordAnnotation] = strings.Repeat("x", validation.TotalAnnotationSizeLimitB)
	require.NoError(t, checkAnnotationSize(annotatedController(annotations), free))
}
