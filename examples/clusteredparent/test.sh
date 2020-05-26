#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-clusterrole.yaml
  kubectl delete -f cluster-parent.yaml
  kubectl delete configmap cluster-parent-controller -n metacontroller
}
trap cleanup EXIT

set -ex

echo "Install controller..."
kubectl create configmap cluster-parent-controller -n metacontroller --from-file=sync.py
kubectl apply -f cluster-parent.yaml

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
