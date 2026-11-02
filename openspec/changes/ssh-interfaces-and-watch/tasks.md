# Tasks

## 1. Registry and menu

- [x] 1.1 `internal/tui/interfaces.go`: `Interface`, `Interfaces()`, `LookupInterface`
- [x] 1.2 Start menu view, `~` binding, palette entry, project picker for Watch
- [x] 1.3 Pause and resume on the activity view

## 2. SSH

- [x] 2.1 Route the session command through the registry; refusal lists the interfaces
- [x] 2.2 `watch` stream: filters, project resolution, PTY and plain rendering, ndjson
- [x] 2.3 Connection registry, keepalive without idle, ctrl-c, shutdown
- [x] 2.4 Shared watch banner in `internal/output`

## 3. Tests

- [x] 3.1 Registry parity: launcher, menu row and usage for every interface
- [x] 3.2 Watch over a real in-process SSH connection with a fake event source
- [x] 3.3 Reserved name and `board KEY`
- [x] 3.4 PTY and plain rendering

## 4. Docs

- [x] 4.1 `docs/deployment.md`, `docs/tui.md`, `skills/tix/reference/ssh.md`
