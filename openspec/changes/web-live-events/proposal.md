## Why

The app has a screen that follows the tenant's raw event stream (being renamed Live events at `/events`),
the CLI has `tix watch` and the TUI its live tail. The browser had only Activity, the audit log, so there
was no way to watch events arrive, filter them by type or replay from a sequence number without a terminal.

## What Changes

- New web screen `/events`, "Live events", under the Manage sidebar group with Advanced screens on, and in
  the command palette.
- Events arrive over the existing WebSocket with the session cookie, newest first, at most 500 kept.
- Pause and Resume with a count of what arrived meanwhile, Clear the list, a visible connection state.
- Filters in the query string: types and project sent to the server, actors matched in the browser, and a
  replay point; an expired cursor is reported and the stream carries on from now.
- Each row opens its task or project and shows the event as JSON with Copy JSON.

## Impact

- `internal/web` (liveevents.go, events.html, assets/events.js, a CSS part, sidebar, palette, help),
  `internal/i18n`, `docs/web-ui.md`, README. No API, service or CLI change.
