# HTTP API and event stream

`tix serve` exposes the same service the CLI calls, over HTTP at `/api/v1`, plus a WebSocket event stream and the
web UI.

```sh
tix serve --listen 127.0.0.1:8080
```

## Authentication

Bearer token:

```sh
curl -H "Authorization: Bearer $TIX_TOKEN" http://127.0.0.1:8080/api/v1/whoami
```

```json
{"id":"01M30B3Y1SNEPGNQJ489NMR8TT","tenant_id":"01M30B3Y1QN9WXZH9YK9VJYMFM","kind":"agent",
 "handle":"ci","scopes":["task:read","task:claim","event:subscribe"],"token_id":"01M30B6JZ96FN5R8765P6FG35Z"}
```

Browsers use a session cookie obtained from `POST /api/v1/auth/login`. The cookie is marked `Secure` when the
server is reached over TLS.

A missing or invalid credential is `401`. A valid credential without the required scope is `403`.

## Health

| Route | Purpose |
| --- | --- |
| `GET /healthz` | liveness; `{"status":"ok","version":"dev"}` |
| `GET /readyz` | readiness; checks the database and the applied migrations |

Neither requires authentication, which is what a load balancer needs.

```json
{"ready":true,"checks":[{"name":"database","ok":true},{"name":"migrations","ok":true}]}
```

## Routes

Everything below is under `/api/v1`.

| Area | Routes |
| --- | --- |
| Identity | `GET /whoami`, `POST /auth/login`, `POST /auth/logout` |
| Users and tokens | `/users`, `/users/{id}`, `/tokens`, `/tokens/{id}` |
| Tenancy | `/tenants`, `/tenants/{ref}`, `/members`, `/members/{actorID}`, `/domains`, `/domains/{hostname}` |
| Projects | `/projects`, `/projects/{ref}`, `/projects/{ref}/fields`, `/projects/{ref}/fields/{key}` |
| Workflows | `/workflows`, `/workflows/{key}` |
| Tasks | `/tasks`, `/tasks/{ref}`, `/tasks/{ref}/transition`, `/tasks/{ref}/restore`, `/tasks/{ref}/tree` |
| Task relations | `/tasks/{ref}/deps`, `/tasks/{ref}/deps/{dep}`, `/tasks/{ref}/tags`, `/tasks/{ref}/tags/{name}` |
| Comments and artifacts | `/tasks/{ref}/comments`, `/tasks/{ref}/artifacts`, `/comments/{id}`, `/tags` |
| Claims | `/tasks/{ref}/claim`, `/tasks/{ref}/claim/renew`, `/tasks/{ref}/claim/release`, `/claims/next`, `/claims/sweep` |
| Webhooks | `/webhooks`, `/webhooks/{id}`, `/webhooks/deliveries`, `/webhooks/deliveries/{id}/redeliver` |
| History | `/audit`, `/tasks/{ref}/audit`, `/retention`, `/prune` |
| Transfer | `/export`, `/import`, `/bundles/export`, `/bundles/import` |
| Sync | `/sync/sources`, `/sync/sources/{id}`, `/sync/run` |
| Events | `/events` (WebSocket) |

## Listing and pagination

Every list is keyset paginated. There is no `OFFSET` anywhere, so a page is stable while rows are being inserted.

```sh
curl -H "Authorization: Bearer $TIX_TOKEN" \
  'http://127.0.0.1:8080/api/v1/tasks?limit=50&sort=created_at&direction=desc'
```

```json
{"items":[ ... ],"next_cursor":"..."}
```

Pass `next_cursor` back as `cursor` for the following page. An empty or absent `next_cursor` means the last page.
Sort fields are `created_at`, `updated_at`, `priority`, `due_at`, `seq` and `title`. An unparseable cursor or
limit is `400` with the standard envelope.

The CLI equivalents are `--cursor`, `--limit`, `--sort`, `--desc`, and `--all` to follow cursors until exhausted.

### NDJSON

Send `Accept: application/x-ndjson` and the list streams one object per line with no envelope:

```sh
curl -H "Authorization: Bearer $TIX_TOKEN" -H 'Accept: application/x-ndjson' \
  'http://127.0.0.1:8080/api/v1/tasks?limit=500'
```

### Filtering tasks

Query parameters narrow `/tasks`: project, status, tag, assignee, claimed and blocked state, and free text.

Custom field values are filtered with a `field.` prefixed parameter, one per field:

```sh
curl -H "Authorization: Bearer $TIX_TOKEN" 'http://127.0.0.1:8080/api/v1/tasks?field.severity=high'
```

A custom field predicate is always permitted; whether it is cheap depends on the field being marked indexed. See
[workflows.md](workflows.md).

## Errors

One envelope everywhere:

```json
{"error":{"code":"not_found","message":"task \"default-999\""}}
```

| Code | Status |
| --- | --- |
| `invalid` | 400 |
| `unauthenticated` | 401 |
| `forbidden` | 403 |
| `not_found` | 404 |
| `no_task_available` | 404 |
| `conflict`, `lease_expired` | 409 |
| `precondition_failed` | 422 |
| `internal` | 500 |

An empty claim queue is a `no_task_available` error rather than an empty success, matching the CLI's exit 3.

