# webhook-auth example

This example demonstrates webhook TLS verification and authentication in
metacontroller, using a `CompositeController` that propagates a Kubernetes
`Secret` across namespaces — the same scenario as the
[secretpropagation](../secretpropagation) example, but with the webhook
server running over HTTPS and enforcing mutual TLS and bearer-token
authentication.

All three security mechanisms are configured via a single `spec.endpointConfigs[]`
entry on the controller, which applies to both the `sync` and `customize`
hooks automatically.

## What is exercised

| Feature                         | Configuration                                                      |
| ------------------------------- | ------------------------------------------------------------------ |
| Server TLS verification         | `endpointConfigs[].caBundle.configMapRef` — CA cert in a ConfigMap |
| Bearer token authentication     | `endpointConfigs[].authorization.secretRef` — token in a Secret    |
| Mutual TLS (client certificate) | `endpointConfigs[].clientTLS.secretRef` — cert + key in a Secret   |

## How it works

The webhook server (`manifest/sync.py`) listens on port 8443 with:

- A TLS server certificate signed by a private CA.
- Client certificate verification (`ssl.CERT_REQUIRED`) against the same CA.
- A bearer-token check on every incoming request. The expected token is read
  at runtime from the `webhook-auth-token` Secret mounted at `/token`.

Metacontroller reads the CA bundle, client certificate, and bearer token from
Kubernetes Secrets and a ConfigMap at controller CR creation time, then uses
them for every webhook call.

## Prerequisites

- `kubectl` configured for a cluster with metacontroller installed.
- `openssl` on `PATH`.

## Running the test

The test script generates all certificates at runtime using `openssl`, creates
the required Secrets and ConfigMap, installs the controller, and verifies that
the secret is propagated to the expected namespaces.

```bash
cd examples/webhook-auth
./test.sh
```
