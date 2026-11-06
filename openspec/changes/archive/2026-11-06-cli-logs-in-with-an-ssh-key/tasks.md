# Tasks

## 1. Contract and stores

- [x] 1.1 `core.SSHChallenge`, `core.SSHLoginInput`, `core.WithAudience`, login method constants
- [x] 1.2 `store.Tx.GetSSHKeyByFingerprint` and `ClaimSSHKeyUse` on both engines

## 2. Auth and service

- [x] 2.1 `internal/auth` sshsig sign and verify, the signed nonce, the ticket carries the method
- [x] 2.2 `SSHLoginChallenge` and `SSHLogin`, registry entries, audit and event carry the method

## 3. Surfaces

- [x] 3.1 HTTP routes and client
- [x] 3.2 `tix login --ssh-key [PATH]` with agent, default files and passphrase prompt
- [x] 3.3 `tix login` dials without asking the server who it is

## 4. Docs

- [x] 4.1 `docs/api.md`, `docs/deployment.md`, `docs/second-factor.md`, `docs/commands.md`
- [x] 4.2 `skills/tix`
