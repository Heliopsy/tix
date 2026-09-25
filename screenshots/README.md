# Screenshots

Captured against a seeded demo database on the commit that added them.

| File | Surface |
| --- | --- |
| [cli-task-list.png](cli-task-list.png) | `tix task ls` |
| [cli-task-show.png](cli-task-show.png) | `tix task show -o yaml` |
| [cli-agent.png](cli-agent.png) | `tix claim next -o json` and NDJSON piped through `jq`, the agent path |
| [tui-board.png](tui-board.png) | `tix tui`, the workflow board |
| [web-task-list.png](web-task-list.png) | Browser task list |
| [web-task-detail.png](web-task-detail.png) | Browser task detail |
| [web-bundles-dark.png](web-bundles-dark.png) | Import and export, the snapshot tab |
| [web-tenant-shape.png](web-tenant-shape.png) | The tenant screen, showing what sits under what |
| [web-stats.png](web-stats.png) | Statistics: throughput, lead time and what is waiting longest |

The browser images use the dark scheme. The interface also offers light and a
low contrast scheme, chosen per browser from the sidebar.

## Regenerating them

Seed a database somewhere disposable, never the one you actually use, and point
`TIX_DATABASE_DSN` at it so the commands read as a reader would type them.

The three CLI images come from `termshot`, rendered with `--columns` set to the
width of the output rather than the default, which otherwise wraps a wide table
into something that looks broken.

Two things make the terminal awkward to capture, and both cost an afternoon to
find:

`tix` writes an OSC 11 background-colour query and a cursor-position report to
stdout when it starts in a pty, and `termshot` panics parsing them. Strip
`\e]11;?\e\\` and `\e[6n`, along with any reply the terminal sends back, before
rendering.

The board is not a `termshot` image. Its font has no box-drawing or emoji
glyphs, so the column frames break into dashes and the project icon and the
`✓` become tofu. That image is a tmux `capture-pane -e` frame, converted with
`aha`, and photographed in Chrome with DejaVu Sans Mono plus Noto Color Emoji
at a line height tight enough for the box characters to join up.

The browser images are Chrome at a device pixel ratio of 2 against a running
`tix serve`, signed in as a seeded user.

Two more things learned the second time round.

`termshot --raw-read` takes a file of raw terminal output instead of running
the command itself, which is the easier way past the panic: capture the output
under `script`, strip the queries and the replies from the file, and hand the
cleaned file to `termshot`. Nothing then races the pty.

A `capture-pane` frame is as tall as the pane, not as tall as the interface, so
the blank rows below the board arrive with it and become empty window inside
the image. Trim the trailing blank lines from the capture before `aha` sees
them, and size the pane so the columns are full rather than padded.
