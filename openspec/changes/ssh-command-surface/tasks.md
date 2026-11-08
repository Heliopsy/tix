# Tasks

## SSH command surface

- [x] `capability.SurfaceSSH`, `SSHCommands()`, exemptions for `login` and `logout`
- [x] Parity test: SSH bound, no SSH exemption on an operation without a CLI binding
- [x] `core.SurfaceFrom` and the service's sign-in surface read from the via record
- [x] `config.Defaulted()` resolution that reads nothing
- [x] `cmd.RunHosted`: pruned tree, pre-set connection, local flag refusal
- [x] `localFile` guard on every path a command opens, with a test that finds unguarded calls
- [x] `bundle import`, `import tasks` and the rest read `-` from standard input
- [x] `whoami` and `status` describe an SSH session
- [x] `sshd`: routing, command sessions, argument limits, input cap, timeout, live connection
- [x] Second factor for command sessions: prompt with a terminal, `TIX_CODE` without one
- [x] Wire the runner into `tix ssh` and `tix serve --ssh`; refuse commands in demo mode
- [x] Tests: real SSH client, refusals per class, streams and exit codes, audit via, parity

## Terminal transfer view

- [x] Transfer view: task, bundle and snapshot export and import by path
- [x] Task import mapping steps: dry run, columns, preview, confirm
- [x] Hosted view names the `ssh` commands; registry Limitation on the six operations
- [x] Tests for the view, local and hosted

## Docs

- [x] `docs/deployment.md`: running commands over SSH, the security model
- [x] `docs/tui.md`: the transfer view, the hosted limitation
- [x] `skills/tix`: commands over SSH
