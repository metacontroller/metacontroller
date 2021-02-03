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
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/metrics/legacyregistry"
	_ "k8s.io/component-base/metrics/prometheus/clientgo"

	"metacontroller.io/server"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	discoveryInterval = flag.Duration("discovery-interval", 30*time.Second, "How often to refresh discovery cache to pick up newly-installed resources")
	informerRelist    = flag.Duration("cache-flush-interval", 30*time.Minute, "How often to flush local caches and relist objects from the API server")
	debugAddr         = flag.String("debug-addr", ":9999", "The address to bind the debug http endpoints")
	clientConfigPath  = flag.String("client-config-path", "", "Path to kubeconfig file (same format as used by kubectl); if not specified, use in-cluster config")
	clientGoQPS       = flag.Float64("client-go-qps", 5, "Number of queries per second client-go is allowed to make (default 5)")
	clientGoBurst     = flag.Int("client-go-burst", 10, "Allowed burst queries for client-go (default 10)")
	workers           = flag.Int("workers", 5, "Number of sync workers to run (default 5)")
	version           = "No version provided"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	klog.InfoS("Discovery cache flush interval", "discovery_interval", *discoveryInterval)
	klog.InfoS("API server object cache flush interval", "cache_flush_interval", *informerRelist)
	klog.InfoS("Http server address", "port", *debugAddr)
	klog.InfoS("Metacontroller build information", "version", version)

	var config *rest.Config
	var err error
	if *clientConfigPath != "" {
		klog.Infof("Using current context from kubeconfig file: %v", *clientConfigPath)
		config, err = clientcmd.BuildConfigFromFlags("", *clientConfigPath)
	} else {
		klog.Info("No kubeconfig file specified; trying in-cluster auto-config...")
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		klog.Fatal(err)
	}

	config.QPS = float32(*clientGoQPS)
	config.Burst = *clientGoBurst

	stopServer, err := server.Start(config, *discoveryInterval, *informerRelist, *workers)
	if err != nil {
		klog.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(legacyregistry.DefaultGatherer, promhttp.HandlerOpts{}))
	srv := &http.Server{
		Addr:    *debugAddr,
		Handler: mux,
	}
	go func() {
		klog.Errorf("Error serving debug endpoint: %v", srv.ListenAndServe())
	}()

	// On SIGTERM, stop all controllers gracefully.
	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigchan
	klog.Infof("Received %q signal. Shutting down...", sig)

	stopServer()
	srv.Shutdown(context.Background())
}
