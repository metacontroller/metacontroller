# Monitoring via prometheus-operator stack

One of the ways to monitor metacontroller.

This example shows how to use [Prometheus operator](https://prometheus-operator.dev/) to spin up
a [Prometheus](https://prometheus.io/) instance for monitoring metacontroller.

## Prometheus Installation
    kubectl apply -k github.com/prometheus-operator/prometheus-operator?ref=v0.49.0
    kubectl apply -f examples/prometheus-monitoring/manifest/prometheus.yaml
    kubectl rollout status --watch --timeout=180s deployment/prometheus-operator
    until kubectl get statefulset prometheus-prometheus; do sleep 1; done  # prometheus operator creates the statefulset, wait until it is created before monitoring status of rollout
    kubectl rollout status --watch --timeout=180s statefulset/prometheus-prometheus

## Metacontroller Installation
### Helm Installation
    helm install metacontoller deploy/helm/metacontroller -n metacontroller --create-namespace -f deploy/helm/metacontroller/ci/service-values.yaml
    kubectl apply -f examples/prometheus-monitoring/manifest/serviceMonitor.yaml
    kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller

### Kubectl Installation
    kubectl apply -k examples/prometheus-monitoring/manifest
    kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller

## Viewing metrics
- port forward prometheus UI port to localhost 
  - `kubectl port-forward statefulset/prometheus-prometheus 9090`
- run test suite 
  - `cd examples && ./test.sh --ignore=prometheus-monitoring`
- go to `http://localhost:9090/`, you should see prometheus UI
- to see all available metrics, use expression `{namespace="metacontroller"}` in query browser

Currently, exposed metrics are:
- [Controller runtime metrics](https://book-v1.book.kubebuilder.io/beyond_basics/controller_metrics.html)
- client-go metrics (i.e., workqueue or API server connections latency)
- go system metrics (usually with `go_` prefix in name)
- metacontroller own metrics (with `metacontroller_` prefix, i.e. `metacontroller_sync_requests_total`)


New metrics will be added along the way, please let us know what insight information you would like to see.