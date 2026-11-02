# Design

## One registry, three readers

`tui.Interfaces()` is the list. The SSH dispatcher keeps one launcher per registered name, the
start menu draws one row per registered name, and the refusal message prints each one's usage.
Tests fail when a registered interface has no launcher, when a launcher has no registered
interface, and when the menu does not draw a row for one. Adding an interface is a registry row,
a launcher and a menu action, and the tests say which is missing.

The registry lives in `internal/tui` because `internal/sshd` already imports it and the menu needs
the view each entry opens, which is unexported.

## The name clash

The command was a bare project key, and `watch` is a valid key. Reserving the word is the only
reading that keeps `ssh host watch` meaning the same thing on every tenant. The escape is
`board KEY`, which opens any key, including `watch` and `board`. `board` on its own opens the
project list, and a one-word command that is not a registered name is still a project key, so
`ssh host infra` is unchanged. The reserved words compare case-insensitively, because project keys
do too.

## The stream is not the interface

`ssh host watch` does not run bubbletea. A stream piped into `grep` has no terminal and must not
receive escape codes or an alternate screen. It writes `output.FormatEventLine` lines, coloured
only when the client allocated a pseudo-terminal whose type takes colour (the same `colorFor` the
interface uses). Under a pseudo-terminal the ssh library's emulated terminal already turns `\n`
into `\r\n` for a client in raw mode, so the stream writes plain newlines either way. `-o ndjson` writes the event objects `tix watch
-o ndjson` writes.

Under a pseudo-terminal ctrl-c arrives as a byte on stdin, so the stream reads stdin and ends on
ctrl-c, ctrl-d or `q`. Without one, the client handles ctrl-c by closing the connection, and
stdin reaching EOF means nothing (`ssh -n`), so stdin is ignored.

The stream registers in the connection registry like an interface session, so an administrator
sees and can end it, and the drain's lingering-session sweep ends it with the shutdown notice.
The keepalive watchdog runs with the idle check off: a watch has no keystrokes by design, and the
keepalive is what notices a client that has gone.

## Project filters

`tix watch --project` takes identifiers. Over SSH a person types keys, so each `--project` value
and positional argument is resolved through `GetProject`, which accepts a key or an identifier and
applies the project read check. A key this actor cannot read is refused before the stream starts.

## The menu is SSH-only

`tix tui` keeps landing on the project list; a local reader started it to work a board. The SSH
listener sets `Config.Start` for a hosted session with no command. A sandbox still opens its seeded
board, which is the demo.
