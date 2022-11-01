#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f "${crd_version}"/my-crd.yaml
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -euo

echo "Install controller..."
kubectl apply -k "${crd_version}"

echo "Create a CRD..."
kubectl apply -f "${crd_version}"/my-crd.yaml

echo "Wait for ClusterRole..."
until [[ "$(kubectl get clusterrole my-tests.ctl.rlg.io-reader -o 'jsonpath={.metadata.name}')" == "my-tests.ctl.rlg.io-reader" ]]; do sleep 1; done
