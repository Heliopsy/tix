# Date preferences follow the person

## Why

A date format and a timezone chosen in the browser lived in two cookies, so they stayed in the one
browser they were picked in. Every other browser, the phone, the terminal interface and the command
line kept showing the deployment's format, which read as "these fields ignore my preference" (the API
token, SSH key, sync and activity screens were the ones people noticed). A tenant had no way to set a
default for its people, and a relative time such as "2 hours ago" could not be pinned down to an
instant on a touch screen at all.

## What Changes

- A date format and a timezone resolve in three levels, each of which can inherit: the deployment
  (`output.time_format`, `output.timezone`), the tenant (set by a tenant administrator on the Tenant
  screen, `tix tenant edit --time-format --timezone`, or `time_format`/`timezone` on
  `PATCH /tenants/REF`), and the person (Settings, `tix user time`, `PATCH /me/time`). The format and the
  zone resolve separately, and every screen that offers one says what is in use and where it came from.
- The person's choice is stored on their user, so it follows them to every browser, device and tenant.
  The two cookies are read once: a value the account does not already hold is moved onto it, and both
  cookies are expired.
- A format or zone set on a machine (a flag, `TIX_OUTPUT_*` or the configuration file) still wins on
  that machine for the command line and the terminal interface. Everything left unset there follows the
  account.
- Every timestamp a web screen shows goes through one template helper, which writes a `<time>` whose
  `datetime` is RFC 3339 and whose title, accessible label and focus tip carry the full instant with
  seconds, the zone and its offset, and how long ago it was.
- A guard renders every screen on a seeded database as a reader in a distinctive format and zone and
  fails on any timestamp in another layout, written as plain text, or missing its tooltip.
- JSON, YAML, NDJSON, CSV, the HTTP API and MCP keep RFC 3339 in UTC.

## Impact

- Migration 0032 adds `time_format` and `timezone` to `tenants` and `users`, defaulting to empty, which
  inherits, so nothing anybody reads changes on upgrade.
- New operations `GetTimeSettings` and `SetTimePrefs`: `GET`/`PATCH /api/v1/me/time`, `tix user time`,
  the web settings screen, and two MCP tools generated from them.
- The tokens screen gains a Last used column.
