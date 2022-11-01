#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-indexedjob.yaml
  kubectl delete po -l app=print-index
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -euo

ij="indexedjobs"

echo "Install controller..."
kubectl apply -k "${crd_version}"

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
