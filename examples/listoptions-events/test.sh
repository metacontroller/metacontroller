#!/bin/bash

crd_version=${1:-v1}

cleanup() {
  exit_code=$?
  set +e
  echo "Delete event wrapper CRs..."
  kubectl delete -f my-eventwrapper.yaml

  echo "Delete Ew controller..."
  kubectl delete -k "${crd_version}"

  echo "Rollback metacontroller..."
  kubectl rollout undo statefulset metacontroller -n metacontroller
  kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller

  echo "Delete namespaces..."
  kubectl delete namespace ew-ns1
  kubectl delete namespace ew-ns2

  exit $exit_code
}
trap cleanup EXIT

set -ex

ew="eventwrappers"

echo "Creating namespaces..."
# create namespace if it does not exist
kubectl create namespace ew-ns1 --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace ew-ns2 --dry-run=client -o yaml | kubectl apply -f -

echo "Enable list options on metacontroller..."
kubectl apply -k ./manifest
kubectl rollout status --watch --timeout=180s statefulset/metacontroller -n metacontroller


echo "Install Ew controller..."
kubectl apply -k "${crd_version}"
kubectl rollout status --watch --timeout=180s deployment/ew-controller -n metacontroller


echo "Create event wrapper CRs..."
kubectl apply -f my-eventwrapper.yaml

# wait for events to be created
until [[ "$(kubectl get events --all-namespaces | grep -E  'Ew[0-9]' | wc -l)" -eq "8" ]]; do sleep 1; done


# logs of metacontroller-0 should only contain Ew7 and Ew8 events
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew1 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew2 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew3 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew4 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew5 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew6 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew7 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs
if [[ "$(kubectl -n metacontroller logs metacontroller-0 | grep Ew8 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs

# logs of metacontroller-1 should not contain any Ew events
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew1 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew2 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew3 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew4 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew5 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew6 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew7 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-1 | grep Ew8 | wc -l)" -ne "0" ]]; then exit 1; fi

# logs of metacontroller-2 should only contain Ew1 and Ew2 events
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew1 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew2 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew3 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew4 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew5 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew6 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew7 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-2 | grep Ew8 | wc -l)" -ne "0" ]]; then exit 1; fi

# logs of metacontroller-3 should only contain Ew3, Ew4, Ew5, Ew6 events
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew1 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew2 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew3 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew4 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew5 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew6 | wc -l)" -eq "0" ]]; then exit 1; fi # should have logs
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew7 | wc -l)" -ne "0" ]]; then exit 1; fi
if [[ "$(kubectl -n metacontroller logs metacontroller-3 | grep Ew8 | wc -l)" -ne "0" ]]; then exit 1; fi
