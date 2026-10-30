# An SSH session can name the board it wants

## Why

Recording the product demo put a terminal pane on camera next to the browser,
connected over SSH to the same tenant, and it sat on the project list for the
whole take. The board was the thing it was there to show.

That is not a fault of the scene. The listener lands a hosted session on the
project list on purpose, because a tenant has several projects and there is no
board the listener could name without inventing one. A sandbox is the
exception: it holds exactly one seeded board and opens it.

The gap is that the client has no way to say which one it wants. The local
interface takes `tix tui -p infra`. Over SSH the username already carries the
tenant, and the only other field a client can send is the command, which the
listener read off the wire and dropped. So `ssh tix@host infra` connected,
ignored the argument and drew the project list, with no word about why.

## What changes

- An SSH session may name a project as the command: `ssh tix@host infra` opens
  that board, and is what `tix tui -p infra` is locally.
- With no argument, nothing changes. A hosted session lands on the project
  list and a sandbox opens its seeded board.
- An argument that could not name a board is refused before a session is
  built, naming what the argument is for. More than one argument is refused as
  reaching for a shell, because that is what it usually is.
- A key that is merely unknown is still the interface's to answer, against the
  projects the actor can actually reach, so a typo lands on the project list
  with the reason rather than a closed connection.

## Impact

- `internal/sshd`: `openProject` takes the session command.
- No change to authority. The board is opened as the session's own actor,
  through the same service and the same scopes, so naming a project reaches
  nothing the project list would not have reached.
