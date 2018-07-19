#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-catset.yaml
  kubectl delete po,pvc -l app=nginx,component=backend
  kubectl delete -f catset-controller.yaml
  kubectl delete configmap catset-controller -n metacontroller
}
trap cleanup EXIT

set -ex

cs="catsets"

echo "Install controller..."
kubectl create configmap catset-controller -n metacontroller --from-file=sync.js
kubectl apply -f catset-controller.yaml

echo "Wait until CRD is available..."
until kubectl get $cs; do sleep 1; done

echo "Create an object..."
kubectl apply -f my-catset.yaml

echo "Wait for 3 Pods to be Ready..."
until [[ "$(kubectl get $cs nginx-backend -o 'jsonpath={.status.readyReplicas}')" -eq 3 ]]; do sleep 1; done

echo "Scale up to 4 replicas..."
kubectl patch $cs nginx-backend --type=merge -p '{"spec":{"replicas":4}}'

echo "Wait for 4 Pods to be Ready..."
until [[ "$(kubectl get $cs nginx-backend -o 'jsonpath={.status.readyReplicas}')" -eq 4 ]]; do sleep 1; done

echo "Scale down to 2 replicas..."
kubectl patch $cs nginx-backend --type=merge -p '{"spec":{"replicas":2}}'

echo "Wait for 2 Pods to be Ready..."
until [[ "$(kubectl get $cs nginx-backend -o 'jsonpath={.status.readyReplicas}')" -eq 2 ]]; do sleep 1; done
