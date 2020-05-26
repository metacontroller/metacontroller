#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-indexedjob.yaml
  kubectl delete po -l app=print-index
  kubectl delete -f indexedjob-controller.yaml
  kubectl delete configmap indexedjob-controller -n metacontroller
}
trap cleanup EXIT

set -ex

ij="indexedjobs"

echo "Install controller..."
kubectl create configmap indexedjob-controller -n metacontroller --from-file=sync.py
kubectl apply -f indexedjob-controller.yaml

echo "Wait until CRD is available..."
until kubectl get $ij; do sleep 1; done

echo "Create an object..."
kubectl apply -f my-indexedjob.yaml

echo "Wait for 10 successful completions..."
until [[ "$(kubectl get $ij print-index -o 'jsonpath={.status.succeeded}')" -eq 10 ]]; do sleep 1; done

echo "Check that correct index is printed..."
if [[ "$(kubectl logs print-index-9)" != "9" ]]; then
  exit 1
fi
