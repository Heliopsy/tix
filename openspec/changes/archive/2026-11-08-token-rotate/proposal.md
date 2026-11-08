# Rotate an API token

## Why

Replacing a token today is two steps: issue a new one, then revoke the old one. The new one has to be
issued under a different name, because a live name is unique, and its scopes, project and expiry retyped by
hand. For an agent's token that is also downtime: the moment the old token is revoked the agent stops
working until somebody hands it the new value. The owner asked for a rotate action alongside revoke.

## What Changes

- New service method `RotateToken(ctx, id, in)`. It mints a token for the same actor with the same name,
  scopes and project, returns its value once, and retires the old one in the same transaction.
- The new token gets the old one's lifetime counted from now: a token that never expired still does not,
  and a 90 day token is good for another 90 days.
- An optional grace period, `0` (immediate, the default) up to `24h`, during which the old token still
  authenticates. It is stored as a future `revoked_at` on the old token, so there is no background job and
  revoking it during the window still cuts it at once.
- Authorization: the token's own actor may rotate it on the scopes it holds, so an agent can rotate its own
  token without `token:admin`; anybody else needs `token:admin`. The new token may never carry a scope the
  rotator does not hold, with the same `global:admin` and `account:read` rules as issuing one.
- Audit `token.rotate` (or `token.rotate_other` for somebody else's token) naming the old and new token ids,
  never a secret, and the `token.rotated` event.
- Every surface: `tix token rotate ID|NAME [--grace D] [--actor A]`, `POST /api/v1/tokens/{id}/rotate`
  (and so an MCP tool), a Rotate button beside Revoke on the Tokens screen, `R` on the terminal account view.
  SSH binds the command.
- Disabling a user now also ends a token still inside its grace window.

## Impact

- `internal/core`: `RotateTokenInput`, `MaxTokenRotateGrace`, `APIToken.Active` honours a future
  `revoked_at`, `APIToken.Retiring`.
- `internal/authz`: `Policy.CanRotateToken`.
- `internal/service`, `internal/client`, `internal/httpapi`, `internal/web`, `internal/tui`, `cmd`.
- Store: `RevokeActorTokens` also ends tokens whose revocation is still in the future. No migration.
- Docs, skill and `api/openapi.yaml`.
