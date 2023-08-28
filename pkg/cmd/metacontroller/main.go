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
	"metacontroller/pkg/profile"
	"os"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/leaderelection"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"metacontroller/pkg/options"
	"metacontroller/pkg/server"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var (
	discoveryInterval          = flag.Duration("discovery-interval", 30*time.Second, "How often to refresh discovery cache to pick up newly-installed resources")
	informerRelist             = flag.Duration("cache-flush-interval", 30*time.Minute, "How often to flush local caches and relist objects from the API server")
	metricsAddr                = flag.String("metrics-address", ":9999", "The address to bind metrics endpoint - /metrics")
	clientGoQPS                = flag.Float64("client-go-qps", 5, "Number of queries per second client-go is allowed to make (default 5)")
	clientGoBurst              = flag.Int("client-go-burst", 10, "Allowed burst queries for client-go (default 10)")
	workers                    = flag.Int("workers", 5, "Number of sync workers to run (default 5)")
	eventsQPS                  = flag.Float64("events-qps", 1./300., "Rate of events flowing per object (default - 1 event per 5 minutes)")
	eventsBurst                = flag.Int("events-burst", 25, "Number of events allowed to send per object (default 25)")
	pprofAddr                  = flag.String("pprof-address", "0", "Enable pprof and bind to endpoint - /debug/pprof, set to 0 to disable pprof serving")
	leaderElection             = flag.Bool("leader-election", false, "Determines whether or not to use leader election when starting metacontroller")
	leaderElectionResourceLock = flag.String("leader-election-resource-lock", resourcelock.LeasesResourceLock, "Determines which resource lock to use for leader election")
	leaderElectionNamespace    = flag.String("leader-election-namespace", "", "Determines the namespace in which the leader election resource will be created")
	leaderElectionID           = flag.String("leader-election-id", "metacontroller", "Determines the name of the resource that leader election will use for holding the leader lock")
	healthProbeBindAddress     = flag.String("health-probe-bind-address", ":8081", "The address the health probes endpoint binds to")
	targetLabelSelector        = flag.String("target-label-selector", "", "Label selector used to restrict an instance of metacontroller to manage specific Composite and Decorator controllers")
	version                    = "No version provided"
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
		"pprofAddr", *pprofAddr,
		"leader-election", *leaderElection,
		"leader-election-resource-lock", *leaderElectionResourceLock,
		"leader-election-namespace", *leaderElectionNamespace,
		"leader-election-id", *leaderElectionID,
		"health-probe-bind-address", *healthProbeBindAddress,
		"target-label-selector", *targetLabelSelector,
		"version", version)

	pprofStopChan := profile.EnablePprof(*pprofAddr)

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
		MetricsEndpoint:        *metricsAddr,
		HealthProbeBindAddress: *healthProbeBindAddress,
		LeaderElectionOptions: leaderelection.Options{
			LeaderElection:             *leaderElection,
			LeaderElectionResourceLock: *leaderElectionResourceLock,
			LeaderElectionNamespace:    *leaderElectionNamespace,
			LeaderElectionID:           *leaderElectionID,
		},
		TargetLabelSelector: *targetLabelSelector,
	}

	// Create a new manager with a stop function
	// for resource cleanup
	mgr, err := server.New(configuration)
	if err != nil {
		logging.Logger.Error(err, "Cannot create server")
		os.Exit(1)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logging.Logger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logging.Logger.Error(err, "unable to set up ready check")
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

	if pprofStopChan != nil {
		<-pprofStopChan
		logging.Logger.Info("Stopped pprof server")
	}

	<-mgrStopChan.Done()
	logging.Logger.Info("Stopped controller manager")

	wg.Wait()
}
