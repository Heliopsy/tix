# SSH interfaces: a start menu and `watch`

## Why

`tix watch` is the quickest way to see what a tenant is doing, and over SSH it does not exist:
`ssh tix@host "watch --project house"` is refused with "the only argument it takes is a project
key to open". The SSH listener knows exactly one thing to host, the board, and a session with no
command lands on the project list with no hint that anything else could be offered.

## What Changes

- The SSH session's command picks an *interface* from a registry. Two are registered: `board`
  (the terminal interface, optionally on a named board) and `watch` (the live event stream).
- `ssh tix@host watch [KEY...] [--project REF] [--type T] [--actor A] [--since N] [--limit N]
  [-o ndjson]` streams the same lines `tix watch` prints, as the key's actor, tenant-scoped,
  through `Subscribe`'s own permission check. Plain without a pseudo-terminal, coloured with one.
  It ends on ctrl-c, client disconnect, a missed keepalive limit, an administrator ending the
  connection and server shutdown.
- `watch` is reserved as a command word. A project keyed `watch` opens with `board watch`; any
  other key still opens with the bare key as before.
- A refused command names every registered interface.
- A hosted session with no command lands on a start menu listing every registered interface:
  Boards and Watch, Watch with a project picker. `~` returns to it from anywhere and the palette
  offers it. The activity view pauses on `space`.

## Impact

- `internal/tui`: the interface registry, the start menu view, pause on the activity view.
- `internal/sshd`: command routing through the registry, the watch stream.
- `internal/output`: the watch banner, shared by `tix watch` and the SSH stream.
- Docs: `docs/deployment.md`, `docs/tui.md`, `skills/tix/reference/ssh.md`.
