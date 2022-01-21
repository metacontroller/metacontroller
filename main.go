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

package main

import (
	"flag"
	"metacontroller/pkg/logging"
	"os"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"metacontroller/pkg/options"
	"metacontroller/pkg/server"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var (
	discoveryInterval = flag.Duration("discovery-interval", 30*time.Second, "How often to refresh discovery cache to pick up newly-installed resources")
	informerRelist    = flag.Duration("cache-flush-interval", 30*time.Minute, "How often to flush local caches and relist objects from the API server")
	metricsAddr       = flag.String("metrics-address", ":9999", "The address to bind metrics endpoint - /metrics")
	clientGoQPS       = flag.Float64("client-go-qps", 5, "Number of queries per second client-go is allowed to make (default 5)")
	clientGoBurst     = flag.Int("client-go-burst", 10, "Allowed burst queries for client-go (default 10)")
	workers           = flag.Int("workers", 5, "Number of sync workers to run (default 5)")
	eventsQPS         = flag.Float64("events-qps", 1./300., "Rate of events flowing per object (default - 1 event per 5 minutes)")
	eventsBurst       = flag.Int("events-burst", 25, "Number of events allowed to send per object (default 25)")
	version           = "No version provided"
)

func main() {
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	logging.InitLogging(&opts)

	logging.Logger.Info("Configuration information",
		"discovery-interval", *discoveryInterval,
		"cache-flush-interval", *informerRelist,
		"metrics-address", *metricsAddr,
		"client-go-qps", *clientGoQPS,
		"client-go-burst", *clientGoBurst,
		"workers", *workers,
		"events-qps", *eventsQPS,
		"events-burst", *eventsBurst,
		"version", version)

	config, err := controllerruntime.GetConfig()
	if err != nil {
		logging.Logger.Error(err, "Terminating")
		os.Exit(1)
	}
	config.QPS = float32(*clientGoQPS)
	config.Burst = *clientGoBurst

	configuration := options.Configuration{
		RestConfig:        config,
		DiscoveryInterval: *discoveryInterval,
		InformerRelist:    *informerRelist,
		Workers:           *workers,
		CorrelatorOptions: record.CorrelatorOptions{
			BurstSize: *eventsBurst,
			QPS:       float32(*eventsQPS),
		},
		MetricsEndpoint: *metricsAddr,
	}

	// Create a new manager with a stop function
	// for resource cleanup
	mgr, err := server.New(configuration)
	if err != nil {
		logging.Logger.Error(err, "Terminating")
		os.Exit(1)
	}

	// Use a WaitGroup to make sure the metrics server
	// and controller manager stop gracefully
	// before we exit
	var wg sync.WaitGroup
	wg.Add(1)
	mgrStopChan := signals.SetupSignalHandler()
	go func() {
		defer wg.Done()
		if err := mgr.Start(mgrStopChan); err != nil {
			logging.Logger.Error(err, "Terminating")
			os.Exit(1)
		}
		logging.Logger.Info("Stopped metacontroller")
	}()

	<-mgrStopChan.Done()
	logging.Logger.Info("Stopped controller manager")
	wg.Wait()
}
