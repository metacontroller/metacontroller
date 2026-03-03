#!/bin/bash
# -----------------------------------------------------------------------
# Integration test for ChildUpdateMethod: OnDelete and Recreate
#
# This test verifies that:
#
#   OnDelete  — When a parent spec changes, the child ConfigMap is NOT
#               updated automatically. It is only updated after the child
#               has been manually deleted (and recreated on the next sync).
#
#   Recreate  — When a parent spec changes, Metacontroller uses an SSA
#               dry-run to detect whether controller-managed fields differ.
#               If they do, the child is deleted and recreated from the
#               hook's desired state on the next sync.
#               Crucially:
#               1. Fields owned by a DIFFERENT field manager (out-of-band)
#                  are invisible to the SSA dry-run. They do NOT trigger a
#                  recreation on their own — the child is kept as-is.
#               2. Only when a controller-managed field actually changes
#                  does the delete+recreate happen.
#               3. After a genuine recreation the out-of-band field is
#                  gone, because the object was physically deleted.
#               4. Once the desired state is satisfied the controller must
#                  NOT keep deleting and recreating (no loop).
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
echo " Phase 2: Inject out-of-band field into both child ConfigMaps"
echo "============================================================"
echo "  Patching data.side-effect=injected onto both ConfigMaps ..."
kubectl patch configmap "${ODD_CM}" --type=merge -p '{"data":{"side-effect":"injected"}}'
kubectl patch configmap "${RCD_CM}" --type=merge -p '{"data":{"side-effect":"injected"}}'

odd_se="$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.data.side-effect}' 2>/dev/null)"
rcd_se="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.side-effect}' 2>/dev/null)"
if [[ "${odd_se}" == "injected" && "${rcd_se}" == "injected" ]]; then
  echo "  Both ConfigMaps have side-effect=injected  ✓"
else
  echo "  FAIL: side-effect field not set correctly (odd='${odd_se}', rcd='${rcd_se}')"
  exit 1
fi

echo ""
echo "============================================================"
echo " Phase 2b: Out-of-band field must NOT trigger a recreation"
echo "============================================================"
echo "  The Recreate strategy uses an SSA dry-run that only considers"
echo "  controller-managed fields. A field owned by a different field"
echo "  manager must be invisible to the dry-run and must NOT cause the"
echo "  child to be deleted."
echo ""
echo "  Recording ConfigMap UIDs before the stability window ..."
rcd_uid_initial="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.metadata.uid}')"
odd_uid_initial="$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.metadata.uid}')"
echo "  [Recreate]  UID = ${rcd_uid_initial}"
echo "  [OnDelete]  UID = ${odd_uid_initial}"
echo "  Waiting 30s (≥ 3 sync cycles) without changing the parent ..."
sleep 30

rcd_uid_stable="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.metadata.uid}' 2>/dev/null || echo 'missing')"
if [[ "${rcd_uid_stable}" == "${rcd_uid_initial}" ]]; then
  echo "  [Recreate]  UID unchanged — out-of-band field did NOT trigger a spurious recreation  ✓"
else
  echo "  [Recreate]  FAIL: UID changed from '${rcd_uid_initial}' to '${rcd_uid_stable}'"
  echo "              A spurious recreation was triggered by the out-of-band field!"
  exit 1
fi

rcd_se_stable="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.side-effect}' 2>/dev/null)"
if [[ "${rcd_se_stable}" == "injected" ]]; then
  echo "  [Recreate]  side-effect=injected is still present after stability window  ✓"
else
  echo "  [Recreate]  FAIL: side-effect='${rcd_se_stable}', expected 'injected'"
  echo "              The out-of-band field was lost without a controller-managed change!"
  exit 1
fi

echo ""
echo "============================================================"
echo " Phase 3: Update configData to v2 in both parent instances"
echo "============================================================"
kubectl patch ${odd} odd-instance --type=merge -p '{"spec":{"configData":"v2"}}'
kubectl patch ${rcd} rcd-instance --type=merge -p '{"spec":{"configData":"v2"}}'

echo ""
echo "--- Verifying Recreate strategy (${RCD_CM}) ---"
echo "  Metacontroller should automatically delete and recreate the ConfigMap."
until [[ "$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null)" == "v2" ]]; do sleep 1; done
echo "  [Recreate]  ${RCD_CM} was automatically updated to value=v2  ✓"

