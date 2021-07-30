# Monitoring via prometheus-operator stack

One of the ways to monitor metacontroller.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

## Installation
* install prometheus-operator - `kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.49.0/bundle.yaml`
  * wait till `Pod` `prometheus-operator-xxx` is ready (in namespace `default`)
* apply prometheus configuration - `kubectl apply -f examples/prometheus-monitoring/prometheus.yaml`
  * wait till `Pod` `prometheus-prometheus-0` is ready (in namespace `default`)

This example shows how to use [Prometheus operator](https://prometheus-operator.dev/) to spin up
a [Prometheus](https://prometheus.io/) instance for monitoring metacontroller.

## Viewing metrics
* port forward prometheus UI port to localhost - `kubectl -n default port-forward prometheus-prometheus-0 9090:9090`
* run test suite (`cd examples && ./test.sh`) 
* go to `http://localhost:9090/`, you should see prometheus UI
* to see all available metrics, use expression `{namespace="metacontroller"}` in query browser

Currently exposed metrics are:
* [Controller runtime metrics](https://book-v1.book.kubebuilder.io/beyond_basics/controller_metrics.html)
* client-go metrics (i.e., workquene or API server connections latency)
* go system metrics (usually with `go_` prefix in name)
* metacontroller own metrics (with `metacontroller_` prefix, i.e. `metacontroller_sync_requests_total`)


New metrics will be added along the way, please let us know what insight information you would like to see,