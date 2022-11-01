#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-bluegreen.yaml
  kubectl delete rs,svc -l app=nginx,component=frontend
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -eo

bgd="bluegreendeployments"

echo "Install controller..."
kubectl apply -k "${crd_version}"

echo "Wait until CRD is available..."
until kubectl get $bgd; do sleep 1; done

echo "Create an object..."
kubectl apply -f my-bluegreen.yaml

# TODO(juntee): change observedGeneration steps to compare against generation number when k8s 1.10 and earlier retire.

echo "Wait for nginx-blue RS to be active..."
until [[ "$(kubectl get rs nginx-blue -o 'jsonpath={.status.readyReplicas}')" -eq 3 ]]; do sleep 1; done
until [[ "$(kubectl get rs nginx-green -o 'jsonpath={.status.replicas}')" -eq 0 ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.activeColor}')" -eq "blue" ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.active.availableReplicas}')" -eq 3 ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.observedGeneration}')" -eq "$(kubectl get $bgd nginx -o 'jsonpath={.metadata.generation}')" ]]; do sleep 1; done


echo "Trigger a rollout..."
kubectl patch $bgd nginx --type=merge -p '{"spec":{"template":{"metadata":{"labels":{"new":"label"}}}}}'

echo "Wait for nginx-green RS to be active..."
until [[ "$(kubectl get rs nginx-green -o 'jsonpath={.status.readyReplicas}')" -eq 3 ]]; do sleep 1; done
until [[ "$(kubectl get rs nginx-blue -o 'jsonpath={.status.replicas}')" -eq 0 ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.activeColor}')" -eq "green" ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.active.availableReplicas}')" -eq 3 ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.observedGeneration}')" -eq "$(kubectl get $bgd nginx -o 'jsonpath={.metadata.generation}')" ]]; do sleep 1; done

echo "Trigger another rollout..."
kubectl patch $bgd nginx --type=merge -p '{"spec":{"template":{"metadata":{"labels":{"new2":"label2"}}}}}'

echo "Wait for nginx-blue RS to be active..."
until [[ "$(kubectl get rs nginx-blue -o 'jsonpath={.status.readyReplicas}')" -eq 3 ]]; do sleep 1; done
until [[ "$(kubectl get rs nginx-green -o 'jsonpath={.status.replicas}')" -eq 0 ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.activeColor}')" -eq "blue" ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.active.availableReplicas}')" -eq 3 ]]; do sleep 1; done
until [[ "$(kubectl get $bgd nginx -o 'jsonpath={.status.observedGeneration}')" -eq "$(kubectl get $bgd nginx -o 'jsonpath={.metadata.generation}')" ]]; do sleep 1; done