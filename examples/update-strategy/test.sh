#!/bin/bash

# Copyright 2024 Metacontroller Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# -----------------------------------------------------------------------
# Integration test for ChildUpdateMethod: OnDelete and Recreate
#
# This test verifies that:
#
#   OnDelete  — When a parent spec changes, the child ConfigMap is NOT
#               updated automatically. It is only updated after the child
#               has been manually deleted (and recreated on the next sync).
#
#   Recreate  — When a parent spec changes, Metacontroller automatically
#               deletes the child ConfigMap. It is then recreated with the
#               new desired state on the next sync iteration. Crucially,
#               once the desired state is satisfied the child must NOT be
#               deleted and recreated again (no recreation loop).
#
# Two separate CRDs and CompositeControllers are used, one for each
# strategy, so their behaviours can be observed independently.
#
# Instance names and their derived ConfigMap names:
#   OnDeleteDemo  odd-instance  ->  odd-instance-data
#   RecreateDemo  rcd-instance  ->  rcd-instance-data
# -----------------------------------------------------------------------

crd_version=${1:-v1}

ODD_CM="odd-instance-data"
RCD_CM="rcd-instance-data"

cleanup() {
  set +e
  echo ""
  echo "--- Cleaning up... ---"
  kubectl delete -f my-ondeletedemo.yaml  2>/dev/null
  kubectl delete -f my-recreatedemo.yaml  2>/dev/null
  kubectl delete -k manifest              2>/dev/null
  kubectl delete -k "${crd_version}"      2>/dev/null
}
trap cleanup EXIT

set -euo pipefail

odd="ondeletedemos"
rcd="recreatedemos"

echo "=== Installing CRDs ==="
kubectl apply -k "${crd_version}"

echo "=== Waiting until CRDs are available ==="
until kubectl get ${odd} 2>/dev/null; do sleep 1; done
until kubectl get ${rcd} 2>/dev/null; do sleep 1; done

echo "=== Installing controllers ==="
kubectl apply -k manifest

echo "=== Waiting for controller Deployment to be ready ==="
kubectl rollout status deployment/update-strategy-controller \
  -n metacontroller --timeout=120s

echo ""
echo "============================================================"
echo " Phase 1: Create parent instances with configData=v1"
echo "============================================================"
kubectl apply -f my-ondeletedemo.yaml
kubectl apply -f my-recreatedemo.yaml

echo "Waiting for child ConfigMap '${ODD_CM}' to appear with value=v1 ..."
until [[ "$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null)" == "v1" ]]; do sleep 1; done
echo "  [OnDelete]  ${ODD_CM} has value=v1  ✓"

echo "Waiting for child ConfigMap '${RCD_CM}' to appear with value=v1 ..."
until [[ "$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null)" == "v1" ]]; do sleep 1; done
echo "  [Recreate]  ${RCD_CM} has value=v1  ✓"

echo ""
echo "============================================================"
echo " Phase 2: Update configData to v2 in both parent instances"
echo "============================================================"
kubectl patch ${odd} odd-instance --type=merge -p '{"spec":{"configData":"v2"}}'
kubectl patch ${rcd} rcd-instance --type=merge -p '{"spec":{"configData":"v2"}}'

echo ""
echo "--- Verifying Recreate strategy (${RCD_CM}) ---"
echo "  Metacontroller should automatically delete and recreate the ConfigMap."
until [[ "$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null)" == "v2" ]]; do sleep 1; done
echo "  [Recreate]  ${RCD_CM} was automatically updated to value=v2  ✓"

echo ""
echo "--- Verifying Recreate strategy does NOT recreate in a loop (${RCD_CM}) ---"
echo "  Recording ConfigMap UID after the initial recreation ..."
rcd_uid_before="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.metadata.uid}')"
echo "  UID = ${rcd_uid_before}"
echo "  Waiting 30s (≥ 3 sync cycles) to observe any unwanted deletions ..."
sleep 30
rcd_uid_after="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.metadata.uid}' 2>/dev/null || echo 'missing')"
if [[ "${rcd_uid_after}" == "${rcd_uid_before}" ]]; then
  echo "  [Recreate]  UID is unchanged (${rcd_uid_after}) — no recreation loop detected  ✓"
else
  echo "  [Recreate]  FAIL: UID changed from '${rcd_uid_before}' to '${rcd_uid_after}'"
  echo "              The Recreate strategy triggered a recreation loop!"
  exit 1
fi
# Also assert the value is still v2 (not lost due to a loop-induced transient state)
rcd_value_after="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null || echo 'missing')"
if [[ "${rcd_value_after}" == "v2" ]]; then
  echo "  [Recreate]  ${RCD_CM} still has value=v2 after stability window  ✓"
else
  echo "  [Recreate]  FAIL: ${RCD_CM} has value='${rcd_value_after}', expected 'v2' after stability check"
  exit 1
fi

echo ""
echo "--- Verifying OnDelete strategy (${ODD_CM}) ---"
echo "  Waiting 10s to confirm Metacontroller does NOT update the ConfigMap..."
sleep 10

odd_value="$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null || echo 'missing')"
if [[ "${odd_value}" == "v1" ]]; then
  echo "  [OnDelete]  ${ODD_CM} still has value=v1 after parent update — update was correctly withheld  ✓"
else
  echo "  [OnDelete]  FAIL: ${ODD_CM} has value='${odd_value}', expected 'v1'"
  echo "              The OnDelete strategy must NOT update children automatically!"
  exit 1
fi

echo ""
echo "============================================================"
echo " Phase 3: Manually delete the OnDelete child to trigger update"
echo "============================================================"
echo "  Deleting ConfigMap '${ODD_CM}'..."
kubectl delete configmap "${ODD_CM}"

echo "  Waiting for '${ODD_CM}' to be recreated with value=v2 ..."
until [[ "$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null)" == "v2" ]]; do sleep 1; done
echo "  [OnDelete]  ${ODD_CM} was recreated with value=v2 after manual deletion  ✓"

echo ""
echo "============================================================"
echo " All assertions passed!"
echo "============================================================"
echo ""
echo "  OnDelete  The child ConfigMap was NOT updated automatically when the"
echo "            parent spec changed. Only after manual deletion was it"
echo "            recreated with the new desired state."
echo ""
echo "  Recreate  The child ConfigMap was automatically deleted by"
echo "            Metacontroller when the parent spec changed, and was"
echo "            recreated with the new desired state on the next sync."
echo "            After settling, the ConfigMap was NOT deleted again —"
echo "            confirming that no recreation loop occurred."
