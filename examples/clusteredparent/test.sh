#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-clusterrole.yaml
  kubectl delete -k manifest
}
trap cleanup EXIT

set -euo

echo "Install controller..."
kubectl apply -k manifest

echo "Create a ClusterRole..."
kubectl apply -f my-clusterrole.yaml

echo "Wait for Namespaced child..."
until [[ "$(kubectl get rolebinding -n default my-clusterrole -o 'jsonpath={.metadata.name}')" == "my-clusterrole" ]]; do sleep 1; done

echo "Delete Namespaced child..."
kubectl delete rolebinding -n default my-clusterrole --wait=true

# Test that the controller with cluster-scoped parent notices the namespaced child got deleted.
echo "Wait for Namespaced child to be recreated..."
until [[ "$(kubectl get rolebinding -n default my-clusterrole -o 'jsonpath={.metadata.name}')" == "my-clusterrole" ]]; do sleep 1; done

# Test to make sure cascading deletion of cross namespaces resources works.
echo "Deleting ClusterRole..."
kubectl delete -f my-clusterrole.yaml

echo "Wait for Namespaced child cleanup..."
until [[ "$(kubectl get clusterrole.rbac.authorization.k8s.io -n default  my-clusterrole 2>&1 )" == *NotFound* ]]; do sleep 1; done
