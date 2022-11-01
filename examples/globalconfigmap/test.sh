#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example-globalconfigmap.yaml
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -euo

echo "Install controller..."
kubectl apply -k "${crd_version}"

echo "Create a CRD..."
kubectl apply -f example-globalconfigmap.yaml

echo "Wait for ConfigMap propagation..."
until [[ "$(kubectl get cm globalsettings -n first -o 'jsonpath={.metadata.name}')" == "globalsettings" ]]; do sleep 1; done
until [[ "$(kubectl get cm globalsettings -n second -o 'jsonpath={.metadata.name}')" == "globalsettings" ]]; do sleep 1; done
until [[ "$(kubectl get cm globalsettings -n third -o 'jsonpath={.metadata.name}')" == "globalsettings" ]]; do sleep 1; done
echo "Check status update on parent..."
until [[ "$(kubectl get GlobalConfigMap globalsettings -o 'jsonpath={.status.working}')" == "fine" ]]; do sleep 1; done