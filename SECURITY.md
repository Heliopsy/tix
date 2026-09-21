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

## Deployment notes

- The server binds loopback by default and refuses a non-loopback bind without TLS or an
  explicit opt-out. Do not use that opt-out on an untrusted network.
- Terminate TLS at the server or at a reverse proxy. Do not expose plaintext HTTP publicly.
- A SQLite database on a network filesystem can corrupt. Use PostgreSQL for shared
  deployments; `tix doctor` reports this.
- API tokens are shown once and stored hashed. Rotate them with `tix token revoke`.
