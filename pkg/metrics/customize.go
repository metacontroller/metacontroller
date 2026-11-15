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

const customizeSubsystem = "customize"

var (
	relatedInformerWakeups = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metacontrollerPrefix,
			Subsystem: customizeSubsystem,
			Name:      "related_informer_wakeups_total",
			Help:      "How many times a controller added parents to its work queue after the cache of a new related informer was filled.",
		},
		[]string{controllerNameLabel},
	)
	relatedInformerEnqueuedParents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metacontrollerPrefix,
			Subsystem: customizeSubsystem,
			Name:      "related_informer_enqueued_parents_total",
			Help:      "How many parents a controller added to its work queue after the cache of a new related informer was filled.",
		},
		[]string{controllerNameLabel},
	)
)

func init() {
	registerer.MustRegister(relatedInformerWakeups, relatedInformerEnqueuedParents)
}

// RecordRelatedInformerWakeup records that the controller added the given
// number of parents to its work queue after the cache of a new related
// informer was filled.
func RecordRelatedInformerWakeup(controllerName string, parents int) {
	relatedInformerWakeups.WithLabelValues(controllerName).Inc()
	relatedInformerEnqueuedParents.WithLabelValues(controllerName).Add(float64(parents))
}
