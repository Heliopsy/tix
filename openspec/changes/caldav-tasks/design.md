# Design

## Where it lives

CalDAV is its own protocol, described by its own discovery rather than by the OpenAPI document, which is
exactly the position SCIM is in. It follows that precedent:

- `internal/caldav` translates WebDAV and CalDAV to a narrow interface, `core.TaskCollections`, and holds no
  rule of its own. It is mounted by `httpapi.Config.CalDAV` under `/dav/`, the way `Config.SCIM` is mounted
  under `/scim/v2/`, and `/.well-known/caldav` is registered before the browser's catch-all.
- `core.TaskCollections` is implemented by `service.Local` and is not part of `core.Service`. Its methods
  are `DAVPrincipal`, `ListDAVCollections`, `DAVSnapshot`, `GetDAVObjects`, and in phase 2 `PutDAVObject`
  and `DeleteDAVObject`. Phase 2 methods do not write anything themselves: they translate to `UpdateTask`,
  `CreateTask` and the multi-hop move, so every guard, audit entry and event is the one those already write.
- `internal/ical` gains `RenderTodo` beside the feed's `Render`, sharing `Text` and `Fold`, and in phase 2 a
  reader for the subset of RFC 5545 tix maps.
- The management operations (`dav.info`, `dav.password.list|create|revoke`) are ordinary service methods with
  registry entries, CLI, HTTP and web bindings, and land in `api/openapi.yaml`.

`internal/caldav` routes are not registry operations, the same as SCIM's. They are listed in
`capability.Endpoints` only if the contract test asks for every mux pattern to be accounted for; the SCIM
handler shows that a subtree handler mounted as one pattern is enough.

## Library or hand-written

`github.com/emersion/go-webdav` v0.7.0 has a CalDAV server, and it was read before deciding. Against it:

- Its server implements no `sync-collection` and no `getctag`. Both are what makes a phone's sync cheap, and
  Apple Reminders and DAVx5 both use `sync-collection` when it is offered. Adding them means forking or
  wrapping the handler, at which point most of the library is bypassed.
- Its `Backend` evaluates `calendar-query` over whatever `ListCalendarObjects` returns, does not bound
  request bodies, and decides privileges and paths in ways that would have to be overridden for tenant keys
  in the path and for a read-only phase.
- It pulls `go-ical`, which pulls `rrule-go`, and `go-vcard` for CardDAV, which is a non-goal. That is three
  modules to review for one feature, against the repo's rule that a new dependency needs a reason.

The protocol subset tix needs is small: four methods, three reports, about twenty properties, and a VTODO
renderer that already half exists in `internal/ical`. `encoding/xml` parses it, and the SAML code already
shows how the repo handles hostile XML. Recommendation: **hand-written, standard library only**, with
request and response fixtures captured from the real clients as the test corpus. go-webdav stays useful as a
reading reference, and its client could drive a test, but not as a module dependency.

## Paths

```text
/.well-known/caldav                          301 to /dav/
/dav/                                        context root: current-user-principal
/dav/TENANT/principals/ACTOR/                the person in that tenant
/dav/TENANT/calendars/ACTOR/                 calendar-home-set
/dav/TENANT/calendars/ACTOR/assigned/        Assigned to me
/dav/TENANT/calendars/ACTOR/p-PROJECT/       one per project, PROJECT the project's identifier
/dav/TENANT/calendars/ACTOR/COLLECTION/NAME.ics
```

`TENANT` is the tenant key, as the shared contract asks. `ACTOR` and `PROJECT` are lowercase identifiers,
not handles or project keys, because a client stores these URLs and a renamed project or a changed handle
would otherwise look like a deleted collection and a new one, and the client would download everything again.
`NAME` is the task's lowercase identifier, or for a task created by a client in phase 2 the name the client
chose. A tenant key change does break stored URLs; that is rare and an administrator act, and the docs say a
client must be added again after one.

## Tenancy

Tenant isolation is the hard requirement, and it is decided before any task is read:

1. The router resolves the tenant as it does for every request. On a host bound to one tenant that is the
   host's tenant. On a shared host, the tenant key in the path plays the part `X-Tix-Tenant` plays for the
   API, and under `/dav/` with no key in the path (the context root) the tenant of the Basic password does,
   the way a bearer token's does today. `credentialHash` learns to read a Basic password, for `/dav/` only.
