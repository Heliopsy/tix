# http-api Specification

## Purpose

Serves the product surface over a versioned REST namespace, so a breaking shape change requires a new prefix rather than a change in place.

## Requirements

### Requirement: Versioned API namespace

The HTTP API SHALL be served under the path prefix `/api/v1`. Breaking changes to request or response shapes SHALL require a new version prefix rather than a change in place.

#### Scenario: Versioned path

- **WHEN** a client requests a task collection at `/api/v1/tasks`
- **THEN** the request is routed to the API and returns a JSON response

#### Scenario: Unversioned path

- **WHEN** a client requests `/api/tasks` without a version segment
- **THEN** the server responds with a not found error in the standard error envelope

### Requirement: Resource routes

The API SHALL expose routes for tasks, projects, workflows, field definitions, dependencies, tags, comments, artifacts, claims, tenants, domains, users, tokens, webhooks, audit entries, export, import, sync, and component bundle export and import. Every operation available through the CLI SHALL have a corresponding HTTP route.

#### Scenario: Coverage

- **WHEN** the set of service operations is compared against the registered HTTP routes
- **THEN** every operation resolves to exactly one route on the real server mux

#### Scenario: Method semantics

- **WHEN** a client sends a request method that a route does not support
- **THEN** the server responds with a method not allowed error in the standard error envelope

### Requirement: Task ref path parameter

A route path parameter identifying a task SHALL accept either the task's stable identifier or its human-readable reference such as `infra-42`. Both forms SHALL resolve to the same task within the resolved tenant.

#### Scenario: Human ref

- **WHEN** a client requests a task at a path containing `infra-42`
- **THEN** the task whose project key is `infra` and whose number is 42 within the resolved tenant is returned

#### Scenario: Stable identifier

- **WHEN** a client requests the same task using its stable identifier
- **THEN** the identical task representation is returned

#### Scenario: Ref from another tenant

- **WHEN** a client presents a ref that exists only in a different tenant
- **THEN** the server responds with a not found error and reveals nothing about the other tenant

### Requirement: Error envelope

Every error response SHALL use a single JSON envelope carrying a machine-readable code, a human-readable message, and optional structured details. The code SHALL map one to one with the service error taxonomy and each code SHALL map to exactly one HTTP status.

#### Scenario: Validation failure

- **WHEN** a request fails field validation
- **THEN** the response carries a validation error code, a message, and details naming each offending field

#### Scenario: Stable mapping

- **WHEN** the same service error is produced by two different routes
- **THEN** both responses carry the identical error code and the identical HTTP status

#### Scenario: No leakage

- **WHEN** an internal failure occurs
- **THEN** the response carries a generic internal error code and message and does not include stack traces, SQL text, or credential material

### Requirement: Keyset pagination

All list endpoints SHALL paginate using a keyset cursor over a stable sort key and identifier, SHALL NOT use OFFSET, and SHALL return an opaque cursor for the next page when more results exist. A cursor is opaque by convention only, being an encoding of a sort key and an identifier, so a cursor SHALL describe only a record the caller is permitted to read. An endpoint SHALL NOT return an empty page together with a cursor.

#### Scenario: First page

- **WHEN** a client requests a list endpoint without a cursor
- **THEN** the response contains up to the page limit of items and a next cursor when further items exist

#### Scenario: Following the cursor

- **WHEN** a client requests the same endpoint with the returned cursor
- **THEN** the response continues immediately after the last item of the previous page with no duplicates and no gaps among items unchanged since the first page

#### Scenario: Exhausted list

- **WHEN** the final page of a list is returned
- **THEN** no next cursor is present in the response

#### Scenario: Cursor describes only visible records

- **WHEN** a caller lists a resource whose underlying rows span tenants and the next row belongs to another tenant
- **THEN** the listing continues past that row rather than returning it, and the returned cursor names a record of the caller's own tenant

#### Scenario: Invalid cursor

- **WHEN** a client supplies a malformed or unrecognized cursor
- **THEN** the server responds with a validation error rather than silently returning the first page

### Requirement: Custom field filters on list queries

A list route SHALL accept custom field filters in two forms: a shorthand query parameter per field, spelled `field.<key>=<value>`, and one typed JSON object in a `custom_fields` parameter. A shorthand value SHALL be typed as the JSON scalar it spells, so an unquoted number filters a number and anything that is not JSON stays the string it was typed as. When both forms name the same key, the shorthand SHALL win.

#### Scenario: Shorthand parameter

- **WHEN** a client lists tasks with `?field.severity=high`
- **THEN** only tasks whose `severity` field holds the string `high` are returned

#### Scenario: Typed object

- **WHEN** a client lists tasks with `?custom_fields={"points":3}`
- **THEN** only tasks whose `points` field holds the number 3 are returned, and a task holding the string `3` is not

#### Scenario: Shorthand wins over the object

- **WHEN** a client supplies the same key in both forms, as in `?custom_fields={"severity":"low"}&field.severity=high`
- **THEN** the filter applied is the shorthand value `high`

### Requirement: An unusable custom field filter is rejected

A key the store cannot address, a shorthand key given more than once, and a value that is not a string, number, or boolean SHALL each be rejected with a validation error.

#### Scenario: Malformed key

- **WHEN** a client supplies a key holding anything other than letters, digits, underscores, and dashes, as in `?field.a'b=1`
- **THEN** the server responds with status 400 and an `invalid` error code in the standard envelope

#### Scenario: Unusable value

- **WHEN** a client supplies a value that is neither a string, a number, nor a boolean, as in `?field.severity=[1,2]`
- **THEN** the server responds with status 400 and an `invalid` error code, and no list is returned

