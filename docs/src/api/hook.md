# Hook

This page describes how hook targets are defined in various APIs.

Each hook that you define as part of using one of the hook-based APIs
has the following fields:

| Field | Description |
| ----- | ----------- |
| [webhook](#webhook) | Specify how to invoke this hook over HTTP(S). |

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