# The CM was physically deleted and recreated from the hook's desired state,
# which does not include 'side-effect'. The field must be gone.
rcd_se_after="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.side-effect}' 2>/dev/null)"
if [[ -z "${rcd_se_after}" ]]; then
  echo "  [Recreate]  side-effect field is gone after recreation — physical delete erased out-of-band data  ✓"
else
  echo "  [Recreate]  FAIL: side-effect='${rcd_se_after}' still present after recreation"
  echo "              The ConfigMap was deleted and recreated; out-of-band data should be gone!"
  exit 1
fi

echo ""
echo "--- Verifying Recreate strategy does NOT recreate in a loop (${RCD_CM}) ---"
echo "  Recording ConfigMap UID after the initial recreation ..."
rcd_uid_before="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.metadata.uid}')"
echo "  UID = ${rcd_uid_before}"
echo "  Patching side-effect=injected back onto the recreated CM to verify"
echo "  it does not cause a further recreation ..."
kubectl patch configmap "${RCD_CM}" --type=merge -p '{"data":{"side-effect":"injected"}}'
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
# The value must still be v2 and side-effect still present (no spurious delete)
rcd_value_after="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null || echo 'missing')"
rcd_se_loop="$(kubectl get configmap "${RCD_CM}" -o 'jsonpath={.data.side-effect}' 2>/dev/null)"
if [[ "${rcd_value_after}" == "v2" ]]; then
  echo "  [Recreate]  ${RCD_CM} still has value=v2 after stability window  ✓"
else
  echo "  [Recreate]  FAIL: ${RCD_CM} has value='${rcd_value_after}', expected 'v2'"
  exit 1
fi
if [[ "${rcd_se_loop}" == "injected" ]]; then
  echo "  [Recreate]  side-effect=injected still present — no spurious recreation in steady state  ✓"
else
  echo "  [Recreate]  FAIL: side-effect='${rcd_se_loop}' — out-of-band field was lost without a change!"
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

# The CM was never deleted, so the out-of-band field must still be present.
odd_se_after="$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.data.side-effect}' 2>/dev/null)"
if [[ "${odd_se_after}" == "injected" ]]; then
  echo "  [OnDelete]  side-effect=injected is still present — out-of-band data was preserved  ✓"
else
  echo "  [OnDelete]  FAIL: side-effect field is '${odd_se_after}', expected 'injected'"
  echo "              The OnDelete strategy must keep the existing child intact!"
  exit 1
fi

echo ""
echo "============================================================"
echo " Phase 4: Manually delete the OnDelete child to trigger update"
echo "============================================================"
echo "  Deleting ConfigMap '${ODD_CM}'..."
kubectl delete configmap "${ODD_CM}"

echo "  Waiting for '${ODD_CM}' to be recreated with value=v2 ..."
until [[ "$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.data.value}' 2>/dev/null)" == "v2" ]]; do sleep 1; done
echo "  [OnDelete]  ${ODD_CM} was recreated with value=v2 after manual deletion  ✓"

# After manual deletion the CM was freshly created by the hook — the
# out-of-band field must now be absent.
odd_se_final="$(kubectl get configmap "${ODD_CM}" -o 'jsonpath={.data.side-effect}' 2>/dev/null)"
if [[ -z "${odd_se_final}" ]]; then
  echo "  [OnDelete]  side-effect field is gone after manual delete + recreation  ✓"
else
  echo "  [OnDelete]  FAIL: side-effect='${odd_se_final}' still present after manual recreation"
  echo "              The ConfigMap should have been freshly created from the hook desired state!"
  exit 1
fi

echo ""
echo "============================================================"
echo " All assertions passed!"
echo "============================================================"
echo ""
echo "  OnDelete  The child ConfigMap was NOT updated automatically when the"
echo "            parent spec changed. An out-of-band field (side-effect)"
echo "            patched by a different field manager was preserved"
echo "            throughout. Only after manual deletion was the CM"
echo "            recreated with the new desired state — at which point"
echo "            the out-of-band field was gone."
echo ""
echo "  Recreate  Adding an out-of-band field (different field manager)"
echo "            did NOT trigger a spurious recreation; the SSA dry-run"
echo "            only sees controller-managed fields. Only when a"
echo "            controller-managed field changed did the delete+recreate"
echo "            happen. After that, the out-of-band field was gone"
echo "            (physical delete). Once settled, re-patching the"
echo "            out-of-band field again did not cause a further"
echo "            recreation — confirming no loop."
