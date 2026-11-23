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

package logging

import (
	"bytes"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var errTest = errors.New("test error")

func TestInitLogging_whenErrorIsLogged_callsObserver(t *testing.T) {
	observed := initObservedLogging(&bytes.Buffer{})

	Logger.Error(errTest, "direct")
	Logger.WithName("child").WithValues("key", "value").Error(errTest, "derived")

	assert.Equal(t, int64(2), observed.Load())
}

func TestInitLogging_whenInfoIsLogged_doesNotCallObserver(t *testing.T) {
	observed := initObservedLogging(&bytes.Buffer{})

	Logger.Info("info")

	assert.Equal(t, int64(0), observed.Load())
}

func TestInitLogging_whenKlogOrHandleErrorReports_callsObserver(t *testing.T) {
	observed := initObservedLogging(&bytes.Buffer{})

	klog.ErrorS(errTest, "klog")
	utilruntime.HandleError(errTest)

	assert.Equal(t, int64(2), observed.Load())
}

func TestInitLogging_whenErrorIsLogged_keepsCallerOfEntry(t *testing.T) {
	var output bytes.Buffer
	InitLogging(
		&controllerruntimezap.Options{Development: true, DestWriter: &output},
		WithErrorObserver(func() {}))

	Logger.Error(errTest, "caller")

	assert.Contains(t, output.String(), "logging/logging_test.go")
}

func TestInitLogging_whenProductionSamplingDropsErrors_observesOnlyWrittenEntries(t *testing.T) {
	const entries = 300
	var output bytes.Buffer
	observed := initObservedLogging(&output)

	for range entries {
		Logger.Error(errTest, "repeated")
	}

	written := int64(strings.Count(output.String(), "\n"))
	assert.Less(t, written, int64(entries))
	assert.Equal(t, written, observed.Load())
}

func TestInitLogging_whenNoObserver_logsError(t *testing.T) {
	var output bytes.Buffer
	InitLogging(&controllerruntimezap.Options{DestWriter: &output})

	Logger.Error(errTest, "no observer")

	assert.Contains(t, output.String(), "no observer")
}

// initObservedLogging starts logging with an observer. It returns the number of
// observed error entries.
func initObservedLogging(output *bytes.Buffer) *atomic.Int64 {
	observed := &atomic.Int64{}
	InitLogging(
		&controllerruntimezap.Options{DestWriter: output},
		WithErrorObserver(func() { observed.Add(1) }))
	return observed
}
