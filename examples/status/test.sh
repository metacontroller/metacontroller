#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-noop.yaml
  kubectl delete rs,svc -l app=noop-controller 
  kubectl delete -f noop-controller.yaml
  kubectl delete configmap noop-controller -n metacontroller
}
trap cleanup EXIT

set -ex

np="noops.metacontroller.k8s.io"

echo "Install controller..."
kubectl create configmap noop-controller -n metacontroller --from-file=sync.js
kubectl apply -f noop-controller.yaml

echo "Wait until CRD is available..."
until kubectl get $np; do sleep 1; done

echo "Create an object..."
kubectl apply -f my-noop.yaml

echo "Wait for status to be updated..."
until [[ "$(kubectl get $np noop -o 'jsonpath={.status.message}')" == "success" ]]; do sleep 1; done