Every response carries `X-Request-Id`, which also appears in the server log line for that request.

Request bodies must be `application/json` (`415` otherwise) and are capped at 1 MiB by default (`413` beyond it,
adjustable with `--max-body-bytes`). Each request has a 30 second timeout by default (`--request-timeout`).

## The event stream

`GET /api/v1/events` upgrades to a WebSocket. The caller must hold `event:subscribe`; anything else is rejected
before the upgrade.

Client messages: `subscribe`, `unsubscribe`, `ping`.
Server messages: `subscribed`, `event`, `pong`, `error`.

### Subscribing

```json
{"type":"subscribe","id":"s1","filter":{"types":["task.*"],"project_ids":["01M3..."]}}
```

The server acknowledges:

```json
{"type":"subscribed","id":"s1"}
```

`id` is chosen by the client and identifies the subscription on that connection. Several subscriptions can share
one connection. `{"type":"unsubscribe","id":"s1"}` ends one.

Event types are matched as glob patterns, so `task.*` selects every task event. The vocabulary is closed:
`task.created`, `task.updated`, `task.transitioned`, `task.claimed`, `task.released`, `task.lease_expired`,
`task.deleted`, `comment.added`, `artifact.added`, `dependency.added`, `tag.added`, `project.created`,
`project.updated`, `workflow.updated`, `field.updated`, `webhook.delivered`, `import.completed`. A type outside it
is an error rather than a filter that silently matches nothing.

A project-scoped token has its subscription narrowed to that project automatically, and naming a different
project is `forbidden`.

`{"type":"ping","id":"p1"}` gets `{"type":"pong","id":"p1"}`. The server also sends WebSocket-level pings every
30 seconds and closes a connection that does not answer within 10.

### Events

```json
{"type":"event","id":"s2","event":{
  "seq":5,
  "id":"01M30B3Y2HVAB2RZRTEWF0HASV",
  "tenant_id":"01M30B3Y1QN9WXZH9YK9VJYMFM",
  "type":"task.claimed",
  "project_id":"01M30B3Y1RBSS6JAHPY850QMB6",
  "subject_type":"task",
  "subject_id":"01M30B3Y1TVT299QJPV8W8MM4X",
  "actor_id":"01M30B3Y1SNEPGNQJ489NMR8TT",
  "payload":{"claimed_by":"01M30B3Y1SNEPGNQJ489NMR8TT","lease_expires_at":"2026-09-20T21:51:24.047605506Z"},
  "occurred_at":"2026-09-20T21:21:24.047605506Z"}}
```

`seq` is a per-tenant monotonic sequence number. It is the cursor.

### Resuming with since_seq

Every mutation writes its event in the same transaction as the data, so the log has no gaps. A client that
records the highest `seq` it processed can reconnect and pick up exactly there:

```json
{"type":"subscribe","id":"s2","since_seq":1}
```

```json
{"type":"subscribed","id":"s2","since_seq":1}
{"type":"event","id":"s2","event":{"seq":2, ...}}
{"type":"event","id":"s2","event":{"seq":3, ...}}
```

`since_seq` is exclusive: delivery starts at `since_seq + 1`. The server replays the durable log from that point
and then switches to live delivery with no window where an event could fall between the two. `since_seq` may be
omitted or `0` for live-only delivery.

Retention eventually drops old events (`retention.events`, 720h by default). A cursor older than the oldest
retained event is refused rather than silently skipped:

```json
{"type":"error","id":"s2","error":{"code":"invalid",
  "message":"cursor 12 is no longer available; the oldest retained event is 4000"}}
```

A client that sees this has provably missed events and should resynchronize by listing rather than pretending to
resume.

### A minimal subscriber

```python
import asyncio, json, websockets

async def main(token, cursor=0):
    async with websockets.connect(
        "ws://127.0.0.1:8080/api/v1/events",
        extra_headers={"Authorization": "Bearer " + token},
    ) as ws:
        await ws.send(json.dumps({"type": "subscribe", "id": "s1",
                                  "since_seq": cursor, "filter": {"types": ["task.created"]}}))
        async for raw in ws:
            msg = json.loads(raw)
            if msg["type"] == "event":
                cursor = msg["event"]["seq"]
                handle(msg["event"])

asyncio.run(main(os.environ["TIX_TOKEN"]))
```

Persist `cursor` alongside whatever the handler wrote, and a restart loses nothing.

## Webhooks

For a receiver that cannot hold a connection open, register an endpoint instead:

```sh
tix webhook put https://ci.example.com/hooks/tix --event task.created --event task.transitioned --secret "$S"
tix webhook ls
tix webhook deliveries --endpoint 01J000... --status failed
tix webhook redeliver 01J000...
```

Deliveries carry `X-Tix-Event`, `X-Tix-Delivery`, `X-Tix-Timestamp` and `X-Tix-Signature`, the last being
`sha256=` followed by the hex HMAC of the timestamp and body under the endpoint's secret. Verify it before
trusting a delivery.

## Related

- [agents.md](agents.md) for claiming and leases
- [deployment.md](deployment.md) for TLS, proxies and the bind guard
- [scripting.md](scripting.md) for the CLI equivalents
