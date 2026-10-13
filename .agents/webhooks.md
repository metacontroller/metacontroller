# WEBHOOK CALLS (`pkg/hooks/`)

Each call from a controller to a user webhook goes through this package:
endpoint resolution, authorization, CA bundles, and the hook call for each
hook API version.

## WHERE TO LOOK
| Task | File | Main names |
|------|------|------------|
| Hook interface and dispatch | `hooks.go` | `Hook`, `NewHook`, `IsEnabled`, `GetVersion`, `Call` |
| HTTP call | `webhook.go` | `HttpClientInterface`, `WebhookExecutor`, `NewWebhookExecutor` |
| Plain call (no ETag) | `webhook_plain.go` | |
| ETag (conditional response) | `webhook_etag.go` | the cached response is used when the ETag did not change |
| Endpoint resolution | `endpoint_config.go` | `ResolveEndpointConfig` |
| Authorization: basic, bearer, client TLS | `auth.go` | `ResolveAuthorization`, `ResolveBasicAuth`, `ResolveClientTLS` |
| CA bundle from a Secret or a ConfigMap | `cabundle.go` | `ResolveCABundle` |
| Rate-limit signal | `errors.go` | `TooManyRequestError` (HTTP 429) |

Each source file has a `_test.go` file. Fakes are in `pkg/internal/testutils/hooks`.

## RULES
- Test names: `Test<Func>_when<Condition>_<expected>`, for example
  `TestResolveBasicAuth_whenSecretMissing_returnsError`. Do not use plain `TestFunc`.
- The tests are almost as large as the code. New behavior needs a test of
  equal weight.
- `ResolveEndpointConfig`, `NewWebhookExecutor`, and `ResolveCABundle` have
  callers in other packages. Examine the callers before you change a signature.
- Integration tests for authorization and TLS are in
  `test/integration/composite_with_auth` and `decorator_with_auth`.
- User documentation for this package is `docs/src/api/hook.md`. Update it
  when behavior changes.

## TRAPS
- No hot reload. The CA bundle and the authorization material are resolved one
  time, when the controller object is created or updated. A changed Secret or
  ConfigMap has no effect until the controller object changes. See the
  `TODO(hot-reload)` comment in `pkg/apis/metacontroller/v1alpha1/types.go`.
  Do not write code that assumes live rotation.
- v2 hooks do not report Kubernetes events from the hook response.