### Requirement: Health endpoint

The server SHALL expose a health endpoint that reports whether the process is alive without depending on the database being reachable.

#### Scenario: Process alive

- **WHEN** the health endpoint is requested while the database is unreachable
- **THEN** the endpoint still responds successfully

#### Scenario: No authentication required

- **WHEN** the health endpoint is requested without any credential
- **THEN** the request succeeds and returns no tenant-specific data

### Requirement: Readiness endpoint

The server SHALL expose a readiness endpoint that reports database reachability and whether schema migrations are current. Readiness SHALL report not ready when either check fails.

#### Scenario: Database unreachable

- **WHEN** readiness is requested while the database cannot be reached
- **THEN** the endpoint reports not ready and names the failing check

#### Scenario: Migrations behind

- **WHEN** readiness is requested while the database schema is older than the binary's migration set
- **THEN** the endpoint reports not ready and names the migration check

#### Scenario: Ready

- **WHEN** the database is reachable and migrations are current
- **THEN** the endpoint reports ready

### Requirement: Authentication transport

The API SHALL accept authentication either as a bearer token in the Authorization header or as a session cookie. When both are present, exactly one SHALL be used according to a deterministic order.

#### Scenario: Bearer token

- **WHEN** a request carries a valid API token in the Authorization header
- **THEN** the request is authenticated as that token's actor with that token's scopes

#### Scenario: Session cookie

- **WHEN** a browser request carries a valid session cookie
- **THEN** the request is authenticated as the session's user

#### Scenario: Missing credential when required

- **WHEN** authentication is required and neither credential is present
- **THEN** the server responds with an unauthenticated error in the standard envelope

### Requirement: Tenant resolution precedes authentication

The server SHALL resolve the tenant from the request Host header before authenticating the caller, and on a shared host as "A shared host serves the tenant the header or credential names" says. A credential belonging to a different tenant than the resolved one SHALL be rejected.

#### Scenario: Unknown host

- **WHEN** a request arrives with a Host that maps to no tenant
- **THEN** the server responds with a not found error and does not attempt authentication

#### Scenario: Cross-tenant token

- **WHEN** a token scoped to tenant A is presented on a host that resolves to tenant B
- **THEN** the request is rejected and no data from either tenant is returned

#### Scenario: Matching tenant

- **WHEN** a token scoped to tenant A is presented on a host that resolves to tenant A
- **THEN** the request proceeds and all results are scoped to tenant A

### Requirement: Claim endpoints

The API SHALL expose endpoints to claim a specific task, claim the next unblocked task, renew a lease, and release a claim. When a claim cannot be taken because another worker holds a live lease, the server SHALL respond with a conflict status.

#### Scenario: Contended claim

- **WHEN** a client attempts to claim a task whose lease is held by another worker and has not expired
- **THEN** the server responds with a conflict status and an error code identifying the claim conflict

#### Scenario: Claim next when queue empty

- **WHEN** a client requests the next unblocked task and none is available
- **THEN** the server responds with a distinct empty-queue outcome rather than an error implying failure

#### Scenario: Stale lease token

- **WHEN** a client renews, transitions, or releases using a lease token that is no longer current for the task
- **THEN** the server responds with a conflict status and the task is unchanged

### Requirement: Request size limits

The server SHALL enforce a configurable maximum request body size and SHALL reject a larger body without reading it fully.

#### Scenario: Oversized body

- **WHEN** a client sends a request body exceeding the configured limit
- **THEN** the server responds with a payload too large status in the standard error envelope

#### Scenario: Within limit

- **WHEN** a client sends a body within the configured limit
- **THEN** the request is processed normally

### Requirement: Content type handling

The API SHALL accept JSON request bodies and SHALL respond with JSON. A request carrying a body with an unsupported content type SHALL be rejected.

#### Scenario: Unsupported request type

- **WHEN** a client submits a body with a content type other than JSON to an API route
- **THEN** the server responds with an unsupported media type error

#### Scenario: Malformed JSON

- **WHEN** a client submits a body that is not valid JSON
- **THEN** the server responds with a validation error naming the parse failure and no data is written

#### Scenario: Response type

- **WHEN** any API route responds, including on error
- **THEN** the response declares a JSON content type

### Requirement: Star endpoints

The API SHALL expose `PUT /api/v1/tasks/{ref}/star` to star a task and `DELETE /api/v1/tasks/{ref}/star`
to unstar it, for the authenticated actor. Both SHALL answer `204 No Content` whether or not the
state changed. A task read SHALL carry `"starred": true` when the reading actor has starred it and
SHALL omit the field otherwise. The task list SHALL accept `starred=true` and `starred=false`.

#### Scenario: Star over HTTP

- **WHEN** a client sends `PUT /api/v1/tasks/OPS-1/star`
- **THEN** the response is 204 and a following `GET /api/v1/tasks/OPS-1` carries `"starred": true`

#### Scenario: Unstar over HTTP is idempotent

- **WHEN** a client sends `DELETE /api/v1/tasks/OPS-1/star` for a task it has not starred
- **THEN** the response is 204

### Requirement: Recurrence on the task resource

A task resource SHALL carry `recurrence`, the canonical rule, and `recurrence_next_id`, the successor's
identifier, each omitted when empty. Task creation SHALL accept `recurrence`. A task update SHALL accept
`recurrence`, where an empty string removes the rule and an absent key leaves it unchanged. A rule that does
not parse SHALL be refused with the validation error envelope. `recurrence_next_id` SHALL NOT be accepted
as input.

#### Scenario: Create

- **WHEN** a task is created with `"recurrence": "daily"`
- **THEN** the response carries `"recurrence": "every day"`