2. The app password is looked up by hash inside that tenant only, as every token is.
3. The tenant key in the path must be that tenant's key, and `ACTOR` must be the password's own actor.
   Anything else is `404`, never `403`, so the answer to "does tenant X or person Y exist" is the same as
   for nonsense.
4. Every read runs in a tenant-scoped transaction through the existing builder; the DAV code never builds
   a query.

The tests use two tenants with the same project keys, the same handles and the same email on one account, and
assert that every path and every report crossing between them is `404`.

## Credentials

### An app password is an API token

It is a row in `api_tokens` with the scope `caldav` and nothing else, so expiry, revocation, `last_used_at`,
the token list, rotation and the audit trail all already exist. This differs from the calendar feed, which has
its own table, and the reasons that justified a table there do not apply: the secret travels in an
Authorization header, not a URL a third party stores and logs, and there is no filter to carry.

`caldav` is **named-only** (`Scope.NamedOnly`), like `scim:provision`: `*` and roles do not imply it, a
token carrying it carries nothing else, no project pin and no claim requirement, and the token verifier
refuses it everywhere but `/dav/` (`caldav_only`, `403`). It is minted only for the caller's own person actor.

### Who may create one

`CreateToken` needs `token:admin`, which only admins hold, so an app password cannot be an ordinary
`tix token create --scope caldav`: a member could not make one for their own phone. It has its own methods
behind a new action, `dav.password`, which needs `task:read` and is not project-confinable, the shape of
`calendar.feed`:

- `CreateDAVPassword(name, collections, read_only)` mints a `caldav` token for the caller's own actor.
  `CreateToken` refuses the `caldav` scope and points at `tix dav password create`, so there is one minting
  path with one set of rules.
- `ListDAVPasswords` lists the caller's own app passwords as metadata, revoked ones included.
- `RevokeDAVPassword(id)` revokes one of the caller's own. An admin revokes anybody's through the existing
  `tix token revoke`, which already records `token.revoke_other`.

The caller must be a person: an agent actor is refused with `403`. A session (password or SSO, browser or
app) and a personal API token with `task:read` may create one. A `caldav` token may not, which the verifier
already guarantees by refusing it off `/dav/`.

Token names are unique per tenant, so two people each calling theirs `iPhone` would collide. The name a person
gives is kept in `dav_passwords` and is unique among that person's live app passwords (a duplicate is `409`);
the `api_tokens` row is named `dav-HANDLE-NAME` with a short random suffix when that is taken, which is what an
admin sees in `tix token list`.

### What it may do

The token's scopes are not its authority. On every request the DAV reader is built the way the feed builds its
reader (`feedReader`): the owner's current membership role, cut down to a ceiling. Phase 1's ceiling is
`task:read`, `project:read`, `workflow:read`. Phase 2's adds `task:write` and `task:transition`, unless the
password was created `read_only`. A demotion to viewer therefore turns a phone read-only on its next request,
removal from the tenant or a disabled account stops it, and the password can never do more than its owner.
`task:delete` is never in the ceiling (see Deleting).

This is how item 8 of the tix-mobile contract is answered: a new password does not freeze the caller's write
scope at creation, it follows the owner's role, and `read_only` is the explicit flag.

### Basic authentication

- Every `401` under `/dav/` and on `/.well-known/caldav` carries `WWW-Authenticate: Basic realm="tix",
  charset="UTF-8"`. Without the challenge iOS and Thunderbird never send credentials.
- The username is the owner's sign-in email, compared case-insensitively. The actor's handle in the tenant is
  accepted too, since some people will type it. A valid password with a username naming somebody else is
  refused (`dav_user_mismatch`); it protects nothing cryptographically, but it stops a password pasted into
  the wrong account from silently working as someone else.
- The password must be a `caldav` token. An account password, a session cookie, a bearer header, an ordinary
  `tix_pat_` token in the password field, and a `caldav` token sent as a bearer are all refused with
  `dav_password_required`. The account password is never passed to the password hasher: the check is the
  token's shape first, so a guessed account password costs a hash lookup, not an Argon2 run.
- People who only sign in through SSO have no account password; an app password is exactly the credential
  they can create, from a session they got through the provider.
- The tenant's second-factor policy exempts API tokens, and an app password is one. The person proved their
  factor when they signed in to create it.

### Brute force and lockout

