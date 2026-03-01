#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f example.yaml
  kubectl delete -k "${crd_version}"
}
trap cleanup EXIT

set -euo

echo "Install controller..."
kubectl apply -k "${crd_version}"

echo "Create example resources..."
kubectl apply -f example.yaml

echo "Wait for Namespace list generation..."
# The child configmap should contain exactly the list of matching namespaces (at least 'test-ns')
# In this example, only 'test-ns' is labeled with example-controller=namespace-list.
expected_content="test-ns"

until [[ "$(kubectl get cm filtered-namespaces-list -n test-ns -o 'jsonpath={.data.namespaces}')" == "$expected_content" ]]; do
  echo "Waiting for exact content match..."
  kubectl get cm filtered-namespaces-list -n test-ns -o 'jsonpath={.data.namespaces}' || true
  sleep 1
done

echo "Check status update on parent..."
# We expect at least 1 (metacontroller), but could be more if other namespaces have the label
until [[ "$(kubectl get namespacelists.examples.metacontroller.io filtered-namespaces -n test-ns -o 'jsonpath={.status.count}')" -ge 1 ]]; do sleep 1; done

echo "SUCCESS: NamespaceList generated correctly from cluster-scoped Namespace resources using v2 API"
