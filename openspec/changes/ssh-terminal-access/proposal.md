# Serve the terminal interface over SSH

## Why

The terminal interface is the part of tix that is hardest to describe and easiest to show, and today seeing it
means installing a binary and pointing it at a database. A link to a video is not the product.

Every machine already has an SSH client. `ssh tix.example.com` can land in the real interface, driven by the
real keys, against a real database, with nothing to install and nothing to configure. The rendering side is
close to free: `tui.Options` already takes `In io.Reader` and `Out io.Writer`, so the interface was never bound
to `/dev/tty`.

The work is identity. A credential today is a password or an opaque token; this needs a public key, and it has
to reach the same authorization decisions everything else goes through rather than growing a second path.

## What Changes

- **`tix ssh`** serves the terminal interface over SSH. tix is the SSH server; there is no sshd, no system user
  and no shell.
- **The key fingerprint is the identity.** Any public key is accepted. SSH requires a client to prove a key but
  does not require the server to have seen it before, and that proof is the whole identity: no signup, no
  password, no enrolment.
- **Each fingerprint gets an ephemeral tenant of its own**, seeded on first connection from a snapshot through
  the existing importer. The same key connecting again gets the same tenant back, with its changes intact.
- **A public-key credential** in `internal/auth`: a fingerprint, and one interface that resolves it to an actor.
  The demo listener satisfies that interface by provisioning; key enrolment will satisfy it from registered keys
  without changing anything above it.
- **Limits that keep a public listener from becoming an availability problem**: a time to live that slides from
  the last connection, a reaper, a cap on live tenants, a cap on tasks per tenant, and connection rate limiting
  per source address.
- **A persisted host key**, generated on first run at mode 0600, so a returning visitor is not warned.
- **The same non-loopback bind guard** `tix serve` applies.
- **Its own database**, refusing the zero-configuration store, because a listener strangers reach must not share
  a process boundary with real tenants.

## Deliberately not in this change

- **Key enrolment for real users** (`tix user key add`). The credential is designed for it; the demo path is
  built first.
- **A read-only mode.** If one is offered it belongs in `internal/authz` as a narrower scope set, never in the
  interface hiding keys. A visitor alone in their own tenant has nobody else's work to spoil, so there is
  nothing for it to protect yet.
- **Binding port 22.** That is a deployment problem, documented rather than solved in code.

## Impact

- New: `internal/sshd`, `cmd/ssh.go`, `internal/auth/publickey.go`.
- New dependencies: `github.com/charmbracelet/wish` and `github.com/charmbracelet/ssh`.
- No change to `internal/core`, `internal/service`, `internal/store` or the schema.
