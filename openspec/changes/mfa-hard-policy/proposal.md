# A hard second factor policy per tenant

## Why

A tenant can require a code at sign-in on each surface (`require_totp_web`, `require_totp_cli`,
`require_totp_ssh`), but the requirement is read only when a session is minted. A person with no factor keeps
working through every session minted before the flag went on, through a session minted on a surface whose flag
is off, through any API token they own, and so through MCP. An administrator who clears somebody's factor
leaves their live sessions working. The owner wants a tenant to be able to say "nobody works here without a
second factor" and have that hold everywhere.

## What Changes

- Tenants get `mfa_policy` (`off`, the default, or `required`) and `mfa_grace` (a duration, default 0).
  `required` covers every surface. The per-surface flags stay as they are: they ask for a code at sign-in and
  gate nothing else.
- While the policy is `required` and the grace has run out, the service refuses every operation to a person who
  holds no confirmed factor, as forbidden with reason `second_factor_enrollment_required`. What stays open is
  enrolling, their own account (who am I, account log, own sessions, own date settings), signing out and
  switching to another tenant.
- The refusal covers sessions and person-owned API tokens on every surface. Tokens of agent and service actors
  (no person account behind them) are exempt and documented as such. A token minted before enrolment works
  again as soon as the person enrols.
- Web: under the policy, a password sign-in by an unenrolled person mints a session and lands on an enrolment
  screen; every other screen redirects there, and htmx and POST requests get 403. During the grace a banner
  names the deadline.
- `tix login` enrols on a terminal as before; without one it is refused (exit 5) naming the web enrolment
  screen. During the grace it warns and signs in.
- SSH: an unenrolled person's interactive session gets the in-terminal enrolment and nothing else; a command
  session is refused naming `ssh -t`.
- Turning the policy on is refused while the caller is a person with no factor of their own, and without a
  sealing key.
- `tix user ls --without-2fa`, a `without_totp` filter on `GET /users`, a filter on the web Users screen and in
  the terminal people view; `tix user ls` shows each person's factor.
- Audit: policy changes (the tenant update entry with `mfa_policy` in its metadata), gate refusals (at most one
  `user.totp.gate` entry per person per hour per process) and enrolments, each with its access channel.
- Settable from `tix tenant edit --mfa-policy --mfa-grace`, `PATCH /tenants/{ref}` (so MCP), the web Tenant
  screen and the terminal tenant editor.

## Capabilities

### Modified Capabilities

- `auth`: the hard policy, the gate, the grace, the lockout guard, the user filter.
- `web-ui`: the enrolment-only session, the grace banner, the Tenant screen's policy section, the Users filter.
- `ssh-access`: the SSH gate under the policy.
- `tui`: the policy in the tenant editor and the filter in the people view.

## Impact

`internal/core`, `internal/service`, a migration for both engines, `internal/httpapi`, `internal/client`,
`cmd/tenant.go`, `cmd/auth.go`, `internal/web` (tenant, users, settings, layout, i18n), `internal/tui`,
`internal/sshd`, `api/openapi.yaml`, docs, the tix skill.
