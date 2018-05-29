---
title: Hook
classes: wide
---
This page describes how hook targets are defined in various APIs.

Each hook that you define as part of using one of the hook-based APIs
has the following fields:

| Field | Description |
| ----- | ----------- |
| [webhook](#webhook) | Specify how to invoke this hook over HTTP(S). |

## Example

```yaml
webhook:
  url: http://my-controller-svc/sync
```

## Webhook

Each Webhook has the following fields:

| Field | Description |
| ----- | ----------- |
| url | A full URL for the webhook (e.g. `http://my-controller-svc/hook`). If present, this overrides any values provided for `path` and `service`. |
| path | A path to be appended to the accompanying `service` to reach this hook (e.g. `/hook`). Ignored if full `url` is specified. |
| [service](#service-reference) | A reference to a Kubernetes Service through which this hook can be reached. |

### Service Reference

Within a `webhook`, the `service` field has the following subfields:

| Field | Description |
| ----- | ----------- |
| name | The `metadata.name` of the target Service. |
| namespace | The `metadata.namespace` of the target Service. |
| port | The port number to connect to on the target Service. Defaults to `80`. |
| protocol | The protocol to use for the target Service. Defaults to `http`. |