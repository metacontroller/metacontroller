#!/bin/bash

cleanup() {
  exit_code=$?
  set +e
  echo "Rollback metacontroller..."
  kubectl scale statefulset --replicas="$previous_replicas" -n metacontroller metacontroller
  kubectl rollout undo statefulset metacontroller -n metacontroller
  kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller
  exit $exit_code
}
trap cleanup EXIT

set -ex

success_msg='successfully acquired lease'
attempt_msg='attempting to acquire leader lease'

previous_replicas=$(kubectl get statefulset metacontroller -n metacontroller -o=jsonpath='{.spec.replicas}')
kubectl apply -k ./manifest
kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller

# both pods must be ready before checking logs
kubectl wait --timeout=180s --for=condition=ready pod metacontroller-0 -n metacontroller
kubectl wait --timeout=180s --for=condition=ready pod metacontroller-1 -n metacontroller

maximum_attempts=36
# wait for one pod to acquire the leader lease
until [[ "$(kubectl logs metacontroller-0 -n metacontroller | grep "$success_msg" | wc -l)" -eq 1 ||
         "$(kubectl logs metacontroller-1 -n metacontroller | grep "$success_msg" | wc -l)" -eq 1 ]]; do
  sleep 5
  # timeout at 180s if no leader lease acquired
  ((maximum_attempts--)) # this will exit with an error when equal to zero
done

# determine which pods have attempted or successfully acquired the leader lease
pod0_attempt=$(kubectl logs metacontroller-0 -n metacontroller | grep "$attempt_msg" | wc -l | xargs echo -n)
pod0_success=$(kubectl logs metacontroller-0 -n metacontroller | grep "$success_msg" | wc -l | xargs echo -n)
pod1_attempt=$(kubectl logs metacontroller-1 -n metacontroller | grep "$attempt_msg" | wc -l | xargs echo -n)
pod1_success=$(kubectl logs metacontroller-1 -n metacontroller | grep "$success_msg" | wc -l | xargs echo -n)

echo
echo "Leader election results:"
echo "metacontroller-0 leader election attempt count: $pod0_attempt, acquired count: $pod0_success"
echo "metacontroller-1 leader election attempt count: $pod1_attempt, acquired count: $pod1_success"
echo

# only one pod should successfully acquire the leader lease
if [[ $((pod0_success + pod1_success)) -eq 0 ]]; then
  exit 1
fi
