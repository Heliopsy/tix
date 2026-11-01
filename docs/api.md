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
| Tasks | `/tasks`, `/tasks/{ref}`, `/tasks/{ref}/transition`, `/tasks/{ref}/route`, `/tasks/{ref}/restore`, `/tasks/{ref}/tree`, `/tasks/{ref}/copy`, `/tasks/bulk`, `GET /trash`; see [Trash](#trash) |
| Task relations | `/tasks/{ref}/deps`, `/tasks/{ref}/deps/{dep}`, `/tasks/{ref}/tags`, `/tasks/{ref}/tags/{name}`, `/tasks/{ref}/star`, `/tasks/{ref}/checklist`, `/tasks/{ref}/snooze` |
| Comments and artifacts | `/tasks/{ref}/comments`, `/tasks/{ref}/artifacts`, `/comments/{id}`, `/tags` |
| Claims | `/tasks/{ref}/claim`, `/tasks/{ref}/claim/renew`, `/tasks/{ref}/claim/release`, `/tasks/{ref}/claim/reclaim`, `/claims/next`, `/claims/sweep` |
| Snoozes | `/snoozes/wake` |
| Webhooks | `/webhooks`, `/webhooks/{id}`, `/webhooks/deliveries`, `/webhooks/deliveries/{id}/redeliver` |
| History | `/audit`, `/tasks/{ref}/audit`, `/retention`, `/prune` |
| Reports | `GET /stats`, see [statistics.md](statistics.md); `GET /status`, the report `tix status` prints, see [scripting.md](scripting.md) |
| Live connections | `GET /connections`, `DELETE /connections/{id}`; see [deployment.md](deployment.md#live-connections) |
| Transfer | `/export`, `/import`, `/bundles/export`, `/bundles/import` |
| Sync | `/sync/sources`, `/sync/sources/{id}`, `/sync/run` |
| Views | `/views/{view}/groups`, `/views/{view}/groups/{group}`, `/views/{view}/order` |
| Saved filters | `/filters`, `/filters/{filter}`, `/filters/order` |
| Calendar feed | `/calendar`, `/calendar/rotate`, `/calendar/feeds/{file}` (no credential); see [Calendar feed](#calendar-feed) |
| Inbox | `/inbox`, `/inbox/count`, `/inbox/{id}/read`, `/inbox/read-all`, `/inbox/prefs`; see [Inbox](#inbox) |
| Notification channels | `/channels`, `/channels/{id}`, `/channels/{id}/test`, `/channels/{id}/deliveries`, `/channel-deliveries/{id}/redeliver`; see [Notification channels](#notification-channels) |
| Templates | `/templates`, `/templates/{ref}`, `/templates/{ref}/use`; see [Task templates](#task-templates) |
| Events | `/events` (WebSocket) |

Every route above has a command: `tix doctor`, `tix login`, `tix user totp`, `tix user key`, `tix actor`,
`tix stats`, `tix status` and `tix connection` among the less obvious ones. [commands.md](commands.md)
lists them.

### Calendar feed

Each person may hold one calendar feed: an iCalendar address a calendar app subscribes to. The
management routes act on the caller's own feed and need `task:read`. [calendar.md](calendar.md) has
the format, the state mapping and the reasons behind them.

| Route | Does |
| --- | --- |
| `GET /calendar` | the caller's feed: `id`, `filter`, `created_at`; never the secret. `404` without one |
| `POST /calendar` | creates the feed from an optional `{"filter": "..."}` and answers `201` with `token`, `path` and the absolute `url`, once. `409` when one is live |
| `PATCH /calendar` | replaces the filter with `{"filter": "..."}`; an empty string restores the default. The address stays |
| `POST /calendar/rotate` | a new `token`, `path` and `url` with the same filter; the old address answers `404` at once |
| `DELETE /calendar` | stops the feed, `204` |
| `GET /calendar/feeds/{token}.ics` | the feed itself, `text/calendar; charset=utf-8`, with no credential |

A filter is validated when it is stored, so `project:nosuch` is `404` and a term that does not parse is
`400`. The feed answer carries a strong `ETag` and `Cache-Control: private, max-age=300`, and a
request whose `If-None-Match` matches gets `304` with no body. A malformed, unknown, rotated or revoked
secret, an owner who is no longer a member and a disabled account all answer the same `404`.

```console
$ curl -s -X POST -H "Authorization: Bearer $TIX_TOKEN" http://127.0.0.1:8080/api/v1/calendar
{"id":"01M3...","tenant_id":"01M2...","actor_id":"01M2...","filter":"","created_at":"...",
 "token":"tix_cal_...","path":"/api/v1/calendar/feeds/tix_cal_....ics",
 "url":"http://127.0.0.1:8080/api/v1/calendar/feeds/tix_cal_....ics"}
```

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

### Quick add

`POST /tasks` with `"parse": true` reads `title` as a quick-add line, the syntax
[features.md](features.md) describes, and takes the attributes it names out of the title before the task
is created. Relative dates (`due:fri`, `due:+3d`) count in UTC, since the API knows nobody's zone. A field
in the body that the line contradicts is `400`; an unknown project, handle or parent is `404`, as it is
for the explicit field. Without `parse` the title is stored as given.

```console
$ curl -s -X POST -H "Authorization: Bearer $TIX_TOKEN" -H 'Content-Type: application/json' \
    -d '{"title":"Fix login !high #auth due:2026-11-20 +infra","parse":true}' \
    http://127.0.0.1:8080/api/v1/tasks
{"id":"01M6...","ref":"infra-9","title":"Fix login","priority":2,"tags":["auth"], ...}
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

### Checklists

A task's `body` is Markdown, and its GitHub task-list items (`- [ ]` and `- [x]`) are its checklist. A
task read carries `"checklist": {"done": 1, "total": 3}` when the description has items and omits the
field when it has none. The count is computed from the body on every read, so it never appears in an
audit snapshot or an export.

`POST /tasks/{ref}/checklist` with `{"item": 2, "done": true, "version": 7}` ticks item 2, counted from 1
in reading order with nested items included, and `"done": false` clears it. Only that item's marker in
the body changes. It needs `task:write`, answers with the task, and records an ordinary `task.update`
whose payload names the item. `version` is optional and, when given, a stale one is refused with `409`
like any other update; an item number the description does not have is a `400` that names the total. An
item already in the asked-for state answers `200` and writes nothing.

`checklist=true|false` narrows a listing to tasks whose body does or does not carry a checklist marker,
and `checklist_open=true|false` to those that do or do not carry an unticked one. See
[filtering.md](filtering.md#checklists).

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

### Snoozing

`PUT /tasks/{ref}/snooze` with `{"until": "2026-12-01T09:00:00Z"}` hides a task from every default
listing, for everybody, until that instant, and answers the task with `snoozed_until` and
`snoozed_by_actor_id`. The time must be in the future (`invalid` otherwise) and is kept to the second.
Snoozing a snoozed task moves its wake time. `DELETE /tasks/{ref}/snooze` brings it back and answers the
task; on a task that is not snoozed it changes nothing and writes nothing. Each change writes an audit
entry naming the caller and emits `task.snoozed` or `task.unsnoozed`.

A task is snoozed while `snoozed_until` is after the present, judged on every read, so it comes back on
time with nothing running. `snoozed=true` lists only snoozed tasks, `snoozed=false` only the others (the
default made explicit), and `snoozed=any` both. Reading one task by reference always works.

`POST /snoozes/wake`, with an optional `{"limit": N}`, clears snoozes whose time has passed and answers
`{"woken": N}`, emitting `task.woke` for each. The server's sweep loop does this every sweep interval.

A snoozed task is not claimable: `/claims/next` skips it, and `POST /tasks/{ref}/claim` answers
`precondition` with the wake time in `details.snoozed_until` until it wakes or is unsnoozed.

### Arranging views

Each actor keeps their own arrangement of the groups a view draws: the columns of a board and the lanes of
the task screen. A view is `board` (the task screen grouped by status, groups are state keys), `lanes` (the
task screen grouped by project, groups are project keys), `mine` (My work, grouped by deadline: `overdue`,
`today`, `week`, `later` and `none`) or `project:KEY` (one project's board, groups are the states of its
workflow). Anything else is `invalid`, and so is a group the view cannot hold.

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

### Saved filters

A saved filter is a named filter expression with the view (`list`, `board` or `lanes`, or empty for the
reader's own) and the sort the task screen opens it with. It belongs to the actor who saved it; one saved
with `"shared": true` is listed for everybody in the tenant, and only its owner or an actor with
`tenant:admin` may change or delete it. Another reader's private filter is `not_found`, never `forbidden`.
A filter is addressed by its id or by its name: the caller's own name first, then a shared one.

| Route | Does |
| --- | --- |
| `GET /filters` | The caller's own filters in their order, then the shared ones by name |
| `POST /filters` | Saves `{"name", "query", "view", "sort", "shared"}`; answers `201` with the filter |
| `GET /filters/{filter}` | One filter, by id or name |
| `PATCH /filters/{filter}` | Changes any of `name`, `query`, `view`, `sort` and `shared` |
| `PUT /filters/order` | Takes `{"filters": [...]}`, the caller's own, and puts them first in that order |
| `DELETE /filters/{filter}` | Removes it; answers `204 No Content` |

```json
{"id": "01J...", "name": "My overdue", "query": "assignee:me due:overdue", "view": "list", "sort": "due_at",
 "shared": false, "owner_id": "01H...", "position": 1, "mine": true, "editable": true}
```

`mine` and `editable` are answered for the caller. The expression is checked by the filter parser when it is
saved, so an unreadable one is `invalid`; the names it holds (projects, states, handles) are resolved when it
is used, like any filter. Names are unique per owner without regard to case, and a taken one is `conflict`.
Each change writes an audit entry and emits `filter.saved`, `filter.updated` or `filter.deleted` carrying the
`name` and `shared`; one that changes nothing writes neither. Saved filters need only `task:read`, and a token
pinned to one project cannot keep them, because a filter spans projects.

In any filter, `me` in `assignee:`, `creator:` or `claimed-by:` (and the `assignee` query parameter) names the
caller, so `assignee:me` lists what is assigned to whoever asks.

### Inbox

Each actor has an inbox. An item is written in the same transaction as the change that caused it when
somebody mentions the actor as `@handle` in a comment or a task description (`mention`), assigns them a
task (`assigned`), comments on a task they are assigned to or starred (`comment`), or moves a task
assigned to them (`status`). Nobody receives an item for their own change, one change gives one
recipient at most one item per task (the strongest reason in that order wins), and a kind the recipient
muted creates nothing. Editing a comment or a description mentions only handles the earlier text did not.

| Route | Does |
| --- | --- |
| `GET /inbox` | The caller's items, newest first; `?unread=true` for unread only, `limit` and `cursor` as every listing |
| `GET /inbox/count` | `{"unread": N}` |
| `PUT /inbox/{id}/read` | Marks one item read and answers it |
| `DELETE /inbox/{id}/read` | Marks one item unread and answers it |
| `POST /inbox/read-all` | Marks every item read; answers `{"marked": N}` |
| `GET /inbox/prefs` | `{"muted": [...]}`, the kinds that create no item for the caller |
| `PUT /inbox/prefs` | Replaces the muted kinds; an unknown kind is `invalid` |

```json
{"id": "01J...", "kind": "status", "task_id": "01J...", "task_ref": "infra-42", "task_title": "Cut over",
 "actor_id": "01J...", "actor_handle": "ada", "from_status": "todo", "to_status": "doing",
 "created_at": "2026-11-01T09:30:00Z"}
```

Every route works on the caller's own items: another actor's identifier is `not_found`. They need
`task:read`, and a token pinned to one project cannot use them, because an inbox spans projects. Marking
and muting write an audit entry and emit `inbox.updated`; one that changes nothing writes neither. A
change that creates items emits one `inbox.updated` whose payload `recipients` lists the actor
identifiers that received one, which is what the web sidebar's badge listens for.

### Notification channels

A person can deliver their own inbox items to endpoints of their own: a Slack incoming webhook, a Discord
webhook, an ntfy topic URL, or any URL taking signed JSON. These are not the tenant's webhooks: a channel
belongs to the actor who added it and receives only that actor's items, of the kinds it lists, and nothing
of a kind they muted. Each item queues a delivery in the transaction that writes it, and the webhook
dispatcher drains the queue beside its own, with the same claim, retry schedule, network guard and
`X-Tix-Signature` (the event header is `inbox.item`, or `inbox.test` for a test).

| Route | Does |
| --- | --- |
| `GET /channels` | The caller's channels, without their secrets |
| `POST /channels` | Takes `name`, `url`, `format` (`generic`, `slack`, `discord`, `ntfy`), `kinds` (default all) and `active`; answers `201` with the channel and its `secret`, which no later read returns |
| `PATCH /channels/{id}` | Changes any of those fields |
| `DELETE /channels/{id}` | Removes the channel and its deliveries; `204` |
| `POST /channels/{id}/test` | Queues a test message and answers the delivery, attempted already when the server drains inline |
| `GET /channels/{id}/deliveries` | The channel's deliveries, newest first, keyset paged |
| `POST /channel-deliveries/{id}/redeliver` | Queues another attempt; `204` |

Every message says who did what to which task, links the task when the tenant has a domain
(`https://DOMAIN/tasks/REF`), and quotes the comment or description when there is one. Signed JSON
carries `type`, `message`, `task_ref`, `task_title`, `url`, `excerpt` and the `item`. Discord messages
disable mentions, so a comment quoting `@everyone` pings nobody. Another actor's channel or delivery is
`not_found`. The audit log keeps a channel's URL cut to its scheme and host, because a Slack or Discord
webhook URL is itself the credential.

### Copying a task

`POST /tasks/{ref}/copy` makes a new task out of an existing one and answers `201` with the copy. Every field
of the body is optional, and an empty body copies the task into its own project:

```json
{
  "project_ref": "web",
  "title": "rotate keys again",
  "keep_status": true,
  "with_comments": true,
  "with_subtasks": true,
  "with_dependencies": true,
  "link": true
}
```

The copy gets a fresh reference in the target project and carries the title (or `title`), description,
priority, tags, assignee, due date, repeat rule and the custom fields the target defines and accepts. It starts
in the target workflow's initial state, or in the same-named state with `keep_status` when the target has one.
`with_comments` copies the comments as new ones by the caller that name the original author and time,
`with_subtasks` the live subtree, `with_dependencies` the edges (pointed at the copies when both ends were
copied), and `link` records the original as `copied_from_id` on each copy. Stars, claims, history and artifacts
never travel. The whole copy is one transaction: each new task writes a `task.copy` audit entry and emits
`task.created` with `copied_from` in its payload. An unknown project is `404`, a required field the target
lacks `400`, and copying out of the project a token is pinned to `403`.

### Bulk actions

`POST /tasks/bulk` applies one action to up to 500 tasks in one request and answers `200` with an outcome per
distinct reference, in the order given, also when some were refused:

```json
{"action": "move", "refs": ["infra-1", "infra-2", "web-7"], "to": "done"}
```

```json
{
  "action": "move",
  "applied": 2,
  "refused": 1,
  "outcomes": [
    {"ref": "infra-1", "applied": true},
    {"ref": "infra-2", "applied": true},
    {"ref": "web-7", "applied": false, "code": "precondition", "error": "no route leads from \"cancelled\" to \"done\""}
  ]
}
```

| `action` | Argument | Same as |
| --- | --- | --- |
| `move` | `to` | `POST /tasks/{ref}/route`, along the shortest route of each task's workflow |
| `priority` | `priority` (1 to 5) | `PATCH /tasks/{ref}` |
| `tag_add`, `tag_remove` | `tag` | `POST /tasks/{ref}/tags`, `DELETE /tasks/{ref}/tags/{name}` |
| `assign` | `assignee_actor_id`, a handle or id; empty clears | `PATCH /tasks/{ref}` |
| `star`, `unstar` | none | `PUT`, `DELETE /tasks/{ref}/star` |
| `copy` | `project_ref` | `POST /tasks/{ref}/copy` with `link` |
| `delete` | `cascade` (optional) | `DELETE /tasks/{ref}`, soft unless the [trash](#trash) is off |
| `restore` | none | `POST /tasks/{ref}/restore` |

Each task is changed through the single-task operation in its own transaction, with its own audit entry and
event, so a refused task never holds back or undoes another; a copy's outcome names the new task in
`copy_ref`. A reference named twice is acted on once. An unknown action, no references, more than 500, or a
missing argument is `400` before any task changes, and a caller without the scope the action needs (for
example `task:delete` for a bulk delete) gets one `403` for the request.

### Trash

A deleted task goes to the tenant's trash, where `POST /tasks/{ref}/restore` brings it back and
`GET /tasks?trashed=true` lists it. A tenant admin can turn the trash off with `PATCH /tenants/{ref}` and
`{"trash_off": true}`, and back on with `false`; the tenant record reports it as `trash_off`. `GET /trash`
answers any reader with what the tenant does now:

```json
{"enabled": false, "held": 3}
```

With the trash off, `DELETE /tasks/{ref}` and a bulk `delete` remove the task permanently whatever `hard`
says, and both restoring and listing with `trashed=true` are refused with `422`:

```json
{"error":{"code":"precondition_failed","message":"trash is turned off for this tenant, so nothing can be restored from it; a tenant admin can turn it back on","details":{"reason":"trash_off"}}}
```

Turning the trash off deletes nothing. The tasks already in it stay there, out of reach, and `held` counts
them; turning it back on makes them listable and restorable again. Changing the setting writes one
`tenant.update` audit entry and one `tenant.updated` event, in the same transaction as the change.

### CSV

`GET /tasks?format=csv`, or the same listing with `Accept: text/csv`, answers the page as `text/csv` with a
header row and the columns `ref, project, title, status, priority, assignee, tags, due, created, updated`.
Cells are quoted by RFC 4180, tags are space separated, times are RFC 3339 in UTC and the assignee is a handle
where it resolves. A cell starting with `=`, `+`, `-`, `@`, a tab or a carriage return is prefixed with `'` so a
spreadsheet reads it as text. There is no envelope, so the next page's cursor is in the `X-Tix-Next-Cursor`
response header:

```sh
curl -H "Authorization: Bearer $TIX_TOKEN" 'http://127.0.0.1:8080/api/v1/tasks?format=csv&status=todo'
```

### Task templates

A template is a named shape of a task kept apart from tasks: it never appears in a task listing, a claim, the
statistics or a snapshot. `{ref}` is the template's name, matched in any case, or its identifier.

| Route | Does |
| --- | --- |
| `GET /templates` | The templates the caller can read, by name; `?project=KEY` keeps that project's and the tenant-wide ones |
| `POST /templates` | Saves a task as a template; answers `201` with it |
| `GET /templates/{ref}` | One template, its task shape and subtasks included |
| `PATCH /templates/{ref}` | Renames, rescopes or edits it; answers the template |
| `DELETE /templates/{ref}` | Deletes it, leaving tasks made from it alone; answers `204 No Content` |
| `POST /templates/{ref}/use` | Creates a task, with the template's subtasks; answers `201` with the new task |

Saving takes the task and a name, and optionally makes the template tenant-wide, keeps the live subtasks at any
depth and gives it a due offset:

{"name": "release", "task_ref": "infra-4", "tenant_wide": false, "with_subtasks": true, "due_in": "3d"}

The template keeps the title, description, priority, tags, custom fields and repeat rule. A `PATCH` takes any of
`name`, `project_ref` (the empty string makes it tenant-wide), `due_in`, `title`, `body`, `priority`, `tags`,
`custom_fields` (a `null` value removes a field) and `recurrence`. Using one takes an optional `project_ref`
(required in practice for a tenant-wide template in a tenant with several projects), `title` and `due_in` (`"0s"`
for no due date, absent for the template's own offset):

{"project_ref": "web", "title": "Release {N}", "due_in": "72h"}

Titles may carry placeholders, matched in any case: `{DATE}` is today's date as YYYY-MM-DD in UTC, `{N}` the use
number and `{PROJECT}` the target project's key. Other braced words stay as written. A use goes through the same
code as [copying a task](#copying-a-task): the tasks start in the project's initial state, keep the custom fields
it defines with its defaults applied, write a `task.template` audit entry and emit `task.created` with the
template's name in its payload. Saving, editing and deleting write `template.save`, `template.update` and
`template.delete` and emit `template.updated`.

A name taken by another template in any case is `409`. A project template needs `task:write` on its project to
change; a tenant-wide one needs `task:write` and a token that is not pinned to a project, so a pinned token sees
and uses its own project's templates and the tenant-wide ones and cannot change the latter (`403`).

### NDJSON

Send `Accept: application/x-ndjson` and the list streams one object per line with no envelope:

```sh
curl -H "Authorization: Bearer $TIX_TOKEN" -H 'Accept: application/x-ndjson' \
  'http://127.0.0.1:8080/api/v1/tasks?limit=500'
```

### Filtering tasks

Query parameters narrow `/tasks`: project, status, tag, assignee, claimed, blocked and starred state, free
text, and `recurring=true` or `recurring=false` for tasks that do or do not carry a repeat rule (`is:recurring` in
the filter expression). `checklist` and `checklist_open` take `true` or `false` the same way (`has:checklist`
and `checklist:open`).

Soft-deleted tasks are left out unless asked for. `include_deleted=true` adds them to the listing
(`is:deleted`), `trashed=true` lists only them (`is:trashed`), and `trashed=false` lists only live tasks
whatever `include_deleted` says (`-is:trashed`).
the filter expression). Snoozed tasks are left out unless `snoozed=true` (only them) or `snoozed=any`
(beside the rest) asks for them.

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
`task.reclaimed`, `task.deleted`, `task.starred`, `task.unstarred`, `task.snoozed`, `task.unsnoozed`,
`task.woke`, `comment.added`, `artifact.added`,
`dependency.added`, `tag.added`, `project.created`, `project.updated`, `workflow.updated`, `field.updated`,
`webhook.delivered`, `import.completed`, `view.arranged`, `filter.saved`, `filter.updated`, `filter.deleted`,
`inbox.updated`, `template.updated`. A type outside it is an error rather than a filter that silently matches
nothing.

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

Inbox items reach a person's own URL through their [notification channels](#notification-channels),
not through these endpoints, which are tenant configuration filtered by event type. One subscribed to
`inbox.updated` still receives every inbox change in the tenant with its `recipients`.

## Related

- [agents.md](agents.md) for claiming and leases
- [deployment.md](deployment.md) for TLS, proxies and the bind guard
- [scripting.md](scripting.md) for the CLI equivalents
