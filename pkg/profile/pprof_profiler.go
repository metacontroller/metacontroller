package profile

import (
	"context"
	"metacontroller/pkg/logging"
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"os"
	"os/signal"
	"sync"
	"syscall"
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

	pprofMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	logging.Logger.V(4).Info("enabling pprof", "address", address)
	server := &http.Server{
		Addr:              address,
		Handler:           pprofMux,
		ReadHeaderTimeout: 30 * time.Second,
	}
	pprofStopChan := make(chan struct{})
	var closeOnce sync.Once

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop

		// We received a signal, shut down.
		logging.Logger.Info("Shutting down pprof server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			logging.Logger.Error(err, "pprof server shutdown")
		}
		closeOnce.Do(func() {
			close(pprofStopChan)
		})
	}()

	go func() {
		// TODO replace with some server with timeout start method
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logging.Logger.Error(err, "error enabling and serving pprof", "address", address)
			// If it failed to start, we should still close the channel if it hasn't been closed
			closeOnce.Do(func() {
				close(pprofStopChan)
			})
		}
	}()

	return pprofStopChan
}
