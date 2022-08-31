package profile

import (
	"context"
	"metacontroller/pkg/logging"
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"os"
	"os/signal"
	"time"
)

func EnablePprof(address string) <-chan struct{} {
	// when controller runtime implements serving pprof data, this can be replaced.
	// feature request: https://github.com/kubernetes-sigs/controller-runtime/issues/1779

	// we are intentionally ignoring G108 in gosec linting (https://github.com/securego/gosec)
	// G108: Profiling endpoint automatically exposed on /debug/pprof
	// Importing pprof via _ for its sideeffects enables the API /debug/pprof
	// To avoid exposing this, we do not add handlers to the default mux
	// and pprof is only exposed locally so no external resources can access profiling data
	// See the below pages for additional information:
	// - https://www.farsightsecurity.com/blog/txt-record/go-remote-profiling-20161028/
	// - https://mmcloughlin.com/posts/your-pprof-is-showing
	// In addition, by default pprof is not enabled. This is only intended to be enabled
	// temporarily while gathering profiling information to help troubleshoot

	if address == "0" {
		logging.Logger.V(5).Info("pprof address is set to 0, pprof will not be enabled")
		return nil
	}

	logging.Logger.V(4).Info("enabling pprof", "address", address)
	pprofMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	server := &http.Server{
		Addr:              address,
		Handler:           pprofMux,
		ReadHeaderTimeout: 30 * time.Second,
	}
	pprofStopChan := make(chan struct{})

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := server.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			logging.Logger.Error(err, "pprof server shutdown")
		}
		close(pprofStopChan)
	}()

	go func() {
		// TODO replace with some server with timeout start method
		err := http.ListenAndServe(address, pprofMux) //nolint:gosec
		if err != nil {
			logging.Logger.Error(err, "error enabling and serving pprof", "address", address)
		}
	}()

	return pprofStopChan
}