There is no HTTP rate limiter in tix today (ROADMAP, "Rate limiting and security headers"), and this change
does not add one. It does not need to for guessing: an app password carries 256 bits from `crypto/rand`, and
the account password is not accepted at all, so there is no password to guess on this surface. What remains
is noise, and the refusal log already deduplicates repeats per client. A stale phone retrying a revoked
password produces `token_revoked` records and nothing else: it does not lock the person's sign-in, their SSO,
their other tokens or their second factor, because none of those reads the refusal log (tix-mobile item 6).
When the HTTP limiter on the roadmap lands, `/dav/` is one more path it covers, keyed by source address and not
by account, so a stale client can never lock its owner out.

## Collections

### Which ones

The home holds:

- `assigned`, **Assigned to me**: tasks assigned to the person, in any project they can read.
- `p-PROJECT`, one per project the person can read, named after the project, coloured with the project's
  colour (`apple:calendar-color`), holding that project's tasks.

The question was whether to offer both. Projects alone make a phone show every project as a Reminders list,
most of them full of other people's and agents' work; Assigned to me alone loses the "everything in infra"
view that a desktop client in Thunderbird wants. Both at once puts every task assigned to you in two lists,
and Apple Reminders has no way to hide one list of an account.

So each app password chooses, when it is created: `assigned` (the default), `projects`, or `all`. A phone
gets its owner's own work, a desktop gets the projects. With `all` a task assigned to you appears in both
lists, which the person asked for, and the docs say so. The choice is stored in `dav_passwords`, keyed by the
token, removed with it. Changing it later means a new password, since clients cache the collection list.

### What a collection holds

A task is a member when it is not in the trash, its state is not in the `cancelled` category, and it is
either open (todo, in progress, blocked, waiting) or done and completed within `caldav.completed_window`
(default 14 days). Finished work stays long enough for the client to show it as ticked, then leaves.
Cancelled work leaves at once; that is what lets `DELETE` mean cancel (below).

