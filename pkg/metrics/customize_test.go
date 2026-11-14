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

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordRelatedInformerWakeup_whenCalled_addsOneWakeupAndTheParents(t *testing.T) {
	const controllerName = "wakeup-test-controller"
	const wakeups = "metacontroller_customize_related_informer_wakeups_total"
	const parents = "metacontroller_customize_related_informer_enqueued_parents_total"
	wakeupsBefore := counterValue(t, wakeups, controllerName)
	parentsBefore := counterValue(t, parents, controllerName)

	RecordRelatedInformerWakeup(controllerName, 3)

	assert.Equal(t, 1.0, counterValue(t, wakeups, controllerName)-wakeupsBefore, "wake-ups")
	assert.Equal(t, 3.0, counterValue(t, parents, controllerName)-parentsBefore, "enqueued parents")
}

// counterValue reads a counter from the registry. It returns 0 when the
// counter has no value for the controller yet.
func counterValue(t *testing.T, name, controllerName string) float64 {
	t.Helper()

	families, err := registerer.Gather()
	require.NoError(t, err, "can't gather metrics")
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "controller_name" && label.GetValue() == controllerName {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}
