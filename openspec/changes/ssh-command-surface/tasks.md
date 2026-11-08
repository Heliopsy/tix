# Tasks

## SSH command surface

- [ ] `capability.SurfaceSSH`, `SSHCommands()`, exemptions for `login` and `logout`
- [ ] Parity test: SSH bound, no SSH exemption on an operation without a CLI binding
- [ ] `core.SurfaceFrom` and the service's sign-in surface read from the via record
- [ ] `config.Defaulted()` resolution that reads nothing
- [ ] `cmd.RunHosted`: pruned tree, pre-set connection, local flag refusal
- [ ] `localFile` guard on every path a command opens, with a test that finds unguarded calls
- [ ] `bundle import`, `import tasks` and the rest read `-` from standard input
- [ ] `whoami` and `status` describe an SSH session
- [ ] `sshd`: routing, command sessions, argument limits, input cap, timeout, live connection
- [ ] Second factor for command sessions: prompt with a terminal, `TIX_CODE` without one
- [ ] Wire the runner into `tix ssh` and `tix serve --ssh`; refuse commands in demo mode
- [ ] Tests: real SSH client, refusals per class, streams and exit codes, audit via, parity

## Terminal transfer view

- [ ] Transfer view: task, bundle and snapshot export and import by path
- [ ] Task import mapping steps: dry run, columns, preview, confirm
- [ ] Hosted view names the `ssh` commands; registry Limitation on the six operations
- [ ] Tests for the view, local and hosted

## Docs

- [ ] `docs/deployment.md`: running commands over SSH, the security model
- [ ] `docs/tui.md`: the transfer view, the hosted limitation
- [ ] `skills/tix`: commands over SSH
