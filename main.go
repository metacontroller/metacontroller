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

	"github.com/golang/glog"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/stats/view"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"metacontroller.app/server"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	discoveryInterval = flag.Duration("discovery-interval", 30*time.Second, "How often to refresh discovery cache to pick up newly-installed resources")
	informerRelist    = flag.Duration("cache-flush-interval", 30*time.Minute, "How often to flush local caches and relist objects from the API server")
	debugAddr         = flag.String("debug-addr", ":9999", "The address to bind the debug http endpoints")
	clientConfigPath  = flag.String("client-config-path", "", "Path to kubeconfig file (same format as used by kubectl); if not specified, use in-cluster config")
)

func main() {
	flag.Parse()

	glog.Infof("Discovery cache flush interval: %v", *discoveryInterval)
	glog.Infof("API server object cache flush interval: %v", *informerRelist)
	glog.Infof("Debug http server address: %v", *debugAddr)

	var config *rest.Config
	var err error
	if *clientConfigPath != "" {
		glog.Infof("Using current context from kubeconfig file: %v", *clientConfigPath)
		config, err = clientcmd.BuildConfigFromFlags("", *clientConfigPath)
	} else {
		glog.Info("No kubeconfig file specified; trying in-cluster auto-config...")
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatal(err)
	}

	stopServer, err := server.Start(config, *discoveryInterval, *informerRelist)
	if err != nil {
		glog.Fatal(err)
	}

	exporter, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		glog.Fatalf("can't create prometheus exporter: %v", err)
	}
	view.RegisterExporter(exporter)

	mux := http.NewServeMux()
	mux.Handle("/metrics", exporter)
	srv := &http.Server{
		Addr:    *debugAddr,
		Handler: mux,
	}
	go func() {
		glog.Errorf("Error serving debug endpoint: %v", srv.ListenAndServe())
	}()

	// On SIGTERM, stop all controllers gracefully.
	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigchan
	glog.Infof("Received %q signal. Shutting down...", sig)

	stopServer()
	srv.Shutdown(context.Background())
}