#### Scenario: Invalid rule

- **WHEN** an update carries `"recurrence": "every blue moon"`
- **THEN** the response is a 400 validation error and the task is unchanged

### Requirement: Routes beyond the resource collections

Besides the resource routes, the API SHALL serve these routes under `/api/v1`:

- identity and sessions: `GET /whoami`, `POST /auth/login`, `POST /auth/logout`
- the directory: `GET /actors`, `GET /actors/{id}`, and membership at `/members` and
  `/members/{actorID}`

#### Scenario: Every registry route is served

- **WHEN** each HTTP binding in the capability registry is requested on the real server mux
- **THEN** none answers 404 or 405

### Requirement: Second factor and SSH key routes are served

Besides the resource routes, the API SHALL serve these routes under `/api/v1`:

- second factor: `GET`, `POST` and `DELETE /auth/totp`, `POST /auth/totp/confirm`,
  `POST /auth/totp/verify`, `POST /auth/totp/recovery-codes`, and `DELETE /users/{id}/totp` for an
  administrator clearing another user's enrolment
- ssh keys: `/ssh-keys` and `/ssh-keys/{id}`

#### Scenario: A revoked SSH key is still listed

- **WHEN** a client revokes one of its two enrolled SSH keys and then lists its keys at `/api/v1/ssh-keys`
- **THEN** both keys are returned, and the revoked one is reported as revoked

### Requirement: Task movement, retention, report and connection routes are served

Besides the resource routes, the API SHALL serve these routes under `/api/v1`:

- task movement and history: `POST /tasks/{ref}/route` for a multi-hop move, `POST
  /tasks/{ref}/restore`, `GET /tasks/{ref}/tree`, `GET /tasks/{ref}/audit` and
  `POST /tasks/{ref}/claim/reclaim`
- retention: `GET` and `PUT /retention`, `POST /prune`
- reports: `GET /stats` and `GET /status`
- live connections: `GET /connections` and `DELETE /connections/{id}`

#### Scenario: Restoring a deleted task over HTTP

- **GIVEN** a soft-deleted task `infra-3`
- **WHEN** a client holding `task:delete` sends `POST /api/v1/tasks/infra-3/restore`
- **THEN** the task is listed again and the response carries it

### Requirement: Routes beyond the resource collections fail and authorize like the CLI

Each route beyond the resource collections SHALL answer with the standard error envelope on failure
and SHALL be held to the same scopes as the CLI command bound to the same operation.

#### Scenario: A route without its scope

- **WHEN** a client without `tenant:admin` sends `GET /api/v1/connections`
- **THEN** the response is 403 in the standard error envelope

### Requirement: Project icon endpoint

A project read SHALL carry `icon`, `icon_name` and `icon_image` as present, the last being the
digest of an uploaded image. Create and update SHALL accept `icon_name` and `icon_file`, an object
of `content_type` and base64 `data`, beside `icon`. The API SHALL expose
`GET /api/v1/projects/{ref}/icon`, answering with the image bytes, their content type,
`X-Content-Type-Options: nosniff`, a sandboxing content security policy and an ETag, and with not
found when the project has no uploaded icon.

#### Scenario: Upload and fetch over HTTP

- **WHEN** a client patches a project with `icon_file` holding a PNG and then requests `GET /api/v1/projects/{ref}/icon`
- **THEN** the patch answers with a project carrying `icon_image`, and the fetch answers 200 with the same bytes and `Content-Type: image/png`

#### Scenario: A project without an image

- **WHEN** a client requests the icon of a project whose icon is text
- **THEN** the response is 404 in the error envelope

### Requirement: View arrangement endpoints

The API SHALL expose, for the authenticated actor: `GET /api/v1/views/{view}/groups` returning the
actor's preferences on the view; and `PATCH /api/v1/views/{view}/groups/{group}` taking any of `hidden`,
`collapsed` and `starred` and returning the resulting preference. A view is `board`, `lanes` or
`project:KEY`; anything else SHALL be refused as invalid, and a group the view cannot hold SHALL be
refused as invalid or not found.

#### Scenario: Hiding a column over HTTP

- **WHEN** a client sends `PATCH /api/v1/views/board/groups/done` with `{"hidden": true}`
- **THEN** the response carries `"hidden": true` and a following `GET /api/v1/views/board/groups` lists it

#### Scenario: An unknown view is refused

- **WHEN** a client sends `GET /api/v1/views/calendar/groups`
- **THEN** the response is 400

### Requirement: View arrangement order and reset endpoints

The API SHALL expose, for the authenticated actor: `PUT /api/v1/views/{view}/order` taking `groups` and
returning the preferences it wrote; and `DELETE /api/v1/views/{view}/groups` forgetting every preference
on the view, or only the one named by the `group` query parameter, and answering `204 No Content`.

#### Scenario: Resetting one group over HTTP

- **GIVEN** an actor with preferences on the `done` and `todo` groups of `project:infra`
- **WHEN** a client resets the view naming `done` in the `group` query parameter
- **THEN** listing the view's preferences returns only the one for `todo`

### Requirement: The project deletion route takes the task choice and the plan has a route

`DELETE /api/v1/projects/{ref}` SHALL accept the query parameters `move_tasks_to` and `delete_tasks`, SHALL answer
a project holding tasks and given neither with the precondition error envelope, and SHALL answer a successful
deletion with `200` and the deletion's result. `GET /api/v1/projects/{ref}/deletion` SHALL accept the same
parameters and return the plan without writing.

#### Scenario: Deleting with no choice over HTTP

- **WHEN** a project holding tasks is deleted over HTTP without a parameter
- **THEN** the response is the precondition error and the project remains

