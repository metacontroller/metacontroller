#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example-configmap.yaml
  kubectl delete -f configmap-propagation.yaml
  kubectl delete configmap configmap-propagation-controller -n metacontroller
}
trap cleanup EXIT

set -ex

echo "Install controller..."
kubectl create configmap configmap-propagation-controller -n metacontroller --from-file=sync.py
kubectl apply -f configmap-propagation.yaml

echo "Create a CRD..."
kubectl apply -f example-configmap.yaml

echo "Wait for ConfigMap propagation..."
until [[ "$(kubectl get cm settings -n one -o 'jsonpath={.metadata.name}')" == "settings" ]]; do sleep 1; done
until [[ "$(kubectl get cm settings -n two -o 'jsonpath={.metadata.name}')" == "settings" ]]; do sleep 1; done
until [[ "$(kubectl get cm settings -n three -o 'jsonpath={.metadata.name}')" == "settings" ]]; do sleep 1; done