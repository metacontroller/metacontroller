#!/bin/bash
# End-to-end test for webhook TLS verification and authentication.
#
# This test exercises the caBundle, authorization (bearer token), and
# clientTLS (mutual TLS) fields configured via spec.endpointConfigs[] on a
# CompositeController. The webhook server enforces all three simultaneously.
#
# Requirements:
#   - kubectl configured for a running cluster with metacontroller installed.
#   - openssl available on PATH.
#   - Do not run against a production cluster.

# The crd_version argument is accepted for compatibility with the examples/test.sh
# harness but is not used — the CRD is bundled in manifest/kustomization.yaml.
_crd_version=${1:-v1}

TMPDIR=$(mktemp -d)

cleanup() {
  set +e
  echo "Clean up..."
  kubectl delete -f manifest/example-with-wrong-auth.yaml --ignore-not-found
  kubectl delete -f manifest/example-with-good-auth.yaml --ignore-not-found
  kubectl delete -k manifest --ignore-not-found
  kubectl delete secret webhook-auth-server webhook-auth-client \
    -n metacontroller --ignore-not-found
  kubectl delete configmap webhook-auth-ca -n metacontroller --ignore-not-found
  rm -rf "${TMPDIR}"
}
trap cleanup EXIT

set -euo pipefail

# ---------------------------------------------------------------------------
# 1. Generate certificates
# ---------------------------------------------------------------------------

echo "Generating CA..."
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout "${TMPDIR}/ca.key" \
  -out "${TMPDIR}/ca.crt" \
  -days 3650 -nodes \
  -subj "/CN=webhook-auth-test-ca"

echo "Generating server key and CSR..."
openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout "${TMPDIR}/server.key" \
  -out "${TMPDIR}/server.csr" \
  -nodes \
  -subj "/CN=webhook-auth-controller.metacontroller"

cat > "${TMPDIR}/server-ext.cnf" <<EOF
subjectAltName = DNS:webhook-auth-controller.metacontroller,DNS:webhook-auth-controller.metacontroller.svc,DNS:webhook-auth-controller.metacontroller.svc.cluster.local
EOF

echo "Signing server certificate..."
openssl x509 -req \
  -in "${TMPDIR}/server.csr" \
  -CA "${TMPDIR}/ca.crt" -CAkey "${TMPDIR}/ca.key" -CAcreateserial \
  -out "${TMPDIR}/server.crt" \
  -days 3650 \
  -extfile "${TMPDIR}/server-ext.cnf"

echo "Generating client key and CSR..."
openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout "${TMPDIR}/client.key" \
  -out "${TMPDIR}/client.csr" \
  -nodes \
  -subj "/CN=metacontroller"

echo "Signing client certificate..."
openssl x509 -req \
  -in "${TMPDIR}/client.csr" \
  -CA "${TMPDIR}/ca.crt" -CAkey "${TMPDIR}/ca.key" -CAcreateserial \
  -out "${TMPDIR}/client.crt" \
  -days 3650

# ---------------------------------------------------------------------------
# 2. Create cert Secrets and CA ConfigMap in the metacontroller namespace.
#    The bearer token Secrets (webhook-auth-token, webhook-auth-wrong-token)
#    are static manifests applied via kubectl apply -k below.
# ---------------------------------------------------------------------------

echo "Creating webhook-auth-server Secret..."
kubectl create secret generic webhook-auth-server \
  -n metacontroller \
  --from-file=tls.crt="${TMPDIR}/server.crt" \
  --from-file=tls.key="${TMPDIR}/server.key" \
  --from-file=ca.crt="${TMPDIR}/ca.crt"

echo "Creating webhook-auth-client Secret..."
kubectl create secret generic webhook-auth-client \
  -n metacontroller \
  --from-file=tls.crt="${TMPDIR}/client.crt" \
  --from-file=tls.key="${TMPDIR}/client.key"

echo "Creating webhook-auth-ca ConfigMap..."
kubectl create configmap webhook-auth-ca \
  -n metacontroller \
  --from-file=ca.crt="${TMPDIR}/ca.crt"

# ---------------------------------------------------------------------------
# 3. Install the CRD, namespaces, both controllers, and webhook server
# ---------------------------------------------------------------------------

echo "Installing..."
kubectl apply -k manifest

# ---------------------------------------------------------------------------
# 4. Apply the valid auth instance (source Secret + parent CR)
# ---------------------------------------------------------------------------

echo "Creating valid auth parent CR and source Secret..."
kubectl apply -f manifest/example-with-good-auth.yaml

# ---------------------------------------------------------------------------
# 5. Positive assertions
# ---------------------------------------------------------------------------

echo "Waiting for secret propagation to secure-alpha..."
until [[ "$(kubectl get secret shareable -n secure-alpha -o 'jsonpath={.metadata.name}' 2>/dev/null)" == "shareable" ]]; do
  sleep 1
done

echo "Waiting for secret propagation to secure-beta..."
until [[ "$(kubectl get secret shareable -n secure-beta -o 'jsonpath={.metadata.name}' 2>/dev/null)" == "shareable" ]]; do
  sleep 1
done

echo "Checking that secure-gamma (no matching label) did not receive the secret..."
if kubectl get secret shareable -n secure-gamma 2>/dev/null | grep -q shareable; then
  echo "FAIL: secret was propagated to secure-gamma, which has no matching label"
  exit 1
fi

echo "Checking status update on parent..."
until [[ "$(kubectl get SecureSecretPropagation secure-secret-propagation -o 'jsonpath={.status.working}')" == "fine" ]]; do
  sleep 1
done

echo "All valid auth assertions passed."

# ---------------------------------------------------------------------------
# 6. Invalid auth test: wrong bearer token must prevent propagation.
#
# example-with-wrong-auth.yaml creates a source Secret and a parent CR with
# label auth: invalid. The already-installed webhook-auth-controller-invalid
# picks it up and calls the webhook server with the wrong token, which returns
# 401. Metacontroller emits a SyncError Warning event on the parent object's
# namespace (default, because the CRD is cluster-scoped). We wait for that
# event and then confirm the child secret was never created.
# ---------------------------------------------------------------------------

echo ""
echo "=== Invalid auth test: wrong bearer token ==="

echo "Creating invalid auth source Secret and parent CR..."
kubectl apply -f manifest/example-with-wrong-auth.yaml

echo "Waiting for SyncError event on the invalid auth parent CR..."
until kubectl get events -n default \
    --field-selector reason=SyncError 2>/dev/null \
    | grep -q "secure-secret-propagation-invalid"; do
  sleep 2
done
echo "SyncError event received."

echo "Confirming shareable-invalid was not propagated to secure-alpha..."
if kubectl get secret shareable-invalid -n secure-alpha 2>/dev/null | grep -q shareable-invalid; then
  echo "FAIL: shareable-invalid was propagated to secure-alpha despite wrong bearer token"
  exit 1
fi
echo "Confirmed: secret not propagated with wrong bearer token."

echo ""
echo "All assertions passed."
