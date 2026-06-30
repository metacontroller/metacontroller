# Hook

This page describes how hook targets are defined in various APIs.

Each hook that you define as part of using one of the hook-based APIs
has the following fields:

| Field                 | Description                                                                |
| --------------------- | -------------------------------------------------------------------------- |
| `version`             | The version of the hook API to use. Can be `v1` or `v2`. Defaults to `v1`. |
| [`webhook`](#webhook) | Specify how to invoke this hook over HTTP(S).                              |

[[_TOC_]]

## Example

```yaml
webhook:
  url: http://my-controller-svc/sync
```

## Webhook

Each Webhook has the following fields:

| Field                                     | Description                                                                                                                                                                                                                             |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [etag](#etag-reference)                   | A configuration for etag logic                                                                                                                                                                                                          |
| url                                       | A full URL for the webhook (e.g. `http://my-controller-svc/hook`). If present, this overrides any values provided for `path` and `service`.                                                                                             |
| timeout                                   | A duration (in the format of Go's time.Duration) indicating the time that Metacontroller should wait for a resserviceponse. If the webhook takes longer than this time, the webhook call is aborted and retried later. Defaults to 10s. |
| path                                      | A path to be appended to the accompanying `service` to reach this hook (e.g. `/hook`). Ignored if full `url` is specified.                                                                                                              |
| [service](#service-reference)             | A reference to a Kubernetes Service through which this hook can be reached.                                                                                                                                                             |
| responseUnMarshallMode                    | Sets the JSON unmarshall mode. One of `loose` or `strict`. In `strict` mode, additional checks are performed to detect unknown and duplicated fields. **Default:** `loose` for `v1` hooks, `strict` for `v2` hooks.                     |
| [caBundle](#cabundle-reference)           | Configures the CA certificate(s) used to verify the webhook server's TLS certificate when the endpoint uses HTTPS with a private or self-signed CA. If omitted, the system trust roots are used.                                        |
| [authorization](#authorization-reference) | Configures a token-based `Authorization` request header (e.g. Bearer). Mutually exclusive with `basicAuth`.                                                                                                                             |
| [basicAuth](#basicauth-reference)         | Configures HTTP Basic Authentication. Mutually exclusive with `authorization`.                                                                                                                                                          |
| [clientTLS](#clienttls-reference)         | Configures a client TLS certificate for mutual TLS (mTLS). Can be combined with any authentication method or used alone.                                                                                                                |

### Service Reference

Within a `webhook`, the `service` field has the following subfields:

| Field     | Description                                                            |
| --------- | ---------------------------------------------------------------------- |
| name      | The `metadata.name` of the target Service.                             |
| namespace | The `metadata.namespace` of the target Service.                        |
| port      | The port number to connect to on the target Service. Defaults to `80`. |
| protocol  | The protocol to use for the target Service. Defaults to `http`.        |

### Etag Reference

More details in [rfc7232](https://www.rfc-editor.org/rfc/rfc7232).

Etag is a hash of response content, controller that supports etag notion should add "ETag" header to each 200 response.
Metacontrollers that support "ETag" should send the "If-None-Match" header with value of ETag of cached content.
If content has not changed, controller should reply with "304 Not modified" or "412 Precondition Failed", otherwise it sends 200 with "ETag" header.

This logic helps save traffic and CPU time on webhook processing.

Within a `webhook`, the `eTag` field has the following subfields:

    Enabled             *bool  `json:"enabled,omitempty"`
    CacheTimeoutSeconds *int32 `json:"cacheTimeoutSeconds,omitempty"`
    CacheCleanupSeconds *int32 `json:"cacheCleanupSeconds,omitempty"`

| Field               | Description                                                              |
| ------------------- | ------------------------------------------------------------------------ |
| Enabled             | true or false. Default is false                                          |
| CacheTimeoutSeconds | Time in seconds after which ETag cache record is forgotten               |
| CacheCleanupSeconds | How often ETag is running garbage collector to cleanup forgotten records |

### CABundle Reference

The `caBundle` field configures the CA certificate(s) used to verify the TLS certificate presented
by the webhook server. It is only needed when the webhook endpoint uses HTTPS with a **private or
self-signed CA**. For publicly-trusted CAs (e.g. Let's Encrypt), omit this field entirely.

Exactly one of the following sources must be specified:

| Field          | Description                                                                              |
| -------------- | ---------------------------------------------------------------------------------------- |
| `inline`       | PEM-encoded CA certificate(s) embedded directly in the spec.                             |
| `secretRef`    | A reference to a key in a Kubernetes Secret containing PEM-encoded CA certificate(s).    |
| `configMapRef` | A reference to a key in a Kubernetes ConfigMap containing PEM-encoded CA certificate(s). |

For both `secretRef` and `configMapRef`, the referenced object has the following subfields:

| Field       | Description                                                                      |
| ----------- | -------------------------------------------------------------------------------- |
| `name`      | The `metadata.name` of the Secret or ConfigMap.                                  |
| `namespace` | The `metadata.namespace` of the Secret or ConfigMap.                             |
| `key`       | The key within the resource's `data` map. Defaults to `ca.crt` if not specified. |

#### Examples

**Inline PEM**:

```yaml
webhook:
  url: https://my-hook.my-ns:8443/sync
  caBundle:
    inline: |
      -----BEGIN CERTIFICATE-----
      MIIDxTCCAq2gAwIB...
      -----END CERTIFICATE-----
```

**Secret reference**:

```yaml
webhook:
  url: https://my-hook.my-ns:8443/sync
  caBundle:
    secretRef:
      name: my-tls-secret # Secret of type kubernetes.io/tls
      namespace: my-ns
      key: ca.crt # optional; defaults to "ca.crt"
```

**ConfigMap reference**:

```yaml
webhook:
  url: https://my-hook.my-ns:8443/sync
  caBundle:
    configMapRef:
      name: my-ca-bundle
      namespace: my-ns
      key: ca.crt # optional; defaults to "ca.crt"
```

> **Note on CA rotation:** The CA bundle is resolved once when the controller CR is created or
> updated. If the referenced Secret or ConfigMap changes (e.g. due to certificate rotation),
> metacontroller will not automatically pick up the new value. To force a reload, update the
> controller CR (e.g. add or change an annotation) to trigger re-creation of the webhook executor.

### Authorization Reference

The `authorization` field adds a token-based `Authorization` header to every webhook request.
`authorization` and `basicAuth` are mutually exclusive — only one may be set per webhook or
connection entry.

The `type` field sets the scheme prefix (e.g. `Bearer`, `Token`).
// +kubebuilder:default="Bearer"

| Field       | Description                                                                                                                                         |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `type`      | The authorization scheme, e.g. `Bearer` or `Token`. Defaults to `Bearer`. Must not be `Basic` — use the `basicAuth` field for Basic authentication. |
| `secretRef` | Reference to a Kubernetes Secret key containing the credential value.                                                                               |

The `secretRef` sub-object has the following fields:

| Field       | Description                                                                                  |
| ----------- | -------------------------------------------------------------------------------------------- |
| `name`      | The `metadata.name` of the Secret.                                                           |
| `namespace` | The `metadata.namespace` of the Secret.                                                      |
| `key`       | The key within the Secret's `data` map whose value is the credential. Required — no default. |

#### Example

```yaml
webhook:
  url: https://my-hook.my-ns:8443/sync
  authorization:
    type: Bearer
    secretRef:
      name: my-token-secret
      namespace: my-ns
      key: token
```

### BasicAuth Reference

The `basicAuth` field configures HTTP Basic Authentication. The username and password are read
from a single Kubernetes Secret. `basicAuth` and `authorization` are mutually exclusive.

| Field         | Description                                                                                 |
| ------------- | ------------------------------------------------------------------------------------------- |
| `secretRef`   | Reference to the Kubernetes Secret containing the credentials.                              |
| `usernameKey` | The key within the Secret's `data` map whose value is the username. Defaults to `username`. |
| `passwordKey` | The key within the Secret's `data` map whose value is the password. Defaults to `password`. |

The `secretRef` sub-object has the following fields:

| Field       | Description                             |
| ----------- | --------------------------------------- |
| `name`      | The `metadata.name` of the Secret.      |
| `namespace` | The `metadata.namespace` of the Secret. |

#### Example

```yaml
webhook:
  url: https://my-hook.my-ns:8443/sync
  basicAuth:
    secretRef:
      name: my-basic-auth-secret
      namespace: my-ns
    usernameKey: username # optional; defaults to "username"
    passwordKey: password # optional; defaults to "password"
```

### ClientTLS Reference

The `clientTLS` field configures a client certificate presented during the TLS handshake for
mutual TLS (mTLS). It can be combined with any authentication method or used alone.

| Field           | Description                                                                                                      |
| --------------- | ---------------------------------------------------------------------------------------------------------------- |
| `secretRef`     | Reference to the Kubernetes Secret containing the certificate and private key.                                   |
| `certKey`       | The key within the Secret's `data` map whose value is the PEM-encoded client certificate. Defaults to `tls.crt`. |
| `privateKeyKey` | The key within the Secret's `data` map whose value is the PEM-encoded private key. Defaults to `tls.key`.        |

The `secretRef` sub-object has the following fields:

| Field       | Description                             |
| ----------- | --------------------------------------- |
| `name`      | The `metadata.name` of the Secret.      |
| `namespace` | The `metadata.namespace` of the Secret. |

#### Example

```yaml
webhook:
  url: https://my-hook.my-ns:8443/sync
  clientTLS:
    secretRef:
      name: my-client-tls-secret
      namespace: my-ns
    certKey: tls.crt # optional; defaults to "tls.crt"
    privateKeyKey: tls.key # optional; defaults to "tls.key"
```

## Endpoint Configs

The `endpointConfigs` field on a `CompositeController` or `DecoratorController` lets you define
per-host connection settings (CA bundle, client TLS, and authentication) that apply to all
webhook hooks whose URL matches a given host. Per-hook fields (if any are set) fully override the
matching endpointConfigs entry for that hook — there is no field-level merging.

```yaml
spec:
  endpointConfigs:
    - host: my-hook.my-ns
      caBundle:
        secretRef:
          name: my-ca-secret
          namespace: my-ns
      authorization:
        secretRef:
          name: my-token-secret
          namespace: my-ns
          key: token
```

Each entry in `endpointConfigs` supports the following fields:

| Field           | Description                                                                                                                                                                                                                                                                                                                                                   |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `host`          | The hostname (optionally with port) of the webhook endpoint, e.g. `my-hook.my-ns` or `my-hook.my-ns:8443`. Matching is case-insensitive. Default HTTPS port (443) and HTTP port (80) are treated as equivalent to omitting the port. Applies to both `url`-form and `service`-form webhooks — for the latter, the host is derived as `name.namespace[:port]`. |
| `caBundle`      | See [CABundle Reference](#cabundle-reference).                                                                                                                                                                                                                                                                                                                |
| `authorization` | See [Authorization Reference](#authorization-reference).                                                                                                                                                                                                                                                                                                      |
| `basicAuth`     | See [BasicAuth Reference](#basicauth-reference).                                                                                                                                                                                                                                                                                                              |
| `clientTLS`     | See [ClientTLS Reference](#clienttls-reference).                                                                                                                                                                                                                                                                                                              |
