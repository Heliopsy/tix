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
| Second factor | `/auth/totp`, `/auth/totp/confirm`, `/auth/totp/verify`, `/auth/totp/recovery-codes`, `/users/{id}/totp`; see [Second factor](#second-factor) |
| Users and tokens | `/users`, `/users/{id}`, `/tokens`, `/tokens/{id}` |
| SSH keys | `/ssh-keys`, `/ssh-keys/{id}`; see [deployment.md](deployment.md#enrolling-a-key) |
| Directory | `/actors`, `/actors/{id}` |
| Tenancy | `/tenants`, `/tenants/{ref}`, `/members`, `/members/{actorID}`, `/domains`, `/domains/{hostname}` |
| Projects | `/projects`, `/projects/{ref}`, `/projects/{ref}/icon`, `/projects/{ref}/deletion`, `/projects/{ref}/fields`, `/projects/{ref}/fields/{key}` |
| Workflows | `/workflows`, `/workflows/{key}` |
| Tasks | `/tasks`, `/tasks/{ref}`, `/tasks/{ref}/transition`, `/tasks/{ref}/route`, `/tasks/{ref}/restore`, `/tasks/{ref}/tree` |
| Task relations | `/tasks/{ref}/deps`, `/tasks/{ref}/deps/{dep}`, `/tasks/{ref}/tags`, `/tasks/{ref}/tags/{name}`, `/tasks/{ref}/star` |
| Comments and artifacts | `/tasks/{ref}/comments`, `/tasks/{ref}/artifacts`, `/comments/{id}`, `/tags` |
| Claims | `/tasks/{ref}/claim`, `/tasks/{ref}/claim/renew`, `/tasks/{ref}/claim/release`, `/tasks/{ref}/claim/reclaim`, `/claims/next`, `/claims/sweep` |
| Webhooks | `/webhooks`, `/webhooks/{id}`, `/webhooks/deliveries`, `/webhooks/deliveries/{id}/redeliver` |
| History | `/audit`, `/tasks/{ref}/audit`, `/retention`, `/prune` |
| Reports | `GET /stats`, see [statistics.md](statistics.md); `GET /status`, the report `tix status` prints, see [scripting.md](scripting.md) |
| Live connections | `GET /connections`, `DELETE /connections/{id}`; see [deployment.md](deployment.md#live-connections) |
| Transfer | `/export`, `/import`, `/bundles/export`, `/bundles/import` |
| Sync | `/sync/sources`, `/sync/sources/{id}`, `/sync/run` |
| Views | `/views/{view}/groups`, `/views/{view}/groups/{group}`, `/views/{view}/order` |
| Events | `/events` (WebSocket) |

Every route above has a command: `tix doctor`, `tix login`, `tix user totp`, `tix user key`, `tix actor`,
`tix stats`, `tix status` and `tix connection` among the less obvious ones. [commands.md](commands.md)
lists them.

### Project appearance

A project carries an optional `color` and `icon`. Both are accepted on `POST /projects` and `PATCH
/projects/{ref}` and returned on every project representation, omitted when unset:

```console
$ curl -s -X POST -H "Authorization: Bearer $TIX_TOKEN" -H 'Content-Type: application/json' \
    -d '{"key":"api1","name":"Api One","color":"amber","icon":"A"}' \
    http://127.0.0.1:8080/api/v1/projects
{"id":"01M30P5...","tenant_id":"01M30NW...","key":"api1","name":"Api One","workflow_id":"01M30NW...",
 "color":"amber","icon":"A","created_at":"...","updated_at":"..."}
```

`color` is one of `slate`, `red`, `amber`, `green`, `teal`, `blue`, `violet` or `pink`, or a custom colour as
`#rrggbb`, which is stored and returned in lowercase. `icon` is one emoji or a monogram of at most two
characters. Anything else is `400`. The CLI spellings are `--color` and `--icon` on
`tix project create` and `tix project edit`; an empty string on either clears it.

The icon can be one of two other kinds instead. `icon_name` picks a built-in symbol (`folder`, `briefcase`,
`home`, `server`, `database`, `cloud`, `code`, `terminal`, `bug`, `rocket`, `shield`, `lock`, `globe`,
`chart`, `book`, `users`, `package`, `wrench`, `flask`, `bolt`). `icon_file` uploads an image, as
`{"content_type": "...", "data": "<base64>"}`: a PNG, JPEG, WebP or SVG of at most 64 KiB, typed by its bytes
rather than by `content_type`. An SVG carrying script, event handlers, `foreignObject` or an external
reference is `400`. A project then carries `icon_image`, the image's digest.

A project has at most one kind at a time. Sending one replaces whichever it had, sending two non-empty ones
is `400`, and sending any of them empty clears the icon.

```console
$ curl -s -X PATCH -H "Authorization: Bearer $TIX_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"icon_file\":{\"data\":\"$(base64 -w0 logo.png)\"}}" http://127.0.0.1:8080/api/v1/projects/api1
$ curl -s -H "Authorization: Bearer $TIX_TOKEN" -o logo.png http://127.0.0.1:8080/api/v1/projects/api1/icon
```

`GET /projects/{ref}/icon` answers with the image itself: its content type, `X-Content-Type-Options:
nosniff`, a `Content-Security-Policy` that forbids script and sandboxes the document, and an `ETag` that
`If-None-Match` revalidates to `304`. A project whose icon is not an uploaded image answers `404`. Snapshots
and bundles carry `icon` and `icon_name`, never the image bytes.

### Deleting a project

`DELETE /projects/{ref}` is permanent. A project holding tasks, soft deleted ones included, needs one of two
query parameters, and without either the answer is `422` with code `precondition` and a `tasks` detail:

- `move_tasks_to=KEY` moves every task to that project first. Each gets the target's next number, keeps its
  identifier and so its comments, attachments, tags, dependencies, stars and history, and has its status
  mapped onto the target's workflow: the same state, else the first state of the same category, else the
  initial state. It needs `task:write` on the target, and an archived target is `422`.
- `delete_tasks=true` deletes them permanently with the project.

Both at once is `400`. A deletion answers `200` with what it did. `GET /projects/{ref}/deletion` takes the
same parameters and returns the same shape without writing, which is how to see the mapping first:

```console
$ curl -s -H "Authorization: Bearer $TIX_TOKEN" \
    'http://127.0.0.1:8080/api/v1/projects/old/deletion?move_tasks_to=ops'
{"project":"old","tasks":3,"deleted":0,"move_to":"ops",
 "statuses":[{"from":"todo","to":"todo","tasks":2},{"from":"review","to":"doing","tasks":1}],"dry_run":true}
```

Every moved task records a `task.move` audit entry and a `task.updated` event naming its old and new
reference, and every deleted one a `task.delete` entry and a `task.deleted` event with `hard: true`.

### Changing a project's key

`PATCH /projects/{ref}` accepts `key`, which renames the project and every task reference with it: `infra-8`
becomes `ops-8`. The old key is kept as an alias of the project for as long as the project exists, so a path,
a `project_ref`, a filter or a task reference that still names it resolves to the renamed project, and the
response carries the new key and the new `ref`. A key that is the current key or an old key of another
project is `409`; a key that breaks the key rules is `400`. The `project.updated` event of a rename carries
`previous_key` beside `key`.

```console
$ curl -s -X PATCH -H "Authorization: Bearer $TIX_TOKEN" -H 'Content-Type: application/json' \
    -d '{"key":"ops"}' http://127.0.0.1:8080/api/v1/projects/infra
{"id":"01M30P5...","key":"ops","name":"Infrastructure", ...}
$ curl -s -H "Authorization: Bearer $TIX_TOKEN" http://127.0.0.1:8080/api/v1/tasks/infra-8
{"id":"01M6...","ref":"ops-8", ...}
```

### Recurring tasks

A task carries an optional `recurrence`, a repeat rule in the syntax [features.md](features.md)
describes, and `recurrence_next_id`, the identifier of the task it spawned when it was finished. Both are
omitted when empty. `POST /tasks` accepts `recurrence`; `PATCH /tasks/{ref}` accepts it too, where `""`
removes the rule and an absent key leaves it alone. The rule comes back in its canonical spelling, so
`"weekly on thu,mon"` is returned as `"every week on mon,thu"`. A rule that does not parse is `400`.
`recurrence_next_id` is never accepted as input.

```console
$ curl -s -X POST -H "Authorization: Bearer $TIX_TOKEN" -H 'Content-Type: application/json' \
    -d '{"project_ref":"home","title":"Pay rent","due_at":"2026-12-01T00:00:00Z","recurrence":"monthly"}' \
    http://127.0.0.1:8080/api/v1/tasks
{"id":"01M6...","ref":"home-4","title":"Pay rent","status":"todo","due_at":"2026-12-01T00:00:00Z",
 "recurrence":"every month", ...}
```

Moving a recurring task into a done state, through `/transition`, `/route` or a claim release with a final
status, creates the next occurrence in the same transaction. The stream carries the move's
`task.transitioned` event with `next`, the new task's reference, followed by an ordinary `task.created`
event whose payload names `recurs_from`. Moving into a cancelled state creates nothing.

### Second factor

When the tenant requires a code on the surface a sign-in names, `POST /auth/login` answers the password
step with a challenge instead of a session. `surface` is `web`, `cli` or `ssh`; leaving it out means `cli`,
so omitting it never walks past a requirement:

```console
$ curl -s -X POST -H 'Content-Type: application/json' \
    -d '{"email":"ana@example.com","password":"...","surface":"cli"}' \
    http://127.0.0.1:8080/api/v1/auth/login
{"challenge":{"kind":"code","ticket":"..."}}
```

The second step sends the ticket back with the code, and no password. A recovery code is accepted in place
of a code. The answer is the session, set as a cookie too:

```console
$ curl -s -X POST -H 'Content-Type: application/json' \
    -d '{"ticket":"...","code":"123456"}' http://127.0.0.1:8080/api/v1/auth/login
{"session":{"token":"...","actor_id":"...","tenant_id":"...","expires_at":"..."}}
```

A challenge of kind `enrol` carries an `enrolment` (`secret`, `uri`, `issuer`, `account`) for a user the
requirement applies to who has no factor yet; the second step confirms a code from it, and the answer then
also carries `recovery_codes`, once. There is no way to send a code with the password in one call.

| Route | Effect |
| --- | --- |
| `GET /auth/totp` | the caller's enrolment and what the tenant requires of them |
| `POST /auth/totp` | start an enrolment; `201` with `secret` and `uri`, inert until confirmed |
| `POST /auth/totp/confirm` | `{"code":"..."}` confirms it and returns `{"codes":[...]}`, the recovery codes, once |
| `POST /auth/totp/recovery-codes` | replace the caller's recovery codes and return the new set once |
| `DELETE /auth/totp` | remove the caller's own enrolment; `204` |
| `POST /auth/totp/verify` | `{"code":"..."}` proves the caller's existing factor and mints nothing; `204`. It is for a surface whose first factor is not a password, such as an SSH key |
| `DELETE /users/{id}/totp` | clear another user's enrolment; needs `totp:admin`; `204` |

API tokens never meet a second factor. [second-factor.md](second-factor.md) covers the policy, the key and
recovery.

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
`due_before` and `due_after` narrow a task listing to a deadline window. Both take `YYYY-MM-DD`, `YYYY-MM-DD
HH:MM` or RFC 3339, both ends are inclusive, and both drop a task carrying no deadline. There is no
`overdue` parameter: a caller holds a clock, so "overdue" is `due_before` set to now. The filter expression's
`due:overdue` is that bound, worked out where the expression is parsed.

Sort fields for tasks are `urgency` (the default), `created_at`, `updated_at`, `priority`, `due_at`, `seq` and
`title`. `urgency` is a compound ordering: priority first, most urgent priority leading, then due date within a
shared priority, soonest deadline leading. A task with no due date sorts after every dated task at the same
priority, as if its deadline were the most distant. An unparseable cursor or limit is `400` with the standard
envelope.

The CLI equivalents are `--cursor`, `--limit`, `--sort`, `--desc`, and `--all` to follow cursors until exhausted.

### Stars

`PUT /tasks/{ref}/star` stars a task for the caller and `DELETE /tasks/{ref}/star` clears it. Both answer
`204` whether or not anything changed, and only a call that changed something writes an audit entry and
emits `task.starred` or `task.unstarred`. A star belongs to the actor that set it: a task read carries
`"starred": true` when the caller starred it and omits the field otherwise, and `starred=true|false`
narrows a task listing by the caller's own stars.

A task listing puts the caller's starred tasks first, then the rest, each group in the requested sort and
direction. The grouping is part of the keyset, so the `next_cursor` that ends inside the starred group
continues into the unstarred one without skipping or repeating a row. A task starred or unstarred while
somebody pages moves between the groups, like a task whose priority changed under a priority sort: the cursor
stays valid and the moved task may be seen twice or not at all. A cursor issued before stars existed resumes
among the unstarred tasks.

### Arranging views

Each actor keeps their own arrangement of the groups a view draws: the columns of a board and the lanes of
the task screen. A view is `board` (the task screen grouped by status, groups are state keys), `lanes` (the
task screen grouped by project, groups are project keys) or `project:KEY` (one project's board, groups are
the states of its workflow). Anything else is `invalid`, and so is a group the view cannot hold.

| Route | Does |
| --- | --- |
| `GET /views/{view}/groups` | The caller's preferences on the view, one per group they touched |
| `PATCH /views/{view}/groups/{group}` | Sets any of `hidden`, `collapsed` and `starred`; answers the preference |
| `PUT /views/{view}/order` | Takes `{"groups": [...]}` and puts them first in that order, positions from 1 |
| `DELETE /views/{view}/groups` | Forgets the whole view, or only `?group=KEY`; answers `204 No Content` |

```json
{"view": "board", "group": "done", "hidden": true, "collapsed": false, "starred": false}
```

A preference changes nothing anybody else sees, and needs only `task:read`; a token pinned to one project
cannot arrange, because a view spans projects. A change writes an audit entry and emits `view.arranged`
carrying `view` and `groups`; one that changes nothing writes neither. A project renamed to a new key keeps
its arrangement.

### NDJSON

Send `Accept: application/x-ndjson` and the list streams one object per line with no envelope:

```sh
curl -H "Authorization: Bearer $TIX_TOKEN" -H 'Accept: application/x-ndjson' \
  'http://127.0.0.1:8080/api/v1/tasks?limit=500'
```

### Filtering tasks

Query parameters narrow `/tasks`: project, status, tag, assignee, claimed, blocked and starred state, free
text, and `recurring=true` or `recurring=false` for tasks that do or do not carry a repeat rule (`is:recurring` in
the filter expression).

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

`event:subscribe` covers ordinary work: tasks, comments, artifacts, dependencies, labels, projects,
workflows and fields. It does not, on its own, deliver events about users, memberships, sessions, API
tokens, SSH keys, tenants, domains, connections, webhook endpoints or sync sources. Those carry details
you would be refused over REST with the same credential, such as a user's email address, the scope list of
a token, or a webhook's destination, so each one also requires the scope that reads it: `user:admin`,
`token:admin`, `tenant:admin`, `webhook:admin` or `sync:admin`. A credential holding `*`, or a role that
grants these, sees everything as before.

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
`task.reclaimed`, `task.deleted`, `task.starred`, `task.unstarred`, `comment.added`, `artifact.added`,
`dependency.added`, `tag.added`, `project.created`, `project.updated`, `workflow.updated`, `field.updated`,
`webhook.delivered`, `import.completed`, `view.arranged`. A type outside it is an error rather than a filter
that silently matches nothing.

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

A signing secret is stored write-only: no read of an endpoint ever returns it. Register one without a secret
and tix generates one, returned in the response to that registration alone — in the browser, `/admin/webhooks`
renders it once, in the same one-time secret region the token screen uses for a freshly minted token, and
never again. Supply your own and nothing is shown back to you, since you already hold it. Saving an existing
endpoint with the secret field blank keeps the secret it already has rather than rotating it.

## Related

- [agents.md](agents.md) for claiming and leases
- [deployment.md](deployment.md) for TLS, proxies and the bind guard
- [scripting.md](scripting.md) for the CLI equivalents
