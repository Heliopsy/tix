# Audit records the access channel

## Why

An audit entry says who changed what and from which surface (`cli`, `api`, `web`, `tui`,
`system`). That is not enough to answer the question an admin actually has after something goes
wrong: did Tom do that in the browser or over SSH, was it the CI token or the MCP agent's token,
was it a local CLI against the database or the CLI through the server. The surface collapses every
token into `api` and every terminal session into `tui`, and says nothing about the address or the
key.

## What Changes

- Every audit entry and every event records an access channel, `via`, set once by the entry point
  the request came through and carried in the context. Kinds: `web`, `api`, `cli-local`,
  `cli-remote`, `ssh`, `mcp`, `sync`, `import`, `system`, and `unknown` for rows written before
  this change.
- An audit entry's channel also carries what identified the caller on it: the token's ID and name
  (never the secret), an opaque reference to the browser session (never the cookie), the SSH key
  fingerprint and client address, the client address (through the trusted proxy policy) and a
  truncated user agent for HTTP, the job name for background work, and the channel that started a
  sync run or an import. A local CLI records the host name and OS user only when
  `cli.record_host` is on.
- Events carry only the kind, the token and the job. Addresses, user agents, sessions and keys stay
  in the audit log.
- The remote CLI and `tix mcp` send `X-Tix-Client: cli|mcp`. The server uses it only to refine a
  token request from `api` to `cli-remote` or `mcp`. It cannot claim web, ssh or system.
- Migration 0033 adds `via_kind`, `via_token` and `via_detail` to `audit_entries` and `via_kind`,
  `via_detail` to `events`, with `via_kind` defaulting to `unknown`.
- Filters: `GET /api/v1/audit` and the task audit route take `via` and `token`; `tix audit ls`
  takes `--via` and `--token`; the activity filter language takes `via:` and `token:`.
- Display: the Activity feed and a task's History show the channel as an icon and a few words
  ("via SSH", "via token ci-agent", "via MCP (token scout-runner)") with the details in the hover
  and the raw view. Activity gets a channel dropdown and a token box in place of the surface
  dropdown. `tix audit ls` gets a VIA column and the terminal history line names the channel.

## Impact

- `internal/core` (`Via`, `ViaKind`, `AuditEntry.Via`, `Event.Via`, `AuditFilter.Vias/Tokens`,
  `Actor.TokenName/SessionID`), both store engines, `internal/service`, `internal/httpapi`,
  `internal/connect`, `internal/client`, `internal/sshd`, `internal/server`, `internal/web`,
  `internal/query`, `internal/tui`, `cmd`, `api/openapi.yaml`, docs.
- Migration 0033 follows 0030 to 0032, which other open changes reserve.
