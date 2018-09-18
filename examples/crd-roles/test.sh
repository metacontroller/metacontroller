#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-crd.yaml
  kubectl delete -f crd-role-controller.yaml
  kubectl delete configmap crd-role-controller -n metacontroller
}
trap cleanup EXIT

set -ex

echo "Install controller..."
kubectl create configmap crd-role-controller -n metacontroller --from-file=sync.py
kubectl apply -f crd-role-controller.yaml

echo "Create a CRD..."
kubectl apply -f my-crd.yaml

echo "Wait for ClusterRole..."
until [[ "$(kubectl get clusterrole my-tests.ctl.rlg.io-reader -o 'jsonpath={.metadata.name}')" == "my-tests.ctl.rlg.io-reader" ]]; do sleep 1; done
