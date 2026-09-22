# Tasks

## 1. The public-key credential

- [x] 1.1 `internal/auth/publickey.go`: `Fingerprint`, `FingerprintID`, `PublicKeyLookup`, `PublicKeyVerifier`
- [x] 1.2 `internal/auth/publickey_test.go`: fingerprint form and stability, verifier resolution and refusals

## 2. The listener

- [x] 2.1 `internal/sshd/sshd.go`: options, defaults, server assembly, listen, serve, close
- [x] 2.2 `internal/sshd/hostkey.go`: generate on first run at 0600, reload, refuse a loose or unparseable key
- [x] 2.3 `internal/sshd/ratelimit.go`: bounded token bucket per source address
- [x] 2.4 `internal/sshd/session.go`: per-session actor, context and model over the session's streams
- [x] 2.6 Colour decided from the session environment through `tui.Config.Color`
- [x] 2.7 Pin the lipgloss colour profile, which a server process otherwise resolves to Ascii
- [x] 2.5 Non-loopback bind guard, reusing `server.CheckBindSafety`

## 3. Ephemeral tenants

- [x] 3.1 `internal/sshd/sandbox.go`: fingerprint to tenant key, lookup, create, touch, visitor actor, scopes
- [x] 3.2 `internal/sshd/seed.go`: demo snapshot through `ImportFrom`, short lease, sandbox notice row
- [x] 3.3 `internal/sshd/reaper.go`: hard delete of tenants unseen for the time to live
- [x] 3.4 `internal/sshd/capped.go`: per-tenant task cap
- [x] 3.5 Tenant cap refusing a new fingerprint without evicting an existing tenant

## 4. Proof

- [x] 4.1 `internal/sshd/isolation_test.go`: concurrent sessions cannot list or address each other's rows
- [x] 4.2 `internal/sshd/isolation_test.go`: a real SSH client, two keys, two tenants
- [x] 4.3 `internal/sshd/sshd_test.go`: seeding, return visit, sliding expiry, cascade, both caps
- [x] 4.4 Hand verification with a real `ssh` client against a scratch database

## 5. Wiring and documentation

- [x] 5.1 `cmd/ssh.go`: flags, target guard, host key resolution, signal handling
- [x] 5.2 `docs/deployment.md`: the listener, its flags, expiry, caps, and port 22
- [x] 5.3 `docs/agents.md`: the public key as a third credential, and what is not built yet

## 6. Configuration, keepalive and session caps

- [x] 6.1 `internal/config`: an `SSH` section covering every listener setting, with defaults and validation
- [x] 6.2 `cmd/ssh.go`: read the resolved keys, flags overriding only where one was given
- [x] 6.3 `internal/sshd/keepalive.go`: keepalive requests, missed-reply count, connection dropped
- [x] 6.4 Idleness measured from the interface's input, so keepalive traffic cannot defeat the idle timeout
- [x] 6.5 `internal/sshd/gate.go`: caps on concurrent sessions per key and in total
- [x] 6.6 `docs/configuration.md` and `docs/deployment.md`: the keys, the keepalive and the caps
- [x] 6.7 Tests: key registry and layering, the activity tap, a vanished client, both caps

## 7. Left for the next change

- [ ] 7.1 `tix user key add`: enrol a public key against an existing user
- [ ] 7.2 A store-backed `PublicKeyLookup` over enrolled keys, and the table behind it
- [ ] 7.3 A read-only scope set in `internal/authz`, if one is wanted
- [ ] 7.4 A `*lipgloss.Renderer` on `tui.Config`, so each session renders at its own client's depth
