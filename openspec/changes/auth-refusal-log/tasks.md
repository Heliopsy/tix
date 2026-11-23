# Tasks

- [x] `auth.RefusalNote` and the reason codes in `internal/auth/refusal.go`
- [x] Token and session verifiers note each refusal; `auth.Withdrawn` from the store lookup for a disabled actor
- [x] Router notes `no_credentials` and `wrong_tenant`, clears the note once a request authenticates
- [x] Service notes policy denials, `require_claim`, the second-factor gate, password, code and SSH sign-in refusals
- [x] `logging.Dedupe` and the `auth refused` record in `internal/httpapi/refusallog.go`
- [x] `client_ip`, `user_agent` and `auth_reason` on the request line
- [x] `reason` on the SSH listener's refusal lines
- [x] Tests: every reason with its fields, no credential in the captured log, trusted proxy resolution,
  dedupe on the fake clock
- [x] Document the fields, the codes and tracing a client in `docs/deployment.md` and `docs/configuration.md`
- [x] Report a held-back count nobody carried, from a sweep and at shutdown; key IPv6 by /64; one overflow bucket
  once full instead of evicting; sweep once a window instead of per insert
- [x] No refusal or `auth_reason` for a 5xx or a recovered policy refusal; a code that could not be checked is
  not `totp_invalid`
