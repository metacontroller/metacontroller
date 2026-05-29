# Hook

This page describes how hook targets are defined in various APIs.

Each hook that you define as part of using one of the hook-based APIs
has the following fields:

| Field | Description |
| ----- | ----------- |
| `version` | The version of the hook API to use. Can be `v1` or `v2`. Defaults to `v1`. |
| [`webhook`](#webhook) | Specify how to invoke this hook over HTTP(S). |

[[_TOC_]]

## Example

```yaml
webhook:
  url: http://my-controller-svc/sync
```

## Webhook

Each Webhook has the following fields:

| Field                        | Description                                                                                                                                                                                                                             |
|------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [etag](#etag-reference)      | A configuration for etag logic                                                                                                                                                                                                          |
| url                          | A full URL for the webhook (e.g. `http://my-controller-svc/hook`). If present, this overrides any values provided for `path` and `service`.                                                                                             |
| timeout                      | A duration (in the format of Go's time.Duration) indicating the time that Metacontroller should wait for a resserviceponse. If the webhook takes longer than this time, the webhook call is aborted and retried later. Defaults to 10s. |
| path                         | A path to be appended to the accompanying `service` to reach this hook (e.g. `/hook`). Ignored if full `url` is specified.                                                                                                              |
| [service](#service-reference) | A reference to a Kubernetes Service through which this hook can be reached.                                                                                                                                                             |
| responseUnMarshallMode | Sets the JSON unmarshall mode. One of `loose` or `strict`. In `strict` mode, additional checks are performed to detect unknown and duplicated fields. **Default:** `loose` for `v1` hooks, `strict` for `v2` hooks. |
| [caBundle](#cabundle-reference) | Configures the CA certificate(s) used to verify the webhook server's TLS certificate when the endpoint uses HTTPS with a private or self-signed CA. If omitted, the system trust roots are used. |

### Service Reference

Within a `webhook`, the `service` field has the following subfields:

| Field | Description |
| ----- | ----------- |
| name | The `metadata.name` of the target Service. |
| namespace | The `metadata.namespace` of the target Service. |
| port | The port number to connect to on the target Service. Defaults to `80`. |
| protocol | The protocol to use for the target Service. Defaults to `http`. |

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

| Field                | Description                                                              |
|----------------------|--------------------------------------------------------------------------|
| Enabled              | true or false. Default is false                                          |
| CacheTimeoutSeconds  | Time in seconds after which ETag cache record is forgotten               |
| CacheCleanupSeconds  | How often ETag is running garbage collector to cleanup forgotten records |

### CABundle Reference

The `caBundle` field configures the CA certificate(s) used to verify the TLS certificate presented
by the webhook server. It is only needed when the webhook endpoint uses HTTPS with a **private or
self-signed CA**. For publicly-trusted CAs (e.g. Let's Encrypt), omit this field entirely.

Exactly one of the following sources must be specified:

| Field          | Description |
|----------------|-------------|
| `inline`       | PEM-encoded CA certificate(s) embedded directly in the spec. |
| `secretRef`    | A reference to a key in a Kubernetes Secret containing PEM-encoded CA certificate(s). |
| `configMapRef` | A reference to a key in a Kubernetes ConfigMap containing PEM-encoded CA certificate(s). |

For both `secretRef` and `configMapRef`, the referenced object has the following subfields:

| Field       | Description |
|-------------|-------------|
| `name`      | The `metadata.name` of the Secret or ConfigMap. |
| `namespace` | The `metadata.namespace` of the Secret or ConfigMap. |
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
      name: my-tls-secret   # Secret of type kubernetes.io/tls
      namespace: my-ns
      key: ca.crt            # optional; defaults to "ca.crt"
```

**ConfigMap reference**:

```yaml
webhook:
  url: https://my-hook.my-ns:8443/sync
  caBundle:
    configMapRef:
      name: my-ca-bundle
      namespace: my-ns
      key: ca.crt            # optional; defaults to "ca.crt"
```

> **Note on CA rotation:** The CA bundle is resolved once when the controller CR is created or
> updated. If the referenced Secret or ConfigMap changes (e.g. due to certificate rotation),
> metacontroller will not automatically pick up the new value. To force a reload, update the
> controller CR (e.g. add or change an annotation) to trigger re-creation of the webhook executor.
