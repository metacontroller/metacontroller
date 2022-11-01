#!/bin/bash

cleanup() {
  exit_code=$?
  set +e
  echo "Killing port-forward for prometheus..."
  pkill kubectl
  echo "Rollback metacontroller..."
  kubectl rollout undo statefulset metacontroller -n metacontroller
  kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller
  echo "Uninstall prometheus..."
  kubectl delete -f ./manifest/prometheus.yaml
  kubectl delete -k github.com/prometheus-operator/prometheus-operator?ref=v0.49.0
  exit $exit_code
}
trap cleanup EXIT

set -euo

echo "Install prometheus..."
kubectl apply -k github.com/prometheus-operator/prometheus-operator?ref=v0.49.0
kubectl rollout status --watch --timeout=180s deployment/prometheus-operator

echo "Wait until prometheus CRD is available..."
until kubectl get prometheus; do sleep 1; done

echo "Set up prometheus..."
kubectl apply -f ./manifest/prometheus.yaml
until kubectl get statefulset prometheus-prometheus; do sleep 1; done  # prometheus operator creates the statefulset, wait until it is created before monitoring status of rollout
kubectl rollout status --watch --timeout=180s statefulset/prometheus-prometheus

echo "Enable prometheus monitoring on metacontroller..."
kubectl apply -k ./manifest
kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller

echo "Starting port-forward for prometheus..."
kubectl port-forward statefulset/prometheus-prometheus 9090 &

echo "Verifying successfully configured prometheus..."
end=$((SECONDS+180))  # timeout after 3 minutes
status=''
while [ $SECONDS -lt $end ]; do
  if [ "$status" != 'up' ] ; then
    status=$(curl --silent http://localhost:9090/api/v1/targets | jq --raw-output '.data.activeTargets[] | select(.labels.service | startswith("metacontroller")) | .health // empty')
  fi

  if [ "$status" == 'up' ] ; then break ; fi

  echo "status is ${status}"
  echo "waiting for successful prometheus configuration with metacontroller..."
  sleep 10
done

if [ "$status" != 'up' ] ; then exit 1; fi
