# Webhook secret over HTTP, and a test ping

## Why

`PUT /api/v1/webhooks` generates a signing secret when the caller supplies none, and the operation's
description says the response returns it once. It never does: `core.WebhookEndpoint.Secret` is tagged
`json:"-"`, so the HTTP answer drops it and so does `internal/client`. A remote `tix webhook put`, the
API and every MCP client register an endpoint whose secret nobody can ever read, which leaves it unable to
verify a single delivery. Only the in-process command line, browser and terminal showed it.

Separately, there is no way to check that an endpoint is reachable and verifying signatures short of
making a real change and waiting for its event. Every webhook sender people are used to has a ping.

## What Changes

- `PutWebhook` answers `core.IssuedWebhook`: the endpoint plus `secret`, present only when tix generated
  it for this call. Reads, listings, audit entries and event payloads still never carry it.
- `POST /api/v1/webhooks/{id}/ping` (`webhook.ping`, service method `PingWebhook`) queues one synthetic
  `webhook.ping` event to that endpoint alone and answers the delivery, attempted already when the process
  drains inline. It appears in the delivery log and can be redelivered like any other.
- `tix webhook ping ID`, a Ping button per endpoint on the browser's webhook screen, and `p` on an endpoint
  in the terminal's integrations view.

## Capabilities

### Modified Capabilities

- `webhooks`: the generated secret reaches HTTP callers once; a test ping to one endpoint.

## Impact

`internal/core` (`IssuedWebhook`, `PingWebhook`), `internal/service`, `internal/httpapi`, `internal/client`,
`internal/capability`, `cmd`, `internal/web`, `internal/tui`, `api/openapi.yaml`, docs and the tix skill.
`PutWebhook`'s Go return type changes; the JSON answer only gains a field.
