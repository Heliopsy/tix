# Security Policy

## Reporting a vulnerability

Report security issues privately through
[GitHub Security Advisories](https://github.com/heliopsy/tix/security/advisories/new).

Please do not open a public issue for a vulnerability.

Include what you can: affected version or commit, a description of the issue, reproduction
steps, and the impact you believe it has. You will get an acknowledgement within a few days
and an assessment once the report has been reviewed.

## Supported versions

Until v1.0.0, only the latest release receives security fixes.

## Scope

tix stores API tokens, session tokens, and password hashes, and exposes an HTTP API and a
web UI. Findings of particular interest:

- **Cross-tenant data access.** Any path where one tenant can read or write another
  tenant's data is the highest-severity class in this project.
- Authentication or authorization bypass, including scope escalation with an API token.
- Lease or claim handling that lets a worker act on a task it does not hold.
- Token, session, or webhook secret disclosure, including through logs, the audit log, or
  exported snapshots.
- Injection of any kind, including into generated SQL.
- Webhook signature forgery or replay.
- Server-side request forgery: a tenant-supplied url (webhook, inbox channel, sync source)
  reaching an address the egress guard should refuse.

## Deployment notes

- The server binds loopback by default and refuses a non-loopback bind without TLS or an
  explicit opt-out. Do not use that opt-out on an untrusted network.
- Terminate TLS at the server or at a reverse proxy. Do not expose plaintext HTTP publicly.
- A SQLite database on a network filesystem can corrupt. Use PostgreSQL for shared
  deployments; `tix doctor` reports this.
- Webhook endpoints, inbox channels and sync sources on `tix serve` never reach loopback,
  link-local (cloud metadata) or private addresses unless `server.egress_allow` or
  `server.egress_allow_private` says so. Keep the allow list narrow on a shared host. See
  [docs/deployment.md](docs/deployment.md#outbound-requests-to-tenant-urls).
- Passwords are hashed with argon2id at m=64 MiB, t=3, p=4, well above OWASP's minimum, and at most
  `auth.max_concurrent_hashes` derivations run at once so sign-ins cannot exhaust memory. A sign-in that
  cannot get a slot is refused, never admitted unchecked. See
  [docs/deployment.md](docs/deployment.md#password-hashing-memory).
- API tokens are shown once and stored hashed. Rotate them with `tix token revoke`.
- Every audit entry records the access channel: the token's ID and name, an opaque session
  reference, the SSH key fingerprint, the client address and a truncated user agent. Any
  member with `audit:read` can see them. The token secret and the session cookie are never
  stored. Set `server.trusted_proxies` so the address is the client's and not the proxy's,
  and leave `cli.record_host` off unless the team wants host names in the log. See
  [docs/audit-log.md](docs/audit-log.md).
