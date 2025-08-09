#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example.yaml
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -euo

echo "Install controller..."
kubectl apply -k "${crd_version}"

echo "Create example resources..."
kubectl apply -f example.yaml

echo "Wait for ConfigMap list generation..."
# The child configmap should contain exactly the list of source configmaps: "source-ns/cm-1" and "source-ns/cm-2"
# We expect them to be newline-separated and sorted by name (as per sync.py)
expected_content="source-ns/cm-1
source-ns/cm-2"

until [[ "$(kubectl get cm my-config-list-list -n target-ns -o 'jsonpath={.data.configmaps}')" == "$expected_content" ]]; do 
  echo "Waiting for exact content match..."
  kubectl get cm my-config-list-list -n target-ns -o 'jsonpath={.data.configmaps}' || true
  sleep 1
done

echo "Check status update on parent..."
until [[ "$(kubectl get configmaplists.examples.metacontroller.io my-config-list -n target-ns -o 'jsonpath={.status.count}')" == "2" ]]; do sleep 1; done

echo "SUCCESS: ConfigMapList generated correctly from other namespace using v2 API"
