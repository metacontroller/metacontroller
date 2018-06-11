#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f my-statefulset.yaml
  kubectl delete -f service-per-pod.yaml
  kubectl delete svc -l app=service-per-pod
  kubectl delete configmap service-per-pod-hooks -n metacontroller
}
trap cleanup EXIT

set -ex

echo "Install controller..."
kubectl create configmap service-per-pod-hooks -n metacontroller --from-file=hooks
kubectl apply -f service-per-pod.yaml

echo "Create a StatefulSet..."
kubectl apply -f my-statefulset.yaml

echo "Wait for per-pod Service..."
until [[ "$(kubectl get svc nginx-2 -o 'jsonpath={.spec.selector.pod-name}')" == "nginx-2" ]]; do sleep 1; done

echo "Wait for pod-name label..."
until [[ "$(kubectl get pod nginx-2 -o 'jsonpath={.metadata.labels.pod-name}')" == "nginx-2" ]]; do sleep 1; done