#### Scenario: Planning over HTTP

- **WHEN** `GET /api/v1/projects/{ref}/deletion?move_tasks_to=other` is requested
- **THEN** the response carries the task counts and the status mapping

### Requirement: The project PATCH changes the key

`PATCH /api/v1/projects/{ref}` SHALL accept a `key` member that changes the project's key, answering
with the updated project. A key held by another project SHALL be answered with 409 and an invalid key
with 400. A path naming a project or task by a kept alias SHALL be served as if it named the current
key, and the response SHALL carry the current key and reference.

#### Scenario: Renaming over the API

- **WHEN** a client sends `PATCH /api/v1/projects/infra` with `{"key":"ops"}`
- **THEN** the response is 200 with `"key":"ops"`

#### Scenario: Reading by an old reference

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a client sends `GET /api/v1/tasks/infra-8`
- **THEN** the response is 200 with `"ref":"ops-8"`

### Requirement: Copy endpoint

The API SHALL expose `POST /api/v1/tasks/{ref}/copy` taking a JSON body with optional `project_ref`,
`title`, `keep_status`, `with_comments`, `with_subtasks`, `with_dependencies` and `link`. It
SHALL answer `201 Created` with the root copy. A task read SHALL carry `copied_from_id` when the
task records one.

#### Scenario: Copy over HTTP

- **WHEN** a client sends `POST /api/v1/tasks/OPS-3/copy` with `{"project_ref": "WEB", "link": true}`
- **THEN** the response is 201 and its body is a WEB task whose `copied_from_id` is OPS-3's identifier

#### Scenario: Unknown target

- **WHEN** a client copies to a project that does not exist
- **THEN** the response is 404

### Requirement: The task listing takes a trashed parameter

`GET /api/v1/tasks` SHALL accept `trashed=true`, listing only soft-deleted tasks, and `trashed=false`,
listing only live ones. The client SHALL send the filter contract's `Trashed` field as that parameter.

#### Scenario: Asking for the trash over the API

- **GIVEN** a deleted task and a live one
- **WHEN** a client lists tasks with `trashed=true`
- **THEN** only the deleted task is returned

### Requirement: Bulk endpoint

The API SHALL expose `POST /api/v1/tasks/bulk` taking a JSON body with `action`, `refs` and the
action's argument (`to`, `priority`, `tag`, `assignee_actor_id`, `project_ref`, `cascade`), and
SHALL answer `200 OK` with the per-task outcomes and the applied and refused counts, also when
some tasks were refused.

#### Scenario: Bulk over HTTP

- **WHEN** a client sends `POST /api/v1/tasks/bulk` with `{"action": "priority", "refs": ["OPS-1", "OPS-2"], "priority": 1}`
- **THEN** the response is 200, both outcomes are applied, and both tasks have priority 1

#### Scenario: Unknown action

- **WHEN** a client sends a bulk body whose action is `explode`
- **THEN** the response is 400

### Requirement: Task listing as CSV

`GET /api/v1/tasks` SHALL answer with `text/csv` when the request carries `format=csv` or an
`Accept` header naming `text/csv`. The body SHALL hold a header row and one row per task of the
page, in the columns `ref, project, title, status, priority, assignee, tags, due, created,
updated`, escaped by RFC 4180, with any cell starting with `=`, `+`, `-`, `@`, a tab or a
carriage return prefixed with `'`. The next cursor, when there is one, SHALL be named in the
`X-Tix-Next-Cursor` response header.

#### Scenario: CSV by query parameter

- **WHEN** a client requests `GET /api/v1/tasks?format=csv`
- **THEN** the response has content type `text/csv` and its first line is the header row

#### Scenario: Formula guard

- **WHEN** a task titled `=HYPERLINK("x")` is listed as CSV
- **THEN** its title cell reads `'=HYPERLINK("x")`, quoted

### Requirement: Saved filter routes

The API SHALL serve `GET /api/v1/filters`, `POST /api/v1/filters` (answering `201`), `GET /api/v1/filters/{filter}`,
`PATCH /api/v1/filters/{filter}`, `PUT /api/v1/filters/order` and `DELETE /api/v1/filters/{filter}` (answering `204`)
over the saved filter operations, with the filter addressed by identifier or name.

#### Scenario: Save and read back

- **WHEN** a client posts `{"name": "ops", "query": "tag:ops"}` to `/v1/filters`
- **THEN** the answer is `201` with the filter, and `GET /api/v1/filters/ops` answers the same filter

### Requirement: Quick-add over HTTP

`POST /api/v1/tasks` SHALL accept `"parse": true`, which reads `title` through the quick-add
parser, resolving relative dates in UTC, before the task is created. Without it the title
SHALL be stored literally. A field in the body that the line contradicts SHALL be refused
with 400.

#### Scenario: Parsed create

- **WHEN** a client posts `{"title": "Fix login !high #auth", "parse": true}`
- **THEN** the response is 201 and the task is titled `Fix login` with priority high and the tag `auth`

#### Scenario: Unknown project

- **WHEN** a client posts `{"title": "Fix login +nope", "parse": true}` and no project has that key
- **THEN** the response is 404 and no task is created

### Requirement: A task read carries its checklist count

A task in any read or update response SHALL carry `checklist` with `done` and `total` when its
description has GitHub task-list items, and SHALL omit the field when it has none. The count SHALL be
computed from the body on read and SHALL NOT appear in audit snapshots or exports.

#### Scenario: The count travels with the task

- **GIVEN** a task whose body is `- [x] a\n- [ ] b`
- **WHEN** it is read
- **THEN** the response carries `"checklist": {"done": 1, "total": 2}`

