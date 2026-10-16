#!/bin/bash

# Metacontroller runs as a StatefulSet (default) or as a Deployment (see
# manifests/deployment). Pick the resource and overlay matching its kind.
if kubectl get -n metacontroller deployment/metacontroller >/dev/null 2>&1; then
  mc=deployment/metacontroller
  mc_overlay=./manifest-deployment
else
  mc=statefulset/metacontroller
  mc_overlay=./manifest
fi

cleanup() {
  exit_code=$?
  set +e
  echo "Rollback metacontroller..."
  kubectl scale "${mc}" --replicas="$previous_replicas" -n metacontroller
  kubectl rollout undo "${mc}" -n metacontroller
  kubectl rollout status --watch --timeout=180s "${mc}" -n metacontroller
  exit $exit_code
}
trap cleanup EXIT

set -euo

success_msg='Successfully acquired lease'
attempt_msg='Attempting to acquire leader lease'

previous_replicas=$(kubectl get "${mc}" -n metacontroller -o=jsonpath='{.spec.replicas}')
kubectl apply -k "$mc_overlay"
kubectl rollout status --watch --timeout=180s "${mc}" -n metacontroller

# Both replicas must be ready before checking logs. Wait on the workload's
# readyReplicas rather than on Pods by label: the latter can also match the
# previous ReplicaSet's Pod while it is terminating, and it never becomes ready.
replicas=$(kubectl get "${mc}" -n metacontroller -o jsonpath='{.spec.replicas}')
kubectl wait --timeout=180s --for=jsonpath='{.status.readyReplicas}'="$replicas" "${mc}" -n metacontroller

# Read the running Pod names dynamically (Deployments use generated names),
# skipping any Pod left over from a previous rollout that is still terminating.
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
