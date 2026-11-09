# Bound the memory password hashing can hold

## Why

The load test showed a server at about 100 MiB jump to about 255 MiB in its first second, with one person
signing in and a live Go heap of 46 MiB. Each argon2id derivation allocates 64 MiB under the old defaults, the
first sign-in after a start derived twice (once to mint the dummy hash unknown addresses are checked against,
once to check the password), and the bound on concurrent verifications was sized from the core count, not
from memory, and did not cover minting a hash. Production at 256 MiB was killed with about four people
signing in.

## What Changes

- One process-wide bound covers every argon2id derivation: sign-in, password change, user create and update,
  tenant admins, invite acceptance and the rehash after a sign-in.
- The bound is sized from the Go soft memory limit (a quarter of it over the per-hash memory, at least 1, at
  most the core count, 2 without a limit), or set by `auth.max_concurrent_hashes`.
- A caller waits for a slot until its context ends or ten seconds pass, then gets `precondition_failed` with
  `reason: password_hashing_busy`. Sign-in and invite acceptance pass that through instead of folding it into
  "invalid credentials", and never admit anyone unchecked.
- The dummy hash is encoded from random bytes at the hasher's parameters, without a derivation. Checking
  against it still derives once.
- Defaults move from m=64 MiB t=3 p=4 to m=46 MiB t=2 p=1. `auth.argon2_memory_kib`, `auth.argon2_time` and
  `auth.argon2_threads` change them; a setting below OWASP's listed configurations is refused.
- `NeedsRehash` reports any difference from the policy, so a stored hash moves to the current setting on the
  next successful sign-in, cheaper or dearer.

## Capabilities

### Modified Capabilities

- `auth`: password hashing runs under a memory-sized bound with a busy refusal, and the cost is configurable.

## Impact

`internal/auth/gate.go`, `internal/auth/password.go`, `internal/service` (session, invite, user, tenant),
`internal/config`, `internal/connect`, `docs/configuration.md`, `docs/deployment.md`, `SECURITY.md`.
