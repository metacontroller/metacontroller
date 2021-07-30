//  MIT License
//
//  © Copyright 2019 travel audience. All rights reserved.
//
//  Permission is hereby granted, free of charge, to any person obtaining a copy
//  of this software and associated documentation files (the "Software"), to deal
//  in the Software without restriction, including without limitation the rights
//  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//  copies of the Software, and to permit persons to whom the Software is
//  furnished to do so, subject to the following conditions:
//
//  The above copyright notice and this permission notice shall be included in all
//  copies or substantial portions of the Software.
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//  SOFTWARE.
//
// code based on https://github.com/travelaudience/go-promhttp/blob/master/client.go

package metrics

import (
	"fmt"
	"metacontroller/pkg/controller/common"
	"net/http"
	"time"

	controllerruntimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"zgo.at/zcache"

	"github.com/prometheus/client_golang/prometheus"
	pph "github.com/prometheus/client_golang/prometheus/promhttp"
)

const metacontrollerPrefix = "metacontroller"

var cache = zcache.New(20*time.Minute, 10*time.Minute)
var registerer = controllerruntimemetrics.Registry

// InstrumentClientWithConstLabels instruments given http.Client with metrics registered in given prometheus.Registerer
func InstrumentClientWithConstLabels(
	controllerName string,
	controllerType common.ControllerType,
	hookType common.HookType,
	c *http.Client,
	url string) (*http.Client, error) {
	constLabels := map[string]string{
		"url":             url,
		"controller_name": controllerName,
		"controller_type": controllerType.String(),
	}
	key := fmt.Sprintf("%s/%s/%s", controllerName, hookType, url)
	instrumentation, err := getOrCreateMetrics(key, hookType, constLabels)
	if err != nil {
		return nil, err
	}

	transport := c.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client := &http.Client{
		CheckRedirect: c.CheckRedirect,
		Jar:           c.Jar,
		Timeout:       c.Timeout,
		Transport: pph.InstrumentRoundTripperInFlight(instrumentation.Collector.inflight,
			pph.InstrumentRoundTripperCounter(instrumentation.Collector.requests,
				pph.InstrumentRoundTripperTrace(instrumentation.Trace,
					pph.InstrumentRoundTripperDuration(instrumentation.Collector.duration, transport),
				),
			),
		),
	}
	return client, nil
}

func getOrCreateMetrics(key string, hookType common.HookType, constLabels map[string]string) (*cachedInstrumentation, error) {
	instrumentationCacheEntry, found := cache.Get(key)
	var instrumentation *cachedInstrumentation
	if !found {
		collector, trace := createNewMetrics(hookType, constLabels)
		newCacheEntry := &cachedInstrumentation{
			Collector: collector,
			Trace:     trace,
		}
		cache.Set(key, newCacheEntry, zcache.NoExpiration)
		instrumentation = newCacheEntry
		err := registerer.Register(instrumentation.Collector)
		if err != nil {
			return nil, err
		}
	} else {
		instrumentation = instrumentationCacheEntry.(*cachedInstrumentation)
	}
	return instrumentation, nil
}

func createNewMetrics(hookType common.HookType, constLabels map[string]string) (*instrumentation, *pph.InstrumentTrace) {
	outgoingInstrumentation := &instrumentation{
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   metacontrollerPrefix,
				Subsystem:   hookType.String(),
				Name:        "requests_total",
				Help:        "A counter for outgoing requests from the wrapped client.",
				ConstLabels: constLabels,
			},
			[]string{"code", "method"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   metacontrollerPrefix,
				Subsystem:   hookType.String(),
				Name:        "request_duration_histogram_seconds",
				Help:        "A histogram of outgoing request latencies.",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: constLabels,
			},
			[]string{"method"},
		),
		dnsDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   metacontrollerPrefix,
				Subsystem:   hookType.String(),
				Name:        "dns_duration_histogram_seconds",
				Help:        "Trace dns latency histogram.",
				Buckets:     []float64{.005, .01, .025, .05},
				ConstLabels: constLabels,
			},
			[]string{"event"},
		),
		tlsDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   metacontrollerPrefix,
				Subsystem:   hookType.String(),
				Name:        "tls_duration_histogram_seconds",
				Help:        "Trace tls latency histogram.",
				Buckets:     []float64{.05, .1, .25, .5},
				ConstLabels: constLabels,
			},
			[]string{"event"},
		),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   metacontrollerPrefix,
			Subsystem:   hookType.String(),
			Name:        "in_flight_requests",
			Help:        "A gauge of in-flight outgoing requests for the wrapped client.",
			ConstLabels: constLabels,
		}),
	}

	trace := &pph.InstrumentTrace{
		DNSStart: func(t float64) {
			outgoingInstrumentation.dnsDuration.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			outgoingInstrumentation.dnsDuration.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			outgoingInstrumentation.tlsDuration.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			outgoingInstrumentation.tlsDuration.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}
	return outgoingInstrumentation, trace
}

type cachedInstrumentation struct {
	Collector *instrumentation
	Trace     *pph.InstrumentTrace
}

type instrumentation struct {
	duration    *prometheus.HistogramVec
	requests    *prometheus.CounterVec
	dnsDuration *prometheus.HistogramVec
	tlsDuration *prometheus.HistogramVec
	inflight    prometheus.Gauge
}

// Describe implements prometheus.Collector interface.
func (i *instrumentation) Describe(in chan<- *prometheus.Desc) {
	i.duration.Describe(in)
	i.requests.Describe(in)
	i.dnsDuration.Describe(in)
	i.tlsDuration.Describe(in)
	i.inflight.Describe(in)
}

// Collect implements prometheus.Collector interface.
func (i *instrumentation) Collect(in chan<- prometheus.Metric) {
	i.duration.Collect(in)
	i.requests.Collect(in)
	i.dnsDuration.Collect(in)
	i.tlsDuration.Collect(in)
	i.inflight.Collect(in)
}
