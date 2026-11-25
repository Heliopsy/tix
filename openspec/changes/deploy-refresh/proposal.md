# Pick up a new release in an open tab

## Why

The assets are served at unversioned paths, so a tab left open across a deploy keeps running the old
scripts against the new server's fragments until someone reloads it by hand. Nobody knows they should,
and the mismatch shows up as odd behaviour rather than as a message.

## What Changes

- Every page carries the release that drew it on `<body data-build="...">`.
- A new script, `assets/deploy.js`, compares that with the `version` field of `GET /healthz` when the
  live socket reconnects after a drop, when the tab becomes visible again, and every seven minutes
  while it is visible.
- When the versions differ and nothing on the page would be lost, the tab reloads itself, keeps its
  scroll position and then shows "Updated to vX" for a few seconds.
- When something would be lost (focus in a field, an edited field, an open dialog or popover, a
  request in flight, rows selected), a non-blocking bar says a new version is available and offers
  Reload. The safe check runs again when the reader leaves a field, submits, closes a dialog or a
  request settles, and reloads as soon as nothing is left to lose.
- Bulgarian strings for the bar and the note.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: tabs follow a server upgrade.

## Impact

- `internal/web/templates/layout.html`, `internal/web/render.go`, `internal/web/assets/deploy.js`,
  `internal/web/assets/live.js`, a new stylesheet part, `internal/i18n`.
- No new route: `/healthz` already answers the version without a session or a database read.
