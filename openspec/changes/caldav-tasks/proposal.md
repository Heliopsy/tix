# Tasks over CalDAV

## Why

The calendar feed puts due dates in Google Calendar, Apple Calendar and Outlook, but as events, because a
subscribed feed is the one place none of them shows a `VTODO`. The apps people actually keep to-do lists in,
Apple Reminders, Thunderbird's Tasks, tasks.org and jtx Board through DAVx5, and Evolution, read tasks only
over CalDAV (RFC 4791). So today a tix task can be seen on a phone's calendar and cannot be ticked off in its
to-do list.

## What Changes

- `tix serve` answers **CalDAV under `/dav/`**, with `/.well-known/caldav` redirecting there (RFC 6764) and
  the usual discovery: `current-user-principal`, `calendar-home-set`, one principal per person per tenant.
  Paths carry the tenant key, and a request can only ever reach the tenant its credential belongs to.
- A person sees **one VTODO-only calendar collection per project** they can read, and an **Assigned to me**
  collection. Each app password chooses which of the two kinds it exposes, so a phone does not have to show
  every project as a list.
- Clients sign in with **HTTP Basic**: the username is the person's sign-in email, the password is an **app
  password**, an API token carrying only the new named-only `caldav` scope. App passwords are created from
  `tix dav password create`, from Settings in the browser and from `POST /api/v1/dav/passwords`, by any
  person in the tenant, including people who only sign in through SSO. Account passwords, session cookies,
  bearer tokens and ordinary API tokens are refused on `/dav/`, and a `caldav` token is refused everywhere
  else. Every refusal goes through the refusal log.
- **Phase 1, read-only:** `OPTIONS`, `PROPFIND`, `GET`/`HEAD`, and the `calendar-query`,
  `calendar-multiget` and `sync-collection` (RFC 6578) reports, with per-object ETags, `getctag` and sync
  tokens. Phase 1 ships on its own.
- **Phase 2, write-back:** `PUT` with `If-Match` changes a task's title, description, due date, priority,
  tags and state; `PUT` with `If-None-Match: *` creates one; `DELETE` cancels an open task. A stale ETag is
  `412`. Writes go through the ordinary service methods, so authorization, audit entries and events are the
  same as from any other surface.
- `GET /api/v1/dav` tells a client (the tix-mobile app, the Settings screen) whether CalDAV is on, the
  address to use, the exact username and whether writes are accepted.
- New configuration: `caldav.enabled`, `caldav.completed_window`, `caldav.max_objects`.

Not in this change: events (`VEVENT`) over CalDAV, CardDAV, scheduling (RFC 6638), `MKCALENDAR`, sharing,
push (Apple's `push-transports`, WebDAV-Push), and mapping alarms or repeat rules onto tix fields.

## Impact

- New package `internal/caldav` (the protocol, like `internal/scim`), a VTODO renderer and a small VTODO
  reader in `internal/ical`, a `core.TaskCollections` interface beside `core.Service`.
- New scope `caldav`, three refusal reasons, six registry entries, `/api/v1/dav` routes in
  `api/openapi.yaml`.
- Migrations: `dav_passwords` (the collection choice per app password), `dav_sync_states` (sync token
  snapshots), and in phase 2 `dav_objects` (client-chosen UIDs and resource names, and the properties tix
  keeps for a client without understanding them).
- No new Go module dependency.
- Nothing changes for a deployment where nobody creates an app password, and `caldav.enabled: false` removes
  the surface entirely.
