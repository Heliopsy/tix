# Listing and revoking your own sessions

## Why

A sign-in leaves a session on the server until it expires, and nothing shows a person which ones they hold.
Someone who signed in on a borrowed machine, or lost a laptop with a browser still signed in, has no way to
see that session or end it short of waiting out its expiry or asking an administrator to disable them.

## What Changes

- `ListSessions` (`session.list`, `GET /api/v1/account/sessions`) lists the caller's own live sessions in
  this tenant, newest first, keyset paginated: identifier, when it began and expires, how it signed in,
  whether a second factor passed, the user agent it signed in with, and which one is the caller's own.
  Never the token or its hash.
- `RevokeSession` (`session.revoke`, `DELETE /api/v1/account/sessions/{id}`) ends one of them. Another
  person's session identifier is not found.
- A token needs `account:read` for either, as for the account log; a session may always.
- New sessions record the user agent of the sign-in request.
- `tix account sessions` and `tix account sessions revoke ID`; a Sessions section on the web settings
  screen with a red revoke button per session; a sessions section in the terminal account view.
- `tix mcp` hides both operations.

## Capabilities

### Modified Capabilities

- `auth`: a person can list and revoke their own sessions.
- `web-ui`: the settings screen has a Sessions section.

## Impact

`internal/core`, `internal/authz`, `internal/service`, `internal/store` (both engines), `internal/httpapi`,
`internal/client`, `internal/capability`, `cmd/account.go`, `internal/web` (settings, i18n),
`internal/tui/account.go`, `internal/mcp/overlay.yaml`, `api/openapi.yaml`, `docs/api.md`, generated CLI
docs, the tix skill.
