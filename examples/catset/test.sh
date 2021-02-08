#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl patch $cs nginx-backend --type=merge -p '{"metadata":{"finalizers":[]}}'
  kubectl delete -f my-catset.yaml
  kubectl delete po,pvc -l app=nginx,component=backend
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -ex

cs="catsets"
finalizer="metacontroller.io/catset-test"

echo "Install controller..."
kubectl apply -k "${crd_version}"

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

echo "Append our own finalizer so we can read the final state..."
kubectl patch $cs nginx-backend --type=json -p '[{"op":"add","path":"/metadata/finalizers/-","value":"'${finalizer}'"}]'

echo "Delete CatSet..."
kubectl delete $cs nginx-backend --wait=false

echo "Expect CatSet's finalizer to scale the CatSet to 0 replicas..."
until [[ "$(kubectl get $cs nginx-backend -o 'jsonpath={.status.replicas}')" -eq 0 ]]; do sleep 1; done

echo "Wait for our finalizer to be the only one left, then remove it..."
until [[ "$(kubectl get $cs nginx-backend -o 'jsonpath={.metadata.finalizers}')" == "[\"${finalizer}\"]" ]]; do sleep 1; done
kubectl patch $cs nginx-backend --type=merge -p '{"metadata":{"finalizers":[]}}'
