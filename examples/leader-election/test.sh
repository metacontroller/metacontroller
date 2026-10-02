#!/bin/bash

cleanup() {
  exit_code=$?
  set +e
  echo "Rollback metacontroller..."
  kubectl scale deployment --replicas="$previous_replicas" -n metacontroller metacontroller
  kubectl rollout undo deployment metacontroller -n metacontroller
  kubectl rollout status --watch --timeout=180s deployment/metacontroller -n metacontroller
  exit $exit_code
}
trap cleanup EXIT

set -euo

success_msg='Successfully acquired lease'
attempt_msg='Attempting to acquire leader lease'

previous_replicas=$(kubectl get deployment metacontroller -n metacontroller -o=jsonpath='{.spec.replicas}')
kubectl apply -k ./manifest
kubectl rollout status --watch --timeout=180s deployment/metacontroller -n metacontroller

# Both replicas must be ready before checking logs. Wait on the Deployment's
# readyReplicas instead of `kubectl wait --for=condition=ready pod -l`: the
# latter also matches the previous ReplicaSet's Pod while it is terminating,
# and a terminating Pod never becomes ready, so the wait would time out.
replicas=$(kubectl get deployment metacontroller -n metacontroller -o jsonpath='{.spec.replicas}')
kubectl wait --timeout=180s --for=jsonpath='{.status.readyReplicas}'="$replicas" deployment/metacontroller -n metacontroller

# Get the names of the running Pods (Deployments use generated, random names),
# skipping any Pod left over from a previous ReplicaSet that is still terminating.
readarray -t pods < <(kubectl get pods -l app.kubernetes.io/name=metacontroller -n metacontroller \
  -o go-template='{{range .items}}{{if and (eq .status.phase "Running") (not .metadata.deletionTimestamp)}}{{.metadata.name}}{{"\n"}}{{end}}{{end}}')
pod0="${pods[0]}"
pod1="${pods[1]}"

maximum_attempts=36
# wait for one pod to acquire the leader lease
until [[ "$(kubectl logs "$pod0" -n metacontroller | grep "$success_msg" | wc -l)" -eq 1 ||
         "$(kubectl logs "$pod1" -n metacontroller | grep "$success_msg" | wc -l)" -eq 1 ]]; do
  sleep 5
  # timeout at 180s if no leader lease acquired
  ((maximum_attempts--)) # this will exit with an error when equal to zero
done

# determine which pods have attempted or successfully acquired the leader lease
pod0_attempt=$(kubectl logs "$pod0" -n metacontroller | grep "$attempt_msg" | wc -l | xargs echo -n)
pod0_success=$(kubectl logs "$pod0" -n metacontroller | grep "$success_msg" | wc -l | xargs echo -n)
pod1_attempt=$(kubectl logs "$pod1" -n metacontroller | grep "$attempt_msg" | wc -l | xargs echo -n)
pod1_success=$(kubectl logs "$pod1" -n metacontroller | grep "$success_msg" | wc -l | xargs echo -n)

echo
echo "Leader election results:"
echo "$pod0 leader election attempt count: $pod0_attempt, acquired count: $pod0_success"
echo "$pod1 leader election attempt count: $pod1_attempt, acquired count: $pod1_success"
echo

# only one pod should successfully acquire the leader lease
if [[ $((pod0_success + pod1_success)) -eq 0 ]]; then
  exit 1
fi
