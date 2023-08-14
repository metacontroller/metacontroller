#!/bin/bash

cleanup() {
  exit_code=$?
  set +e
  
  echo "Delete secretpropagation example resources..."
  kubectl delete -f ../secretpropagation/example-secret.yaml
  kubectl delete -k ./manifest

  echo "Rollback metacontroller..."
  kubectl rollout undo statefulset metacontroller -n metacontroller
  kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller
  exit $exit_code
}
trap cleanup EXIT

set -euo

# install metacontroller with the --target-label-selector arg.
kubectl apply -k ./instance
kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller

# wait for metacontroller pod to be ready
kubectl wait --timeout=180s --for=condition=ready pod metacontroller-0 -n metacontroller

# install the the secretpropagation example and applies a patch to add labels to the CompositeController instance.
kubectl apply -k ./manifest

echo "Create a CR..."
kubectl apply -f ../secretpropagation/example-secret.yaml

echo "Wait for Secret propagation..."
until [[ "$(kubectl get secret shareable -n alpha -o 'jsonpath={.metadata.name}')" == "shareable" ]]; do sleep 1; done
until [[ "$(kubectl get secret shareable -n beta -o 'jsonpath={.metadata.name}')" == "shareable" ]]; do sleep 1; done
echo "Check status update on parent..."
until [[ "$(kubectl get SecretPropagation secret-propagation -o 'jsonpath={.status.working}')" == "fine" ]]; do sleep 1; done
