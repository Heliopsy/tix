# SSH command surface

## Why

The SSH listener opens interfaces only: `board`, `watch` and a bare project key. Anything else needs a
configured command line with an API token, so a person whose only credential is an enrolled SSH key can
read their board but cannot script it, export it or import into it. Every other surface reaches the
same operations, and the owner wants the five surfaces to cover the same features.

The terminal interface also still lacks the six import and export operations, recorded as GAPs.

## What Changes

- An SSH session whose command names a tix CLI command runs it in-process as the key's actor, in the
  key's tenant: `ssh acme@host task ls -o json`, `ssh acme@host task add "x" -p INFRA`. It is never a
  shell and never an exec of a binary.
- The commands a session may run are exactly the CLI bindings of the capability registry that carry no
  SSH exemption. Commands with no registry binding (`config`, `ctx`, `serve`, `init`, `docs` and the
  rest) are not reachable at all; `login` and `logout` are exempted with a reason.
- Flags that pick a local target or credential (`--db`, `--server`, `--token`, `--ctx`, `--config`,
  `--tenant`, `--no-discovery`, `--allow-network-fs`, the `--log-*` family) are refused, and so is any
  flag or argument naming a file on the server. Standard input and output take their place.
- Standard input, output, error and the exit code pass through, so `ssh host export tasks -p X > file`
  and `ssh host import tasks -p X < file` work.
- Audit entries record `via=ssh` with the key fingerprint and client address.
- The second factor is owed on the same terms as a board session. A command session proves it at a
  prompt when it has a terminal, or with `TIX_CODE` sent as an environment variable when it has none.
- `ssh` becomes a capability surface. The parity test fails when an operation the CLI binds is not
  reachable over SSH and carries no exemption with a reason.
- Command sessions hold a session slot, register as a live connection, read at most
  `server.max_import_bytes` of input and stop after a fixed command timeout.
- The local terminal interface gets a transfer view that exports and imports by file path, with the
  column mapping steps of a task import. Over SSH the view names the matching `ssh host ...` command
  instead, recorded as a Limitation on the TUI binding.

## Capabilities

### Modified Capabilities

- `ssh-access`: a session may run CLI commands; the command word rules change.
- `capability`: `ssh` is a bound surface derived from the CLI binding.
- `tui`: a transfer view binds the six import and export operations.

## Impact

`internal/sshd`, `internal/capability`, `internal/core` (the sign-in surface of an SSH call),
`internal/service/totp.go`, `internal/config`, `internal/connect`, `cmd/` (the hosted runner, local file
guards, wiring in `tix ssh` and `tix serve --ssh`), `internal/tui` (transfer view), `docs/deployment.md`,
`docs/tui.md`, `skills/tix`.
