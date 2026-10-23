# Tasks

## 1. Schema and configuration

- [x] 1.1 Migration: `user_totp` (user_id PK referencing `users`, sealed secret, nonce,
      `confirmed_at`, `last_step`, `failures`, `locked_until`, timestamps) and
      `user_recovery_codes` (user_id, code_hash, `used_at`)
- [x] 1.2 Migration: `tenants` gains `require_totp_web`, `require_totp_cli`, `require_totp_ssh`,
      all defaulting to false, so no existing deployment changes behaviour
- [x] 1.3 `internal/config`: `auth.totp_key` with its `TIX_AUTH_TOTP_KEY` mapping, 32 bytes base64,
      validated at load so a malformed key fails at startup rather than at first enrolment

## 2. The verifier

- [x] 2.1 `internal/auth/totp.go`: RFC 6238 over HMAC-SHA1, six digits, thirty second step,
      accepting one step either side. Table tests against the RFC's published vectors
- [x] 2.2 `internal/auth/totp.go`: the accepted-step high-water mark, so a code is single use.
      `FakeClock`-driven test replaying one code inside the window
- [x] 2.3 `internal/auth/seal.go`: AES-256-GCM over the secret, random nonce, user id as AAD.
      Test that a ciphertext moved to another user's row fails to open
- [x] 2.4 `internal/auth/recovery.go`: ten 128-bit codes, SHA-256 hashes, constant-time compare,
      single use. The design records why this one is not argon2id
- [x] 2.5 `internal/auth/totp.go`: five failures then a fifteen minute lock, counted on the
      enrolment, cleared by success, reported as its own error kind

## 3. Service and policy

- [x] 3.1 `internal/service/totp.go`: begin, confirm, remove, regenerate codes, administrative clear.
      Enrolment refused outright when no key is configured
- [x] 3.2 `internal/service/session.go`: the requirement is evaluated at session mint against the
      tenant and the surface, never per request
- [x] 3.3 `internal/service/totp.go`: an unenrolled user on a requiring surface enters enrolment and
      completes sign-in on success. Test that enabling the flag cannot lock out the tenant
- [x] 3.4 `internal/authz`: the scope for clearing another user's enrolment, held by admin and not
      by member. Matrix test
- [x] 3.5 Audit entries for enrol, confirm, remove, administrative clear, and recovery code spent,
      naming which code

## 4. The three surfaces

- [ ] 4.1 `internal/web`: the second sign-in step, working without JavaScript, preserving `next`;
      enrolment and recovery codes on settings; enrolment state on the admin user list
- [ ] 4.2 `cmd/login.go`: the prompt after the password, never a flag or an environment variable;
      a non-interactive terminal fails naming API tokens
- [ ] 4.3 `cmd/user_totp.go`: enrol, status, remove, admin clear, with `otpauth://` and a scannable
      code where the terminal supports it
- [ ] 4.4 `internal/sshd`: the prompt after the key is accepted and before any tenant data is drawn
- [ ] 4.5 `internal/capability`: registry entries with CLI, HTTP and Web bindings for every new
      operation, since a missing binding fails the parity build

## 5. Diagnostics and documentation

- [ ] 5.1 `tix doctor`: whether a key is configured, how many enrolments exist, enrolments with no
      key flagged as unrecoverable, and the API token exemption stated
- [ ] 5.2 `tix doctor`: warn when a tenant requires a factor on all three surfaces and holds fewer
      than two enrolled administrators, which is the configuration that locks somebody out
- [ ] 5.3 `docs/`: the key is as load-bearing as the database, losing it means re-enrolment for
      everyone, and API tokens are exempt
