# Configuration

This page describes how to configure Metacontroller.

[[_TOC_]]

## Command line flags

The Metacontroller server has a few settings that can be configured
with command-line flags (by editing the Metacontroller StatefulSet
in `manifests/metacontroller.yaml`):

| Flag | Description |
| ---- | ----------- |
| `--zap-log-level` | ZapGws Level to configure the verbosity of logging. Can be one of ‘debug’, ‘info’, ‘error’, or any integer value > 0 which corresponds to custom debug levels of increasing verbosity(e.g. `--zap-log-level=5`). Level 4 logs Metacontroller's interaction with the API server. Levels 5 and up additionally log details of Metacontroller's invocation of lambda hooks. See the [troubleshooting guide](./troubleshooting.md) for more. |
| `--zap-devel` | Development Mode (e.g. `--zap-devel`) defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). |
| `--zap-encoder` | Zap log encoding - `json` or `console` (e.g. `--zap-encoder='json'`) defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). |
| `--zap-stacktrace-level` | Zap Level at and above which stacktraces are captured - one of `info` or `error` (e.g. `--zap-stacktrace-level='info'`). |
| `--discovery-interval` | How often to refresh discovery cache to pick up newly-installed resources (e.g. `--discovery-interval=10s`). |
| `--cache-flush-interval` | How often to flush local caches and relist objects from the API server (e.g. `--cache-flush-interval=30m`). |
| `--metrics-address` | The address to bind metrics endpoint - /metrics (e.g. `--metrics-address=":9999"`). It can be set to "0" to disable the metrics serving. |
| `--kubeconfig` | Path to kubeconfig file (same format as used by kubectl); if not specified, use in-cluster config (e.g. `--kubeconfig=/path/to/kubeconfig`). |
| `--client-go-qps` | Number of queries per second client-go is allowed to make (default 5, e.g. `--client-go-qps=100`) |
| `--client-go-burst` | Allowed burst queries for client-go (default 10, e.g. `--client-go-burst=200`) |
| `--workers` | Number of sync workers to run (default 5, e.g. `--workers=100`) |
| `--events-qps` | Rate of events flowing per object (default - 1 event per 5 minutes, e.g. `--events-qps=0.0033`) |
| `--events-burst` | Number of events allowed to send per object (default 25, e.g. `--events-burst=25`) |

Logging flags are being set by `controller-runtime`, more on the meaning of them can be found [here](https://sdk.operatorframework.io/docs/building-operators/golang/references/logging/#overview)
