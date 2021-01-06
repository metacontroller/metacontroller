#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example-secret.yaml
  kubectl delete -f secret-propagation.yaml
  kubectl delete configmap secret-propagation-controller -n metacontroller
}
trap cleanup EXIT

set -ex

echo "Install controller..."
kubectl create configmap secret-propagation-controller -n metacontroller --from-file=sync.py
kubectl apply -f secret-propagation.yaml

echo "Create a CRD..."
kubectl apply -f example-secret.yaml

echo "Wait for Secret propagation..."
until [[ "$(kubectl get secret shareable -n alpha -o 'jsonpath={.metadata.name}')" == "shareable" ]]; do sleep 1; done
until [[ "$(kubectl get secret shareable -n beta -o 'jsonpath={.metadata.name}')" == "shareable" ]]; do sleep 1; done