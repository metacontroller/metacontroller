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

const logErrorsMetric = "metacontroller_log_errors_total"

func TestRecordLogError_whenCalled_incrementsCounter(t *testing.T) {
	before := logErrorsValue(t)

	RecordLogError()
	RecordLogError()

	assert.Equal(t, before+2, logErrorsValue(t))
}

// logErrorsValue reads the log error counter from the metrics registry. The
// counter exists before the first error.
func logErrorsValue(t *testing.T) float64 {
	t.Helper()

	families, err := registerer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() == logErrorsMetric {
			return family.GetMetric()[0].GetCounter().GetValue()
		}
	}
	t.Fatalf("metric %s not found", logErrorsMetric)
	return 0
}
