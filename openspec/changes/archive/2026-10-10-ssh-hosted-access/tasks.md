# Tasks

## 1. Contract and schema

- [x] 1.1 `internal/core/sshkey.go`: `SSHKey`, `SSHKeyFilter`, and the `Service` methods `EnrolSSHKey`,
      `ListSSHKeys`, `RevokeSSHKey`. Stdlib only, per the core rule
- [x] 1.2 `internal/store/migrations/00NN_ssh_keys.sql`: the table, the unique on `(tenant_id, fingerprint)`,
      the partial lookup index
- [x] 1.3 `internal/store/postgres/schema.go`: extend `adjustments()` so the new table receives an RLS policy.
      It only runs at migration 1, so a table added later gets none unless this is done
- [x] 1.4 `internal/store/sql`: scoped queries for the table, through the builder, never raw

## 2. Service

- [x] 2.1 `internal/service/sshkey.go`: enrolment parsing and validation, conflict on a duplicate fingerprint
      in a tenant, revocation, listing including revoked keys, all through the audit and outbox helper
- [x] 2.2 Authorization: an actor manages its own keys, a tenant administrator manages the tenant's
- [x] 2.3 `internal/service/sshkey_test.go`: the four enrolment scenarios, both revocation scenarios

## 3. Enrolled lookup

- [x] 3.1 `internal/sshd/enrolled.go`: the second `auth.PublicKeyLookup`, resolving fingerprint plus username
      to an actor by the four-step order in the design
- [x] 3.2 Refusals that disclose nothing: an unenrolled key and a nonexistent tenant are indistinguishable
- [x] 3.3 `last_used_at` written on success, outside any transaction, never failing the connection
- [x] 3.4 `internal/sshd/enrolled_test.go`: each resolution step, the ambiguity refusal and its message, the
      revoked key, the wrong tenant

## 4. Mode selection

- [x] 4.1 `cmd/ssh.go`: `--demo` opting in to sandbox provisioning; enrolled lookup is the default
- [x] 4.2 Narrow the zero-configuration store refusal to demo mode
- [x] 4.3 `internal/config`: the `ssh.demo` key and its generated environment variable
- [x] 4.4 Update the command help and examples to lead with the hosted case

## 5. Managing keys from every surface

- [x] 5.1 `cmd/user_key.go`: `tix user key add | ls | rm`, reading a key from a file or stdin
- [x] 5.2 `internal/httpapi`: the `/api/v1/ssh-keys` routes and their handlers
- [x] 5.3 `internal/web`: the screen, its template and its handlers
- [x] 5.4 `internal/capability`: registry entries binding each operation to CLI, HTTP and web. The parity test
      fails the build without them
- [x] 5.5 Golden tests for the command, route tests that assert bodies rather than status codes alone

## 6. One process, two listeners

- [x] 6.1 `internal/server`: hold an optional `sshd.Server` in the worker set, binding before the HTTP server
      accepts so a port conflict fails at startup
- [x] 6.2 Drain SSH sessions on the shutdown timeout, closing a session that outlives it with a message
- [x] 6.3 `cmd/serve.go`: `--ssh-listen`, empty by default, and the SSH options it needs
- [x] 6.4 `internal/server/ssh_test.go`: both listeners serving, a change over one visible to the other, no
      listener without an address, the port conflict, the drain

## 7. Per-session renderer

- [x] 7.1 `internal/tui`: a `*lipgloss.Renderer` on `Config`, defaulting to today's behaviour
- [x] 7.2 `internal/sshd/session.go`: a renderer per session against its own pty
- [x] 7.3 A test holding two sessions at different depths, asserting each renders at its own

## 8. Isolation

- [x] 8.1 Extend the leak suite to enrolled keys across the service, the API routes and the web handlers
- [x] 8.2 A Postgres RLS test for the new table, which is the trap in 1.3 proving itself caught

## 9. Documentation

- [x] 9.1 `docs/deployment.md`: hosting SSH, enrolment, the username convention, running both listeners
- [x] 9.2 `docs/configuration.md`: the new keys
- [x] 9.3 Release note for the default flip, naming `--demo` as the migration

## 10. Security suite

This is a credential and an authentication path, so these are owned tasks rather than assumed coverage. Each
asserts a refusal, and each is verified by mutation: break the check, watch the test fail, restore it.

- [x] 10.1 Canonical storage: options and command directives are dropped, the stored key is the key alone,
      the fingerprint matches what `ssh-keygen -lf` prints for the same file
- [x] 10.2 One identity per key: the same key differing in comment or whitespace is a conflict, not a second row
- [x] 10.3 Multi-key and trailing-content submissions are refused, nothing recorded
- [x] 10.4 Authentication refusals: unenrolled, revoked, wrong tenant named, tenant not named while ambiguous
- [x] 10.5 No credential but a public key authenticates: password, keyboard-interactive and none are refused
- [x] 10.6 Privilege escalation: enrolling against another actor without authority is refused, including
      against an actor with a higher role, and including across the tenant boundary
- [x] 10.7 A session holds exactly its actor's authority, no more, asserted against the policy rather than by
      inspection
- [x] 10.8 Disclosure: an unenrolled key gets the same refusal for a tenant that exists and one that does not;
      the ambiguity message names only tenants the caller is enrolled in
- [x] 10.9 The unscoped lookup is confined: a test asserts it is the only unscoped path to enrolled keys and
      that no session runs without a tenant scope
- [x] 10.10 A failure recording last use does not change the authentication outcome
- [x] 10.11 Postgres RLS on `ssh_keys`, proving the migration-keyed policy actually landed
- [x] 10.12 Leak suite extended: enrolled keys across the service, every API route and every web handler