### Requirement: The API ticks one checklist item

`POST /api/v1/tasks/{ref}/checklist` SHALL take `item`, counted from one in reading order, `done`, and an
optional `version`, and SHALL require `task:write`. It SHALL rewrite only that item's marker, record a
`task.update` audit entry and event in the same transaction, and answer with the task. A stale version
SHALL answer `409`; an item number the description does not have SHALL answer `400`; an item already in
the asked-for state SHALL answer `200` and write nothing.

#### Scenario: A stale tick is refused

- **WHEN** a tick names a version the task has moved past
- **THEN** the answer is `409` and the body is unchanged

### Requirement: The task listing filters by checklist

`GET /api/v1/tasks` SHALL accept `checklist=true|false`, selecting tasks whose body does or does not
carry a checklist marker, and `checklist_open=true|false`, selecting those that do or do not carry an
unticked one.

#### Scenario: Only open checklists

- **WHEN** a listing asks for `checklist_open=true`
- **THEN** only tasks with an unticked marker are listed

### Requirement: Calendar feed endpoints

The API SHALL expose, for the authenticated actor: `GET /api/v1/calendar` returning their feed's
metadata; `POST /api/v1/calendar` taking an optional `filter` and answering `201` with the feed, its
secret and its absolute `url`; `PATCH /api/v1/calendar` taking `filter`; `POST
/api/v1/calendar/rotate` answering with the new secret and `url`; and `DELETE /api/v1/calendar`
answering `204`. It SHALL serve `GET /api/v1/calendar/feeds/{token}.ics` without a credential.

#### Scenario: Creating a feed over HTTP returns its URL once

- **WHEN** a client sends `POST /api/v1/calendar` with `{}`
- **THEN** the answer is 201 with a `url` ending `.ics`, and a following `GET /api/v1/calendar`
  carries no secret

#### Scenario: The feed route needs no credential

- **WHEN** a client with no Authorization header and no cookie fetches the `url` it was given
- **THEN** the answer is 200 with `Content-Type: text/calendar; charset=utf-8`

### Requirement: Inbox endpoints

The API SHALL expose, for the authenticated actor: `GET /api/v1/inbox` listing items newest first with
`limit`, `cursor` and `unread`, and a `next_cursor`; `GET /api/v1/inbox/count` returning `unread`;
`PUT /api/v1/inbox/{id}/read` and `DELETE /api/v1/inbox/{id}/read` marking one item read or unread and
returning it; and `POST /api/v1/inbox/read-all` returning how many it marked.

#### Scenario: Paging the inbox

- **GIVEN** an actor with three items
- **WHEN** a client sends `GET /api/v1/inbox?limit=2` and then follows `next_cursor`
- **THEN** it sees the three items newest first, each once

### Requirement: Inbox preference endpoints

The API SHALL expose, for the authenticated actor, `GET /api/v1/inbox/prefs` and `PUT /api/v1/inbox/prefs`
reading and replacing the muted kinds. An unknown kind SHALL be refused as invalid.

#### Scenario: Muting an unknown kind

- **WHEN** a client sends `PUT /api/v1/inbox/prefs` with `{"muted": ["everything"]}`
- **THEN** the response is 400

### Requirement: Notification channel endpoints

The API SHALL expose `GET /api/v1/channels`, `POST /api/v1/channels` (answering 201 with the channel and its
`secret`), `PATCH /api/v1/channels/{id}`, `DELETE /api/v1/channels/{id}`, `POST /api/v1/channels/{id}/test`,
`GET /api/v1/channels/{id}/deliveries` (keyset paginated, newest first) and
`POST /api/v1/channel-deliveries/{id}/redeliver`, all over the authenticated actor's own channels.

#### Scenario: The secret is returned once

- **WHEN** a client creates a channel and then lists channels
- **THEN** the creation carries `secret` and the listing does not

### Requirement: Snooze over HTTP

`PUT /api/v1/tasks/{ref}/snooze` with `{"until": RFC3339}` SHALL snooze the task and return it,
`DELETE /api/v1/tasks/{ref}/snooze` SHALL unsnooze it and return it, and
`POST /api/v1/snoozes/wake` with an optional `{"limit": N}` SHALL run one wake pass and return
`{"woken": N}`. `GET /api/v1/tasks` SHALL take `snoozed=true|false|any`.

#### Scenario: Snooze and list

- **WHEN** a client snoozes OPS-3 over HTTP and lists tasks with `snoozed=true`
- **THEN** the listing holds OPS-3 with its `snoozed_until`

### Requirement: Template endpoints

The API SHALL expose `GET /api/v1/templates` (with `project`, `limit` and `cursor`),
`POST /api/v1/templates` taking `name`, `task_ref`, `tenant_wide`, `with_subtasks` and `due_in`,
`GET`, `PATCH` and `DELETE /api/v1/templates/{ref}`, and `POST /api/v1/templates/{ref}/use` taking
`project_ref`, `title` and `due_in`, answering `201 Created` with the new root task.

#### Scenario: Use over HTTP

- **WHEN** a client posts `{"due_in": "3d"}` to `/v1/templates/release/use`
- **THEN** the response is 201 and its body is the new task with a due date

#### Scenario: Unknown template

- **WHEN** a client reads `/v1/templates/nope`
- **THEN** the response is 404

### Requirement: The API sets and reports the trash

`PATCH /api/v1/tenants/{ref}` SHALL accept `trash_off`. `GET /api/v1/trash` SHALL answer `enabled` and
`held`. A refusal because the trash is off SHALL be `422 precondition_failed` with `details.reason`
`trash_off`.

