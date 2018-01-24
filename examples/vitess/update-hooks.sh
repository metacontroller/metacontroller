#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

kubectl_output="$(kubectl create configmap vitess-operator-hooks -n metacontroller --from-file=hooks --append-hash)"
echo "${kubectl_output}"
expr='configmap "(.+)" created'
if [[ "${kubectl_output}" =~ $expr ]]; then
  configmap="${BASH_REMATCH[1]}"
  patch="{\"spec\":{\"template\":{\"spec\":{\"volumes\":[{\"name\":\"hooks\",\"configMap\":{\"name\":\"${configmap}\"}}]}}}}"
  kubectl patch deployment -n metacontroller vitess-operator -p "${patch}"
fi
