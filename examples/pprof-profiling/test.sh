#!/bin/bash

# Metacontroller runs as a StatefulSet (default) or as a Deployment (see
# manifests/deployment). Pick the resource and overlay matching its kind.
if kubectl get -n metacontroller deployment/metacontroller >/dev/null 2>&1; then
  mc=deployment/metacontroller
  mc_overlay=./manifest-deployment
else
  mc=statefulset/metacontroller
  mc_overlay=./manifest
fi

cleanup() {
  exit_code=$?
  set +e
  echo "Rollback metacontroller..."
  kubectl rollout undo "${mc}" -n metacontroller
  kubectl rollout status --watch --timeout=180s "${mc}" -n metacontroller
  kubectl delete -f manifest/service.yaml
  exit $exit_code
}
trap cleanup EXIT

set -euo

kubectl apply -f manifest/service.yaml
sleep 5
set +e -x # do not fail because we expect an error when pprof is not enabled
kubectl run pprof-should-fail -i --image=golang:alpine --restart=Never --rm -n metacontroller -- go tool pprof -top 20 http://metacontroller:6060/debug/pprof/heap
if ((!$?)); then
  echo "Expected failure sending request to disabled pprof"
  exit 1
fi
set -euo

echo "Enable pprof on metacontroller..."
kubectl apply -k "$mc_overlay"
kubectl rollout status --watch --timeout=180s "${mc}" -n metacontroller

echo "Test profiling metacontroller..."
sleep 5
kubectl run pprof-should-pass -i --image=golang:alpine --restart=Never --rm -n metacontroller -- go tool pprof -top 20 http://metacontroller:6060/debug/pprof/heap
