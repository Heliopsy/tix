# Sessions renewed by use

## Why

A sign-in still ends a fixed `session.ttl` after it was made, used or not. Somebody working in the browser, the
desktop app or the mobile app all day is signed out in the middle of it 12 hours after signing in, and every
client then has to recover from a `401` it had no warning of. Keep me signed in only moves the cliff to 30 days.

## What Changes

- Each use renews a session to `session.ttl` past that use (`session.remember_ttl` for a remembered one). The
  write rides on the existing once-a-minute last-use touch, so it costs no extra writes.
- New server keys cap renewal from the sign-in: `session.max_lifetime` (30d) and `session.remember_max_lifetime`
  (90d), with `TIX_` variables and `tix serve` flags. `0` turns renewal off, the old fixed lifetime.
- A tenant switch carries the original sign-in time, so switching does not restart the cap.
- A remembered session's cookie is sent again with the new `Max-Age` when a request renews it.
- `GET /whoami` with a session reports `session_expires_at`; `SessionPolicy` reports `max_lifetime` and
  `remember_max_lifetime`. The sessions list already shows the stored expiry, which now moves.
- A session deleted between its lookup and its touch is refused as unauthenticated instead of a 500.
- No migration: the cap is computed from `sessions.created_at`.