#### Scenario: Restoring over the API with the trash off

- **GIVEN** a task in the trash and the trash turned off
- **WHEN** a client posts to the task's restore route
- **THEN** it answers 422 with `details.reason` `trash_off`

### Requirement: The API takes keys and references in any case and answers the stored value

Every API route and query parameter that names a project or a task SHALL accept it in any case.
Responses, webhook payloads and events SHALL carry the stored lower-case key and reference, unchanged
from earlier releases.

#### Scenario: Upper-case reference

- **WHEN** a client requests `GET /api/v1/tasks/INFRA-1`
- **THEN** the answer is the task with `"ref": "infra-1"`

#### Scenario: Upper-case project filter

- **WHEN** a client requests `GET /api/v1/tasks?project=INFRA`
- **THEN** the tasks of `infra` are listed

### Requirement: The API exposes housekeeping

`GET /api/v1/housekeeping` SHALL answer the policy, with `?project=KEY` the override and effective rules, and
the preview counts. `PUT /api/v1/housekeeping` SHALL set either level. `POST /api/v1/housekeeping/run` SHALL
run one pass, or preview with `dry_run`.

#### Scenario: Refused while the trash is off

- **GIVEN** a tenant whose trash is off
- **WHEN** a client puts `{"done": "30"}`
- **THEN** it answers 422 with `details.reason` `trash_off`

### Requirement: The API sets and explains lease length

`PATCH /api/v1/tasks/REF` SHALL accept `lease` and `PATCH /api/v1/projects/KEY` SHALL accept `lease_default` and
`lease_max`. `GET /api/v1/tasks/REF/lease` SHALL answer with the lease a claim naming no ttl would be granted,
with `ttl`, `source`, `max`, `max_source` and `clamped`. Claim and renew responses SHALL carry the same as `lease`.

#### Scenario: Reading a task's lease

- **WHEN** a task's lease is 4h and a client reads `/api/v1/tasks/REF/lease`
- **THEN** the answer has `source` `task` and `ttl` of four hours

### Requirement: SSH key sign-in routes

`POST /api/v1/auth/ssh/challenge` and `POST /api/v1/auth/ssh/verify` SHALL be reachable without a
credential, after tenant resolution, like `POST /api/v1/auth/login`. The challenge route SHALL answer
`nonce`, `namespace` and `expires_at`, the namespace built from the request's Host. The verify route
SHALL take `public_key`, `signature`, `nonce` and optional `tenant`, SHALL answer a login result shaped
like the password route's, and SHALL set the session cookie when a session is issued.

#### Scenario: A challenge names the host

- **WHEN** a client posts to the challenge route on host `tix.example`
- **THEN** the namespace is `tix-login@tix.example`

#### Scenario: Verifying without a credential

- **WHEN** an anonymous client posts a valid signature to the verify route
- **THEN** it answers 200 with a session

### Requirement: The API sets a lease when it creates a task

`POST /api/v1/tasks` SHALL accept `lease`, a duration such as `"4h"`. A lease above the effective maximum SHALL be
refused with `invalid` and create nothing.

#### Scenario: Creating a task with a lease over HTTP

- **WHEN** a client posts a task with `"lease": "2h"`
- **THEN** the created task carries a two-hour lease

### Requirement: The API serves its OpenAPI document

The server SHALL answer `GET /api/v1/openapi.json` without a credential and before tenant resolution with the
OpenAPI 3.1 document of the API it runs, as JSON. The document SHALL be generated from the capability registry,
committed as `api/openapi.yaml` and embedded in the binary, and SHALL NOT be edited by hand.

#### Scenario: Reading the document without a credential

- **WHEN** a client sends `GET /api/v1/openapi.json` with no `Authorization` header
- **THEN** the response is `200` with an `application/json` document whose `openapi` field starts with `3.1`

#### Scenario: Every operation names its authority

- **WHEN** an operation of the document needs a credential
- **THEN** its security names the bearer token and the session cookie with the scope the service enforces, and an
  operation served without a credential has an empty security list

### Requirement: The OpenAPI document holds exactly the served routes

The document SHALL describe one operation for every method and path the router registers, and none it does not.
Each operation SHALL carry the registry name as its operation identifier, the first segment of that name as its
tag, its request body, query and path parameters, its success response and the error statuses the error taxonomy
maps to, each answering the standard error envelope.

#### Scenario: A route registered without a registry entry

- **WHEN** a handler is bound to the router with no registry operation or endpoint describing it
- **THEN** the build fails naming the route

#### Scenario: The committed document is stale

- **WHEN** a route, a request or response type, or a doc comment changes and `just openapi` was not run
- **THEN** the build fails naming the first line that differs

### Requirement: Responses match the OpenAPI document

Every request the HTTP API accepts and every response it gives SHALL validate against the document's schema for
that operation and status. A query parameter the operation does not declare SHALL be a contract failure.

#### Scenario: A handler answers a shape the document does not describe

- **WHEN** a handler's response differs from the type its registry entry declares
- **THEN** the API tests fail naming the operation, the status and the field

### Requirement: Every OpenAPI parameter is described

Every query and path parameter of the OpenAPI document SHALL carry a description, and so SHALL every parameter
family an operation reads by prefix. A query parameter's description SHALL be the doc comment of its field on
the operation's query type. A parameter without one SHALL fail the build.

#### Scenario: A query field loses its doc comment

- **WHEN** a field of a query type has no doc comment and `just openapi` runs
- **THEN** the openapi tests fail naming the operation and the parameter

#### Scenario: A tool argument carries the parameter's description

