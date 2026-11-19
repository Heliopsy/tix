# Say why authentication was refused, and who it refused

## Why

A client on the LAN kept getting `401` on `GET /api/v1/projects` and `GET /api/v1/workflows`. The only
record was the request line, `method`, `path`, `status`, `duration_ms` and `tenant`. It did not say which
token was presented, why it was refused, or which client sent it: the proxy knew the address, tix did not log
it, and the request carried no User-Agent. Every refusal reads `invalid credentials` to the client on purpose,
so the server's log is the only place the reason can be found, and it was not there.

## What Changes

- The request line gains `client_ip`, resolved through `server.trusted_proxies` as everything else in tix
  resolves it, and `user_agent`, truncated to 160 bytes. A refused request also carries `auth_reason`.
- A refused credential or sign-in writes one `auth refused` record at `warn` with a stable `reason` code, the
  client address, user agent and access channel, and a safe name for the credential: a token's id and name
  once it is found, a `t-` reference derived from the stored hash for a well formed token that matched
  nothing, the `s-` session reference the audit log already uses for a browser session.
- The record is written at most once a minute per client address, reason and token, and the next one carries
  `suppressed` with the number held back.
- Refusals of an authenticated caller by the policy (`scope_denied`, `project_denied`, `require_claim`,
  `second_factor_required`) are logged when the request was answered `403`.
- The SSH listener's existing refusal lines gain the same `reason` field.
- What the client receives does not change.

## Capabilities

### Modified Capabilities

- `server`: the request line carries the client address and user agent, and refused authentication is
  logged with a reason.

## Impact

`internal/auth`, `internal/httpapi`, `internal/logging`, `internal/server`, `internal/service`,
`internal/sshd`, `docs/deployment.md`, `docs/configuration.md`.
