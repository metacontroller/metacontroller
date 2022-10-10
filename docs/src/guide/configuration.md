# Configuration

This page describes how to configure Metacontroller.

[[_TOC_]]

## Command line flags

The Metacontroller server has a few settings that can be configured
with command-line flags (by editing the Metacontroller StatefulSet
in `manifests/metacontroller.yaml`):

| Flag | Description |
| ---- | ----------- |
| `--zap-log-level` | Zap log level to configure the verbosity of logging. Can be one of ‘debug’, ‘info’, ‘error’, or any integer value > 0 which corresponds to custom debug levels of increasing verbosity(e.g. `--zap-log-level=5`). Level 4 logs Metacontroller's interaction with the API server. Levels 5 and up additionally log details of Metacontroller's invocation of lambda hooks. See the [troubleshooting guide](./troubleshooting.md) for more. |
| `--zap-devel` | Development Mode (e.g. `--zap-devel`) defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). |
| `--zap-encoder` | Zap log encoding - `json` or `console` (e.g. `--zap-encoder='json'`) defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). |
| `--zap-stacktrace-level` | Zap Level at and above which stacktraces are captured - one of `info` or `error` (e.g. `--zap-stacktrace-level='info'`). |
| `--discovery-interval` | How often to refresh discovery cache to pick up newly-installed resources (e.g. `--discovery-interval=10s`). |
| `--cache-flush-interval` | How often to flush local caches and relist objects from the API server (e.g. `--cache-flush-interval=30m`). |
| `--metrics-address` | The address to bind metrics endpoint - /metrics (e.g. `--metrics-address=:9999`). It can be set to "0" to disable the metrics serving. |
| `--kubeconfig` | Path to kubeconfig file (same format as used by kubectl); if not specified, use in-cluster config (e.g. `--kubeconfig=/path/to/kubeconfig`). |
| `--client-go-qps` | Number of queries per second client-go is allowed to make (default 5, e.g. `--client-go-qps=100`) |
| `--client-go-burst` | Allowed burst queries for client-go (default 10, e.g. `--client-go-burst=200`) |
| `--workers` | Number of sync workers to run (default 5, e.g. `--workers=100`) |
| `--events-qps` | Rate of events flowing per object (default - 1 event per 5 minutes, e.g. `--events-qps=0.0033`) |
| `--events-burst` | Number of events allowed to send per object (default 25, e.g. `--events-burst=25`) |
| `--pprof-address` | Enable pprof and bind to endpoint /debug/pprof, set to 0 to disable pprof serving (default 0, e.g. `--pprof-address=:6060`) |
| `--leader-election` | Determines whether or not to use leader election when starting metacontroller (default `false`, e.g., `--leader-election`) |
| `--leader-election-resource-lock` | Determines which resource lock to use for leader election (default `leases`, e.g., `--leader-election-resource-lock=leases`). Valid resource locks are `endpoints`, `configmaps`, `leases`, `endpointsleases`, or `configmapsleases`. See the client-go documentation [leaderelection/resourcelock](https://pkg.go.dev/k8s.io/client-go/tools/leaderelection/resourcelock#pkg-constants) for additional information. |
| `--leader-election-namespace` | Determines the namespace in which the leader election resource will be created. If metacontroller is running in-cluster, the default leader election namespace is the same namespace as metacontroller. If metacontroller is running out-of-cluster, the default leader election namespace is undefined. If you are running metacontroller out-of-cluster with leader election enabled, you must specify the leader election namespace. (e.g., `--leader-election-namespace=metacontroller`) |
| `--leader-election-id` | Determines the name of the resource that leader election will use for holding the leader lock. For example, if the leader election id is `metacontroller` and the leader election resource lock is `leases`, then a resource of kind `leases` with metadata.name `metacontroller` will hold the leader lock. (default metacontroller, e.g., `--leader-election-id=metacontroller`) |
| `--api` | Enable or disable http api server, e.g. `--api=true` |
| `--api-port` | The address to bind api server endpoint, e.g. `--api-port=8090` |
| `--api-trigger-sync` | Enable or disable sync api trigger (GET localhost:8090/trigger_sync), e.g. `--api-trigger-sync=true` |


Logging flags are being set by `controller-runtime`, more on the meaning of them can be found [here](https://sdk.operatorframework.io/docs/building-operators/golang/references/logging/#overview)
