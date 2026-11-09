# Tasks

- [x] One process-wide gate around argon2id hash and verify, sized from the soft memory limit or
  `auth.max_concurrent_hashes`, waiting on the request context with a ten second cap
- [x] Busy refusal (`precondition_failed`, `reason: password_hashing_busy`) passed through by sign-in and
  invite acceptance
- [x] Dummy hash minted without a derivation; checking against it still derives once
- [x] Defaults to m=46 MiB t=2 p=1; `auth.argon2_*` keys with an OWASP floor
- [x] `NeedsRehash` on any difference, so old hashes migrate down on the next sign-in
- [x] Tests: N+1 verifies hold at most N, the timeout path refuses without deriving, the dummy derives once,
  migration through `NeedsRehash` and through sign-in, config parsing and validation
- [x] Docs: `docs/deployment.md` (per-hash cost times the bound), `docs/configuration.md`, `SECURITY.md`
- [x] Measured with `just load-smoke` before and after
