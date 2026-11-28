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
	"github.com/prometheus/client_golang/prometheus"
)

var logErrors = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: metacontrollerPrefix,
	Subsystem: "log",
	Name:      "errors_total",
	Help:      "A counter for all error log entries of Metacontroller.",
})

func init() {
	registerer.MustRegister(logErrors)
}

// RecordLogError counts one error log entry. Give it to
// logging.WithErrorObserver.
func RecordLogError() {
	logErrors.Inc()
}
