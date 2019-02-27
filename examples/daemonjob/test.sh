#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-daemonjob.yaml
  kubectl delete daemonset hello-world-dj
  kubectl delete po -l app=hello-world
  kubectl delete -f daemonjob-controller.yaml
  kubectl delete configmap daemonjob-controller -n metacontroller
}
trap cleanup EXIT

set -ex

dj="daemonjobs"

echo "Install controller..."
kubectl create configmap daemonjob-controller -n metacontroller --from-file=sync.py
kubectl apply -f daemonjob-controller.yaml

echo "Wait until CRD is available..."
until kubectl get $dj; do sleep 1; done

echo "Create an object..."
kubectl apply -f my-daemonjob.yaml

echo "Wait for successful completion..."
until [[ "$(kubectl get $dj hello-world -o 'jsonpath={.status.conditions[0].status}')" == "True" ]]; do sleep 1; done

echo "Check that DaemonSet gets cleaned up after finishing..."
until [[ "$(kubectl get daemonset hello-world-dj 2>&1)" =~ NotFound ]]; do sleep 1; done
