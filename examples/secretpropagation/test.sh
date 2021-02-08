#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example-secret.yaml
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -ex

echo "Install controller..."
kubectl apply -k "${crd_version}"

echo "Create a CRD..."
kubectl apply -f example-secret.yaml

echo "Wait for Secret propagation..."
until [[ "$(kubectl get secret shareable -n alpha -o 'jsonpath={.metadata.name}')" == "shareable" ]]; do sleep 1; done
until [[ "$(kubectl get secret shareable -n beta -o 'jsonpath={.metadata.name}')" == "shareable" ]]; do sleep 1; done