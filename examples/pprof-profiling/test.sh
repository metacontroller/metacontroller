#!/bin/bash

cleanup() {
  exit_code=$?
  set +e
  echo "Rollback metacontroller..."
  kubectl rollout undo statefulset metacontroller -n metacontroller
  kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller
  kubectl delete -f manifest/service.yaml
  exit $exit_code
}
trap cleanup EXIT

set -euo

kubectl apply -f manifest/service.yaml
sleep 2
set +e -x # do not fail because we expect an error when pprof is not enabled
kubectl run pprof-should-fail -it --image=golang:alpine --restart=Never --rm -n metacontroller -- go tool pprof -top 20 http://metacontroller.metacontroller:6060/debug/pprof/heap
if ((!$?)); then
  echo "Expected failure sending request to disabled pprof"
  exit 1
fi
set -euo

echo "Enable pprof on metacontroller..."
kubectl apply -k ./manifest
kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller

echo "Test profiling metacontroller..."
sleep 2
kubectl run pprof-should-pass -it --image=golang:alpine --restart=Never --rm -n metacontroller -- go tool pprof -top 20 http://metacontroller.metacontroller:6060/debug/pprof/heap
