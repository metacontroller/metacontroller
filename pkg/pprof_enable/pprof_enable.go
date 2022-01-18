package pprof_enable

import (
	"metacontroller/pkg/logging"
	"net/http"
	_ "net/http/pprof"
)

func EnablePprof(address string) {
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
	pprofMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	if address == "0" {
		logging.Logger.Info("pprof address is set to 0, pprof will not be enabled")
	} else {
		logging.Logger.Info("enabling pprof", "address", address)
		go func() {
			err := http.ListenAndServe(address, pprofMux)
			if err != nil {
				logging.Logger.Error(err, "error enabling and serving pprof", "address", address)
			}
		}()
	}
}