- **WHEN** `tix mcp` builds the `task_list` tool
- **THEN** its `snoozed` argument's schema carries the description of `TaskListQuery.Snoozed`

### Requirement: A task update tells an absent due date from a null one

`PATCH /api/v1/tasks/{ref}` SHALL treat `due_at` in three ways: an absent key SHALL leave the due date
unchanged, `null` SHALL clear it, and an RFC 3339 timestamp SHALL set it. The OpenAPI document SHALL
describe the field as nullable.

#### Scenario: Null clears the due date

- **WHEN** a task with a due date is updated with `{"due_at": null}`
- **THEN** the task read back has no `due_at`

#### Scenario: An absent key leaves it

- **WHEN** a task with a due date is updated with `{"title": "renamed"}`
- **THEN** the task read back keeps its due date

#### Scenario: A remote client clears it

- **WHEN** `tix task edit REF --due ""` runs against `--server`
- **THEN** the server clears the task's due date

### Requirement: The source body and response carry the settings

`PUT /api/v1/sync/sources` SHALL store `config` and `mapping_path`, and its response and the source listing
SHALL return them without any credential.

#### Scenario: Round trip

- **WHEN** a source is put with config `url` and a mapping path
- **THEN** the response and `GET /api/v1/sync/sources` both return that url and mapping path

### Requirement: Machine interfaces read dates in one format only

The HTTP API, including `"parse": true` on task creation, and the MCP tools generated from it SHALL read
dates as `YYYY-MM-DD` or RFC 3339 only, whatever date format a deployment or a browser has chosen, because
a program must not depend on a person's preference or a browser cookie.

#### Scenario: A dotted date over HTTP

- **WHEN** a client creates a task with `"parse": true` and a title holding `due:06.11.2026`
- **THEN** the request is refused, as it is under every configured format

### Requirement: Tenant creation returns the first admins

`POST /api/v1/tenants` SHALL accept `admins`, a list of `email`, `handle`, `display_name` and `password`,
and SHALL answer 201 with the tenant's fields plus `admins` and, for generated passwords,
`initial_credentials` of `email`, `handle` and `password`.

#### Scenario: Creating with one admin

- **WHEN** a global admin posts a tenant with one admin and no password
- **THEN** the answer is 201 and `initial_credentials` holds one password for that email

### Requirement: Global admin routes

The API SHALL serve `GET` and `POST /api/v1/global-admins`, `DELETE /api/v1/global-admins/{email}`,
`GET` and `POST /api/v1/tenants/{ref}/users`, and `PATCH` and `DELETE /api/v1/tenants/{ref}/users/{id}`,
all for a global admin only.

#### Scenario: A tenant admin's token

- **WHEN** a token holding `*` but not `global:admin` lists `/api/v1/tenants/other/users`
- **THEN** the answer is 403

### Requirement: Invite and password change endpoints

The API SHALL accept `require_password_change` on `POST /api/v1/users` and `PATCH
/api/v1/users/{id}`, and `new_password` on `POST /api/v1/auth/login`. It SHALL serve `POST|GET
/api/v1/invites`, `DELETE /api/v1/invites/{id}` and `POST /api/v1/invites/{id}/reissue` with
`user:admin`, and `GET|POST /api/v1/auth/invites/{token}` without a credential. An issued invite
SHALL carry its absolute `url`.

#### Scenario: Creating an invite over HTTP

- **WHEN** an administrator sends `POST /api/v1/invites` with `{"email": "ada@example.com"}`
- **THEN** the answer is 201 with a `url` built from the request's own scheme and host

### Requirement: The caller's date settings over HTTP

`GET /api/v1/me/time` SHALL return the deployment, tenant and personal levels, the values in force and the
level each came from. `PATCH /api/v1/me/time` SHALL set the caller's own `time_format` and `timezone`,
where `""` inherits and an absent field is left alone. A caller with no user record SHALL be refused
`invalid` by the `PATCH`.

#### Scenario: An agent cannot hold a preference

- **GIVEN** a token held by an agent
- **WHEN** it sends `PATCH /api/v1/me/time` with `{"time_format": "us"}`
- **THEN** the response is `invalid` and nothing is stored

#### Scenario: Unknown values are refused

- **WHEN** a person sends `{"timezone": "Mars/Olympus"}` or `{"timezone": "local"}`
- **THEN** the response is `invalid`

### Requirement: A tenant's default date settings over HTTP

`PATCH /api/v1/tenants/REF` SHALL accept `time_format` and `timezone`, where `""` returns the field to the
deployment default.

#### Scenario: Tenant default set

- **WHEN** an administrator sends `{"time_format": "dotted"}`
- **THEN** members who chose nothing read dotted timestamps

### Requirement: Audit routes filter by via and token

GET /api/v1/audit and the task audit route SHALL accept repeatable via and token query parameters, and every audit entry they return SHALL carry via.

#### Scenario: Filtering by channel

- **WHEN** GET /api/v1/audit?via=mcp is requested
- **THEN** only entries recorded over MCP are returned, each with its via

### Requirement: Clients name themselves in X-Tix-Client

The tix client SHALL send X-Tix-Client: cli on every request, and tix mcp SHALL send X-Tix-Client: mcp. The server SHALL read the header only to refine the channel of a token request.

#### Scenario: tix mcp is told apart

- **WHEN** tix mcp creates a task with a token the CLI also uses
- **THEN** the MCP change records mcp and the CLI change records cli-remote

### Requirement: Project task export endpoint

The API SHALL expose `POST /api/v1/projects/{ref}/tasks/export` taking the export choices as a JSON body,
and SHALL answer with the file as `text/csv`, `application/json`, `application/x-ndjson` or
`text/markdown` with a `Content-Disposition` naming it, or with the error envelope when refused before
the first byte.

