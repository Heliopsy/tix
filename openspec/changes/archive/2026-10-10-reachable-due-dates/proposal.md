# Due dates you can ask for, and see

## Why

A task carries `due_at` as a typed column. `--due` sets it on `task add` and `task edit`, `--sort due_at`
orders by it, and the default `--sort urgency` is a compound ordering that reads it inside every priority
band. `core.TaskFilter` has carried `DueBefore` and `DueAfter` since the beginning, both stores answer
them, and `internal/query` answers them in memory.

None of that is reachable from the command line. `tix task ls` has `--assignee`, `--blocked`, `--claimed`,
`--query`, `--tag`, `--status` and `--project`, and no deadline flag at all. The filter expression has
`due-before:` and `due-after:`, which take a date the reader has to work out for themselves, so the
question anybody actually asks — "what is overdue?" — is answerable over HTTP and not from a shell. That
is backwards for a tool whose primary users are agents on a command line.

The terminal board is worse. It drew a due-date marker on every card, which marked nearly every card in a
real backlog and so distinguished nothing, and the marker was removed rather than narrowed. Nothing
replaced it, so a due date is invisible on the board while silently driving the default sort order: the
cards are ordered by a deadline the reader cannot see.

The browser shows the date on the task screen and nothing on the listing, and offers no way to filter by
it.

Underneath all of it is a structural hole. `capability.Registry` asserts that `task.list` is *bound* on
every surface. It does not assert that the surfaces accept the same filters, which is exactly how
`DueBefore` and `DueAfter` came to reach HTTP and not the CLI, and it is the same shape of hole that let
multi-hop transitions exist in one control and nowhere else.

## What Changes

- **`tix task ls` gains `--overdue`, `--due-before` and `--due-after`**, taking the dates `--due` already
  takes. A flag wins over the same bound named in an expression, as `--limit` and `--sort` already do, and
  `--overdue` beside an explicit `--due-before` is refused rather than silently overriding it.
- **The filter expression gains `due:`**, with the named windows `overdue`, `today`, `week` and `month`
  and the bounded forms `due:<DATE` and `due:>DATE`. It is a second spelling of the existing bounds rather
  than a new capability, so the terminal filter bar and the browser filter bar get it for free through the
  one parser they already share. `due-before:` and `due-after:` keep working.
- **A card carries a deadline marker only while the deadline is pressing.** `due!` for a deadline past,
  `due` for one inside the next week, and nothing at all for a date further out or for no date. The
  marker's colour comes from the theme, through a `Theme.Due` style built the same way `Theme.Priority`
  is, so it works on every shipped scheme; the marker is a word, so it survives a colourless terminal.
  The task detail names the state beside the date.
- **The browser listing gains a deadline badge and a deadline control.** The badge follows the same rule
  as the card. The control writes a `due:` term into the filter box rather than setting a bound of its
  own, so what it asked is visible and correctable and cannot drift from the language.
- **`core.DueState` is the one answer to "how does this deadline stand".** Every surface reads it, so
  "overdue" cannot come to mean three things in three places.
- **A filter parity guard** reflects over `core.TaskFilter` and compares the filter surface across the CLI,
  the terminal, the browser and the API, including a round trip of every field through the real client and
  the real router. A field reachable from one transport and not another fails the build unless the absence
  is named with a reason.

## Impact

- Affected specs: `task-filtering`, `tui-board`, `web-tasks`
- Affected code: `internal/core`, `internal/query`, `cmd/task.go`, `internal/tui`, `internal/web`,
  `internal/capability` (tests only)
- No change to the HTTP API's parameters: `due_before` and `due_after` already travel, and the round trip
  guard now proves it. No migration, no change to `core.Service`, no new capability registry entry.
