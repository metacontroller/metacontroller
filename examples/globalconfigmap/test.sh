#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example-globalconfigmap.yaml
  kubectl delete -f globalconfigmap.yaml
  kubectl delete configmap globalconfigmap-controller -n metacontroller
}
trap cleanup EXIT

set -ex

echo "Install controller..."
kubectl create configmap globalconfigmap-controller -n metacontroller --from-file=sync.py
kubectl apply -f globalconfigmap.yaml

echo "Create a CRD..."
kubectl apply -f example-globalconfigmap.yaml

echo "Wait for ConfigMap propagation..."
until [[ "$(kubectl get cm globalsettings -n first -o 'jsonpath={.metadata.name}')" == "globalsettings" ]]; do sleep 1; done
until [[ "$(kubectl get cm globalsettings -n second -o 'jsonpath={.metadata.name}')" == "globalsettings" ]]; do sleep 1; done
until [[ "$(kubectl get cm globalsettings -n third -o 'jsonpath={.metadata.name}')" == "globalsettings" ]]; do sleep 1; done