A collection holds at most `caldav.max_objects` tasks (default 2000, the feed's bound). Past it, the
collection holds the first 2000 by open before done, then due date, then most recently updated, and its
`calendar-description` says how many were left out. That keeps every report bounded by a known number.

## Mapping

| VTODO | From tix | Notes |
| --- | --- | --- |
| `UID` | `task-<id>@tix`, the feed's UID | a client-created task keeps the client's UID |
| `SUMMARY` | the title, alone | no `REF:` prefix as in the feed, so an edit round-trips |
| `DESCRIPTION` | the body, as written | Markdown is left as text; nothing is appended |
| `STATUS` | `NEEDS-ACTION` todo, blocked, waiting; `IN-PROCESS` in progress; `COMPLETED` done | never `CANCELLED`, see below |
| `COMPLETED` | `completed_at`, UTC | with `PERCENT-COMPLETE:100` |
| `DUE` | the due date | see Dates |
| `DTSTART` | the snooze, while it is in the future and before `DUE` | tasks.org and Thunderbird read it as "hide until" |
| `PRIORITY` | highest 1, high 3, normal 0, low 7, lowest 9 | see Priority |
| `CATEGORIES` | the task's tags | the contract said labels; tix calls them tags |
| `RELATED-TO;RELTYPE=PARENT` | the parent's UID | only when the parent is readable |
| `URL` | the task's page, absolute, from `Origin` | |
| `X-TIX-REF` | `INFRA-12` | also `X-TIX-STATE` with the state's key, for clients that show X- props |
| `CREATED`, `LAST-MODIFIED`, `DTSTAMP` | created and updated times | |
| `SEQUENCE` | the task's version | |
| `CLASS` | not written | |

`RRULE` is never written. tix spawns the next occurrence when a repeating task is finished; a client given an
`RRULE` would roll the task forward itself, and the two would disagree. The next occurrence simply appears on
the next sync. A repeating task's `DESCRIPTION` is not changed to say so, because the description round-trips.

### Status

`CANCELLED` is accepted from a client in phase 2 and never written. This is a change to the shared contract,
which listed it among the values written. A cancelled task leaves its collection instead, because Apple
Reminders has no cancelled state and would show it as an open reminder, and because it is what lets a `DELETE`
from any client be a cancel that does not come back on the next sync. Blocked and waiting have no VTODO
counterpart and read as `NEEDS-ACTION`, with `X-TIX-STATE` naming the real state.

### Dates

A due date is an instant, and a bare date is stored as midnight UTC on every surface, as `docs/calendar.md`
says. A due time of exactly midnight UTC is written as `DUE;VALUE=DATE:20261105`, which every client shows as
that day in every zone, the same rule the feed applies. Any other due time is written as a UTC date-time,
`DUE:20261105T143000Z`, which clients show at the right local time. This differs from the feed, which always
writes a date: a reminder can carry a time and an all-day event cannot carry one usefully. When `DUE` is a date,
`DTSTART` is written as a date too, as RFC 5545 requires of the pair.

Reading, in phase 2: `VALUE=DATE` is midnight UTC; a UTC time is itself; a `TZID` is resolved with the zone
database the binary already embeds (`time/tzdata`), and a `TZID` the database does not know falls back to the
`VTIMEZONE` offsets in the same object, then to the person's own time zone preference; a floating time is read
in the person's time zone preference, then the tenant's, then UTC.

### Priority

iCalendar's priority runs 1 (highest) to 9, with 0 for undefined, and Apple Reminders and Thunderbird write 1,
5 and 9 for high, medium and low. tix writes normal as 0, so a normal task shows no priority marker in
Reminders instead of `!!` on every row. Read back, 0 is normal, 1 and 2 highest, 3 and 4 high, 5 normal, 6 and
7 low, 8 and 9 lowest. Everything tix writes reads back to itself. Apple's "High" (1) reads as highest and
its "Low" (9) as lowest, which the docs say.

## ETags, getctag and sync tokens

An object's ETag is a hash of the task's identifier, its version and update time, the collection's rendering
inputs (the project key, the workflow's state-to-category map, the origin the URLs are built from), and in
phase 2 the hash of what is kept for the person in `dav_objects`. It is computed without rendering, so a
`PROPFIND` of ETags costs a query and no iCalendar.

The tasks' event log was the first candidate for sync tokens, as the shared contract suggested: the token is the
highest event sequence the client has seen, and changes are the task events after it. It does not hold up
against what a collection is:

- A task leaves a collection with no event about it: its completion ages out of the window, it is pushed past
  `max_objects` by another task, or its state's category changes because somebody edited the workflow.
- A project rename or key change changes every object (`X-TIX-REF`, `URL`) through one project event.
- A role change changes what the person can read with no task event at all.
- Events are pruned by retention.

So the sync token is a **snapshot**: the set of `(name, etag)` pairs the collection held when the token was
issued, stored in `dav_sync_states` keyed by the token's hash. A `sync-collection` report computes the current
set, which is bounded by `max_objects`, and answers the difference: changed and new members with their ETags,
departed members as `404`. This is correct for every way a task can enter or leave, and it is how Radicale does
it. The event log is still used, as a short cut: the snapshot records the tenant's latest event sequence and
the UTC day, and when neither has moved and the configuration is unchanged, the answer is "no changes" without
listing anything. Any event in the tenant spends the short cut, which is conservative and never wrong.

- A snapshot is written only when the set changed. Identical sets share one row, so a client polling an
  unchanged collection writes nothing.
- The last three snapshots per password and collection are kept, plus any younger than a day; a token older
  than that answers `403` with `DAV:valid-sync-token`, and every client then does a full sync, which is what
  RFC 6578 expects.
- A `limit` smaller than the difference is honoured by answering part of it with a token for the snapshot that
  part leads to, followed by `507` on the request URI, as RFC 6578 section 3.6 describes.
- `getctag` (`http://calendarserver.org/ns/`) and `DAV:sync-token` are the same value, for clients that poll the
  tag and then list ETags (Thunderbird, Evolution).
- A sync token is a URI, `https://tix.red/ns/sync/HEX`, as RFC 6578 requires.

`dav_sync_states` is a cache of what was answered, not domain state: writing it records no audit entry and no
event, the same as `last_used_at` and idempotency keys. That is called out because AGENTS.md asks every
mutation for both.

## Phase 1: read-only

Served: `OPTIONS` (`DAV: 1, 3, access-control, calendar-access`), `PROPFIND` at depth 0 and 1, `GET` and `HEAD`
of an object, `REPORT` `calendar-query`, `calendar-multiget` and `sync-collection`.

- `current-user-privilege-set` is `DAV:read` alone. Apple Reminders, DAVx5 and Thunderbird all show such a list
  as read-only, so a person cannot tick something on the phone that will be refused.
- `PUT`, `DELETE`, `PROPPATCH`, `MKCOL`, `MKCALENDAR`, `MOVE`, `COPY`, `LOCK` and `POST` are `403`, with
  `DAV:need-privileges` in the body where one applies. `PROPPATCH` answers `207` with `403` per property, as RFC
  4918 asks, because Apple sends one for colour and order and treats a bare error as a broken account.
- `calendar-query` is evaluated in memory over the collection's members, which `max_objects` bounds: comp,
  prop and param filters, `is-not-defined`, `text-match` with `i;ascii-casemap` and `i;octet`, and `time-range`
  on `VTODO` by RFC 4791 section 9.9. An unsupported collation is `403` with `CALDAV:supported-collation`.
- `calendar-multiget` takes at most 1000 hrefs; more is `403` with `DAV:number-of-matches-within-limits`.
- Unknown properties in a `PROPFIND` answer `404` in their own `propstat`, as every client expects.

Phase 1 is shippable on its own: it needs the scope, the password operations, `dav_passwords`,
`dav_sync_states`, the renderer and the handler, and no write path.

## Phase 2: write-back

### Updating

`PUT` to an existing object requires `If-Match`; without it the answer is `428`, and with a stale ETag `412`.
The ETag check and the task's version are the same thing, so the service is called with `Version` set and a
concurrent change from the browser is a conflict, not an overwrite. The VTODO is read and compared with what
tix would render for the current task, and only the differences are applied:

- `SUMMARY` changes the title; `DESCRIPTION` the body; `DUE` the due date, and its absence clears it, because
  every client removes the property to remove a date; `PRIORITY` the priority; `CATEGORIES` the tags.
- `STATUS` and `COMPLETED` move the task. `COMPLETED` moves it to the state of the `done` category its workflow
  reaches, `IN-PROCESS` to `in_progress`, `NEEDS-ACTION` from a done task back to `todo`, and `CANCELLED` to
  `cancelled`. The move uses the multi-hop route, so a workflow that needs two steps works and every guard
  runs. A move the workflow refuses, one needing a comment, an approval or a claim, or one with two equally
  short routes, is `403` with a plain text body naming why, and the task is unchanged.
- `DTSTART` in the future sets the snooze to it; a past or absent one changes nothing, because Thunderbird
  writes the creation time there.
- A property that is absent leaves the field alone, except `DUE` and `STATUS`/`COMPLETED`. Apple Reminders
  rewrites the whole object and drops `CATEGORIES`, so treating absence as "clear" would wipe tags from every
  task touched on an iPhone.

The answer carries no `ETag`, because what is stored is never byte-for-byte what was sent (tix adds `URL` and
`X-TIX-REF`). RFC 4791 then has the client fetch the object again, which all four clients do.

### Creating

`PUT` to a new name with `If-None-Match: *` creates a task with `CreateTask`, in the project of a project
collection, or, in Assigned to me, in the tenant's default project and assigned to the person. The client's
UID and resource name are kept in `dav_objects`, unique per tenant, because CalDAV clients identify objects
by both and must find them again; a UID already in use is `403` with `CALDAV:no-uid-conflict`. A `PUT`
without `If-None-Match` to a name that does not exist is treated as a create too, since Evolution sends none.

### Deleting

`DELETE` never removes a task and never moves one to the trash. Trashing needs `task:delete`, which members do
not hold, and a tap on a phone should not be the one way work is lost without anyone deciding it.

- An open task is moved to its workflow's `cancelled` state by the same route as `STATUS:CANCELLED`, which takes
  it out of the collection. A workflow with no reachable cancelled state answers `403` and the task stays.
- A done task is left alone and the answer is `204`. Apple Reminders' "Clear completed" sends a `DELETE` per
  finished reminder, and refusing them shows the person an error per task. The task stays a member until its
  window passes; a client syncing by token will not see it again unless it changes, and a client that relists
  by ETag (Thunderbird) may show it again until it ages out. The docs say so.

This is a change from the open question in the shared contract ("cancel vs delete; decide"): cancel.

### What tix cannot store

Alarms (`VALARM`), `LOCATION`, `GEO`, `ATTACH`, `ATTENDEE`, `ORGANIZER`, `CLASS`, `PERCENT-COMPLETE` other than
100, and client extensions (`X-APPLE-SORT-ORDER`, `X-MOZ-*`, `X-OC-*`) have no tix field. Dropping them would
make an alarm set on an iPhone vanish on the next sync, which looks like the server losing data. So phase 2 keeps
them, verbatim, in `dav_objects` per person and per task, at most 8 KiB, and writes them back only into that
person's own copy of the object. They are never shown in tix, never interpreted, never reach another person,
and are removed with the task, the person or the tenant. Over 8 KiB, the object is still accepted and the
extras are dropped, with a log record. This keeps alarms working without "VALARM write-back", which stays a
non-goal: tix does not learn about reminders on a phone.

The exceptions are recurrence (`RRULE`, `RDATE`, `EXDATE`, `RECURRENCE-ID`), which is dropped and never kept, for
the reason in Mapping, and `RELATED-TO`, which is ignored on the way in: reparenting from a client is a follow-up.

`PUT` bodies are capped at 256 KiB, one `VCALENDAR` holding exactly one `VTODO`; anything else is `415` or `403`
with `CALDAV:valid-calendar-object-resource`.

## Security

- **XML.** `encoding/xml` resolves no external entities, so there is no XXE in the usual sense, but it does not
  bound a document either. Request bodies under `/dav/` are capped at 64 KiB for `PROPFIND` and `REPORT`
  (smaller than the API's 1 MiB, enough for any client's largest multiget), the decoder walks tokens and refuses
  any `<!DOCTYPE`, directive or entity declaration, nesting deeper than 32, and more than 2000 elements. A body
  that breaks a rule is `400`.
- **Report cost.** Every report is bounded by `max_objects` and multiget by 1000 hrefs; `PROPFIND` with
  `Depth: infinity` is `403` with `DAV:propfind-finite-depth`; the router's request timeout applies.
- **Enumeration.** Another tenant's key, another person's principal, a project the reader cannot read and a
  task outside the collection all answer the same `404` as a name that never existed.
- **CSRF.** A browser that once had Basic credentials typed into it for the origin sends them on cross-site
  requests. `/dav/` accepts no `POST`, sends no CORS headers, and every write method is one a browser preflights,
  so another site cannot drive it. `GET` is read-only and its answer is unreadable cross-origin.
- **Secrets in logs.** The Authorization header is never logged, as today; the path carries no secret.
- **Plain HTTP.** Basic sends the password on every request. `tix serve` already refuses a non-loopback bind
  without TLS; the docs say CalDAV must sit behind it, and iOS refuses plain HTTP accounts on its own.

## Client quirks

- **Apple Reminders (iOS, macOS).** Needs the Basic challenge, `/.well-known/caldav` and
  `calendar-user-address-set` (a `mailto:`). Shows only collections whose component set includes `VTODO`, and
  shows VTODO-only ones in Reminders, not Calendar. Uses `sync-collection` when `supported-report-set` lists it,
  and `getctag` otherwise. Rewrites whole objects and drops properties it does not use, hence "absent means
  unchanged". Sends `PROPPATCH` for colour and order. CalDAV lists get no tags, subtasks or sections in
  Reminders.
- **Thunderbird.** Discovers from the server root since 102. Polls `getctag`, then lists ETags and multigets;
  uses sync tokens when offered. Writes `X-MOZ-GENERATION` and `X-MOZ-LASTACK` on every change, which the
  extras keep per person. Offers the person overwrite or discard on `412`.
- **DAVx5 with tasks.org, jtx Board or OpenTasks.** Discovers through the well-known URL; respects read-only
  privileges; uses `sync-collection`; re-downloads after a `PUT` without an ETag; ical4j rejects a `DTSTART`
  after `DUE` and a `DUE` of a different value type from `DTSTART`, which the mapping avoids. tasks.org writes
  subtasks as `RELATED-TO` and hides tasks until `DTSTART`.
- **Evolution.** Uses `getctag` and multiget; may `PUT` without `If-None-Match` for a new object.

The tests replay a captured request sequence from each, from discovery to a full and an incremental sync, and in
phase 2 an edit, a completion, a create and a delete.

## Contract with tix-mobile

The tix-mobile plan (heliopsy/tix-mobile draft PR #163, `openspec/changes/tasks-in-other-apps/`) relies on the
following. Each item is confirmed, adjusted or rejected.

1. **Confirmed.** CalDAV under `{server}/dav/`, `/.well-known/caldav` redirecting there, `current-user-principal`
   and `calendar-home-set`. Every collection's `supported-calendar-component-set` is exactly `VTODO`.
2. **Confirmed, with one field added.** `dav.info` is `GET /api/v1/dav`, authenticated with a session cookie or
   a bearer token, answering `enabled`, `url` (absolute, built by `Origin`, so it honours `server.public_url` and
   trusted proxies), `username` (the exact Basic username: the caller's sign-in email), `write` (false in phase 1,
   and false for a viewer in phase 2), and `reason` when disabled. Added: `collections`, the choices a password can
   be created with. A server without this change answers `404`.
3. **Confirmed, with additions.** `dav.password.list`, `dav.password.create` and `dav.password.revoke` at
   `/api/v1/dav/passwords` and `/api/v1/dav/passwords/{ID}`. No `token:admin`; they need `task:read` and a person
   caller, and work from SSO and password sessions and from personal API tokens; an agent actor is `403`. Create
   takes `name`, and optionally `collections` (`assigned` by default, `projects`, `all`), `read_only` and
   `expires_at` (default: never, unlike `tix token create`, because a phone account that stops after ninety days
   with no prompt is a support call). It answers `201` with `id`, `name`, `password` (once), `username`, `url`,
   `scopes` (`["caldav"]`), `collections`, `read_only`, `created_at`, `expires_at`. List answers metadata only:
   `id`, `name`, `scopes`, `collections`, `read_only`, `created_at`, `expires_at`, `last_used_at`, `revoked_at`.
   A duplicate live name is `409`. The `token.create` alternative is rejected: `caldav` is refused there.
4. **Adjusted.** Creating needs `task:read` in both phases. Write authority is not decided when the password is
   created: it is the owner's role at request time, cut to the phase's ceiling, so phase 2 needs no new password.
5. **Confirmed.** `caldav` works only under `/dav/` and is refused on `/api/v1/*` (`caldav_only`). Sessions,
   bearer tokens, account passwords and ordinary API tokens are refused on `/dav/` (`dav_password_required`).
6. **Confirmed.** Failed Basic authentication is logged and nothing else; no lockout exists on this surface and
   none is added.
7. **Confirmed.** Revocation takes effect on the next request; `revoked_at` is in the list.
8. **Adjusted.** Writes go through `UpdateTask`, `CreateTask` and the multi-hop move: the version increases and the
   ordinary events are written. Passwords do not inherit the caller's scopes; see 4. `read_only` is the explicit
   flag.
9. **Confirmed.** Operation identifiers `dav.info`, `dav.password.list`, `dav.password.create`,
   `dav.password.revoke` in `api/openapi.yaml`.
10. **Confirmed.** `docs/caldav.md` ships with phase 1, with the anchors `#apple-reminders`, `#davx5-and-tasksorg`,
    `#thunderbird` and `#evolution`, plus `#app-passwords` and `#troubleshooting`.
11. **Confirmed.** The first release containing phase 1, which as a `feat` is the next minor after it lands
    (0.69.0 at the time of writing, if nothing else bumps it first). Phase 2 is a later minor. `dav.info` lets the
    app ask instead of comparing versions.
12. **Confirmed.** Creating and revoking an app password write the token's existing events and audit entries
    (`token.created`, `token.revoked`), with `caldav` in the scopes, so a webhook or the event stream sees them.

### Other changes to the shared contract

- Collections: Assigned to me and one per project, chosen per password (the contract left it open).
- `STATUS:CANCELLED` is read but never written; cancelled tasks leave the collection.
- `DELETE` cancels an open task and is a no-op on a done one; nothing is trashed.
- Sync tokens are snapshots rather than event sequences.
- Unmappable properties are kept per person in phase 2 rather than dropped.
- The operations are named `dav.*` and live at `/api/v1/dav`, to match the tix-mobile plan; the CLI is
  `tix dav`.
- The username is the sign-in email; the handle is accepted too.

## Open questions

- Whether a tenant admin should be able to turn CalDAV off for one tenant. This change has the deployment key
  only.
- WebDAV-Push and Apple's push, so phones stop polling. Both need a sender per platform, like notifications.
- Mapping `RRULE` to tix's repeat rules, and `RELATED-TO` writes to reparenting.
