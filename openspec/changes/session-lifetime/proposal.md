# Configurable session lifetime

## Why

A password, SSH key or tenant switch sign-in lasts exactly 12 hours, a constant in `internal/service/session.go`.
An operator cannot shorten it for a deployment with stricter rules, cannot end sessions somebody walked away from,
and a person on their own machine has to sign in again every morning. A tenant on a shared server has no say at
all.

## What Changes

- Server keys `session.ttl` (12h), `session.idle_timeout` (0, off) and `session.remember_ttl` (30d, 0 disables),
  each with its `TIX_` variable and a `tix serve` flag.
- A tenant administrator can tighten all three for the tenant (`session_ttl`, `session_idle_timeout`,
  `session_remember_ttl`, which also takes `off`), from Tenant settings, `tix tenant edit`, the terminal tenant
  screen and `PATCH /v1/tenants`. A value past the server's is refused naming the server's. Unset inherits.
- Keep me signed in: a checkbox on the web sign-in screen and `tix login --remember` mint a session that lasts
  the remember lifetime. Its browser cookie persists with a matching `Max-Age`; any other session's cookie is a
  browser-session cookie.
- A session records when it last authenticated a request, at most once a minute, and a session unused past the
  idle timeout is refused. The idle timeout applies to remembered sessions too.
- Existing sessions keep the expiry they were minted with; new lifetimes apply to new sign-ins, the idle timeout
  at once.
- `SessionPolicy` (`GET /v1/auth/session-policy`, `tix tenant session-policy`, no credential needed) reports the
  values in force beside the server's. The sessions list marks a remembered session.
- Migration adding the three tenant columns and `sessions.last_seen_at`, `sessions.remembered`.
