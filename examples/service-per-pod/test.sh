#!/bin/bash

cleanup() {
  set +e
  echo "Clean up..."
  kubectl patch statefulset nginx --type=merge -p '{"metadata":{"finalizers":[]}}'
  kubectl delete -f my-statefulset.yaml
  kubectl delete -k manifest
  kubectl delete svc -l app=service-per-pod
}
trap cleanup EXIT

set -euo

finalizer="metacontroller.io/service-per-pod-test"

echo "Install controller..."
kubectl apply -k manifest

echo "Create a StatefulSet..."
kubectl apply -f my-statefulset.yaml

echo "Wait for per-pod Service..."
until [[ "$(kubectl get svc nginx-2 -o 'jsonpath={.spec.selector.pod-name}')" == "nginx-2" ]]; do sleep 1; done

echo "Wait for pod-name label..."
until [[ "$(kubectl get pod nginx-2 -o 'jsonpath={.metadata.labels.pod-name}')" == "nginx-2" ]]; do sleep 1; done

echo "Remove annotation to opt out of service-per-pod without deleting the StatefulSet..."
kubectl annotate statefulset nginx service-per-pod-label-

echo "Wait for per-pod Service to get cleaned up by the decorator's finalizer..."
until [[ "$(kubectl get svc nginx-2 2>&1)" == *NotFound* ]]; do sleep 1; done

echo "Wait for the decorator's finalizer to be removed..."
while [[ "$(kubectl get statefulset nginx -o 'jsonpath={.metadata.finalizers}')" == *decoratorcontroller-service-per-pod* ]]; do sleep 1; done

echo "Add the annotation back to opt in again..."
kubectl annotate statefulset nginx service-per-pod-label=pod-name

echo "Wait for per-pod Service to come back..."
until [[ "$(kubectl get svc nginx-2 -o 'jsonpath={.spec.selector.pod-name}')" == "nginx-2" ]]; do sleep 1; done

echo "Append our own finalizer so we can check deletion ordering..."
kubectl patch statefulset nginx --type=json -p '[{"op":"add","path":"/metadata/finalizers/-","value":"'${finalizer}'"}]'

echo "Delete the StatefulSet..."
kubectl delete statefulset nginx --wait=false

echo "Wait for per-pod Service to get cleaned up by the decorator's finalizer..."
until [[ "$(kubectl get svc nginx-2 2>&1)" == *NotFound* ]]; do sleep 1; done

echo "Wait for the decorator's finalizer to be removed..."
while [[ "$(kubectl get statefulset nginx -o 'jsonpath={.metadata.finalizers}')" == *decoratorcontroller-service-per-pod* ]]; do sleep 1; done
