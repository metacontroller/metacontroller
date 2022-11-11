#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example-configmap.yaml
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -euo

echo "Install controller..."
kubectl apply -k "${crd_version}"

echo "Create a CRD..."
kubectl apply -f example-configmap.yaml

echo "Wait for ConfigMap propagation..."
until [[ "$(kubectl get cm settings -n one -o 'jsonpath={.metadata.name}')" == "settings" ]]; do sleep 1; done
until [[ "$(kubectl get cm settings -n two -o 'jsonpath={.metadata.name}')" == "settings" ]]; do sleep 1; done
until [[ "$(kubectl get cm settings -n three -o 'jsonpath={.metadata.name}')" == "settings" ]]; do sleep 1; done
echo "Check status update on parent..."
until [[ "$(kubectl get ConfigMapPropagation settings-propagation -o 'jsonpath={.status.actual_copies}')" == "3" ]]; do sleep 1; done
sleep 5
if [[ "$(kubectl get cm settings -n four -o 'jsonpath={.metadata.name}')" == "settings" ]];
then
  echo "ERROR: Found configmap in unmanaged namespace"
  exit 1
fi