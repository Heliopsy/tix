# The command line logs in with an SSH key

## Why

A person who already reaches the terminal interface with an enrolled SSH key still needs a password to
get a session for the command line. The key is already the credential the operator chose to trust for
that person, it already lives in their agent, and typing a password into a shell is the habit tix
otherwise tries to discourage.

## What Changes

- `tix login --ssh-key [PATH]` signs a server-issued challenge with an enrolled key and receives the same
  session `tix login --email` issues. With no path it tries the keys in `ssh-agent`, then the default key
  files under `~/.ssh`; with a path it loads that private key and asks for its passphrase without echo.
- Two routes, both public like the password login: `POST /api/v1/auth/ssh/challenge` answers a
  short-lived signed nonce and the signature namespace, `POST /api/v1/auth/ssh/verify` takes the public
  key, the signature and the nonce and answers a `LoginResult`.
- The signature is an OpenSSH `sshsig` signature (`ssh-keygen -Y sign`) under the namespace
  `tix-login@HOST`, so it is useless for git, for file signing, and against any other server.
- A second factor still applies: a person with a TOTP enrolment, or in a tenant requiring one on the
  command line, gets the same code challenge the password step returns.
- `tix login` no longer asks the server who it is before signing in, which it could not answer for a
  caller that has no credential yet.
- The audit entry `user.login` and the `session.created` event carry `method`: `password` or `ssh-key`.

## Capabilities

### Modified Capabilities

- `auth`: SSH key sign-in, its challenge, replay protection and second factor.
- `cli`: `tix login --ssh-key`.
- `http-api`: the two routes.

## Impact

`core.Service` gains `SSHLoginChallenge` and `SSHLogin`; `store.Tx` gains the tenant-scoped
`GetSSHKeyByFingerprint` and `ClaimSSHKeyUse`. No migration: replay protection reuses
`ssh_keys.last_used_at`. No new module: `golang.org/x/crypto/ssh/agent` is part of `x/crypto`, already
required.