#### Scenario: JSON export over HTTP

- **WHEN** a client posts `{"format":"json","columns":["title"]}` to the export route of INFRA
- **THEN** the response is 200 `application/json`, an array of objects with a `title` key, named `infra-tasks.json`

#### Scenario: Refused export

- **WHEN** a client posts an export naming an unknown column
- **THEN** the response is 400 in the error envelope with no `Content-Disposition`

### Requirement: Project task import endpoint

The API SHALL expose `POST /api/v1/projects/{ref}/tasks/import` taking the import choices as JSON with the
file as `content`, or as a multipart form with the file in a `file` part, and SHALL answer with the result,
`committed` false when any row is in error.

#### Scenario: Multipart import

- **WHEN** a client posts a multipart form with a CSV in `file` and a mapping field
- **THEN** the response is 200 with the tasks created and `committed` true

### Requirement: Import upload limit

The server SHALL cap the body of a project task import, over the API and in the browser's upload step, at `server.max_import_bytes`, 32 MiB by default, in place of the general body limit, which SHALL still cap every other request. A larger import SHALL be answered with 413 and a message naming the limit and the key.

#### Scenario: An import larger than the general limit

- **WHEN** a client uploads a 5 MiB file to the import endpoint of a server with a 1 MiB body limit
- **THEN** the import is read and answered with 200

#### Scenario: An import over its own limit

- **WHEN** a client uploads a file larger than `server.max_import_bytes`
- **THEN** the response is 413 with a message naming the limit and `server.max_import_bytes`

#### Scenario: Other routes keep the general limit

- **WHEN** a client posts a body larger than the general limit to any route that is not an import
- **THEN** the response is 413

### Requirement: whoami lists the caller's memberships

`GET /whoami` SHALL report the caller's account identifier and, for a session or a token carrying the `account:read` scope, every tenant the account is a member of with its key, name, role, actor and host. The `*` scope SHALL NOT imply `account:read`.

#### Scenario: Session caller in two tenants

- **WHEN** a person who is a member of HO and `test` calls whoami with a session
- **THEN** the answer lists both tenants with the role held in each

#### Scenario: Token without the scope

- **WHEN** a token carrying `*` but not `account:read` calls whoami
- **THEN** the answer describes the token's actor and lists no memberships

### Requirement: A shared host serves the tenant the header or credential names

A host no tenant domain claims, and a host whose domain is marked shared, SHALL be resolved alike, once per request before authentication: the tenant the `X-Tix-Tenant` header names, else the tenant of the session or token presented, else the domain's tenant or, on an unclaimed host, the default tenant.

#### Scenario: Sign-in to a named tenant on the shared host

- **WHEN** a sign-in on the shared host carries `X-Tix-Tenant: test`
- **THEN** the sign-in is checked against tenant `test` and the session minted belongs to it

#### Scenario: Credential of another tenant on a shared domain

- **WHEN** a session of tenant `test` is presented, with no header, on tix.z09.org, a shared domain of HO
- **THEN** the request is served as tenant `test`, and reads nothing of HO

#### Scenario: Nothing names a tenant

- **WHEN** a request with no header and no credential reaches a shared domain of HO
- **THEN** it is served as HO

#### Scenario: Header and credential disagree

- **WHEN** a request on the shared host names tenant A in the header and presents a credential of tenant B
- **THEN** it is refused as a credential not valid for the request's tenant

### Requirement: A bound host serves its own tenant alone

On a host claimed by a domain not marked shared, a header naming another tenant SHALL be refused before authentication, and a credential of another tenant SHALL be refused as not valid for the host.

#### Scenario: Header on a bound host

- **WHEN** a request to a host claimed by tenant HO, not marked shared, carries `X-Tix-Tenant: test`
- **THEN** it is refused and nothing is read from either tenant

#### Scenario: Another tenant's session on a bound host

- **WHEN** a session of tenant `test` is presented on a host bound to HO
- **THEN** it is refused as unauthenticated

### Requirement: A tenant with its own address is not served on a shared host

A shared host SHALL refuse a header naming a tenant that has a domain of its own not marked shared, other than the host's own tenant, and SHALL ignore a credential of such a tenant, so that tenant's sessions live at its own address.

#### Scenario: Tenant at its own address

- **WHEN** tenant `test` has its own domain and a request on a shared host names it in the header
- **THEN** it is refused as forbidden

### Requirement: Tenant switch routes

The API SHALL serve `POST /auth/switch`, taking a target tenant key and an optional code, and answering with a session for the same host or a code challenge.

#### Scenario: Switch on the same host

- **WHEN** a session on the shared host posts a switch to a tenant the person belongs to
- **THEN** the answer carries a new session for that tenant and the old session no longer authenticates

#### Scenario: A token switches

- **WHEN** a request authenticated by an API token posts a switch
- **THEN** it is refused as invalid and no session is minted

### Requirement: Domain update routes

The API SHALL serve `PATCH /domains/{hostname}`, taking `shared`, and `GET /tenants/{ref}/domains`, both for a global admin only, and SHALL answer anybody else as forbidden.

#### Scenario: Tenant admin token

- **WHEN** a token of a tenant admin carrying `*` patches a domain's `shared`
- **THEN** it is answered 403 and the domain is unchanged

### Requirement: Account log routes

The API SHALL serve `GET /account/log` for the caller's own account and `GET /accounts/{email}/log` for a global admin, both keyset paginated.

#### Scenario: Global admin reads a person's log

- **WHEN** a global admin requests the log of villy@
- **THEN** the answer lists villy@'s account changes across every tenant
