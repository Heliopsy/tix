# One palette for every surface, completion that installs itself, and numbers

## Why

A tenant's colour exists today, and it is a hash. `brandFor` runs the tenant key and id through FNV and
picks one of six hard-coded pairs. Nobody chose it, nobody can change it, and it reaches exactly one
surface: the web UI. The TUI has its own unrelated palette in `internal/tui/theme.go`, so the same tenant
is teal in a browser and whatever lipgloss was given in a terminal. There is no answer to "make our board
match our brand" beyond changing the tenant's name until the hash lands somewhere better.

The web's light / dark / dim schemes are a separate axis and they work. This is about the accent, and about
the two surfaces disagreeing on it.

Shell completion has the same shape of gap. `tix completion bash` prints a script, and then the reader has
to know that bash wants it in one directory, zsh wants it on `fpath`, and fish wants a differently named
file somewhere else. The command that knows all three prints its output to stdout and leaves the knowing to
the user.

## What Changes

- **A theme is a named thing with a palette**, in `internal/core`, resolvable by name. Several are built in.
- **A tenant records which theme it uses.** `tenants.theme` is a new column; empty keeps today's hashed
  accent, so nothing changes for a tenant nobody has themed.
- **Custom themes come from configuration.** A `themes:` block defines palettes by name, and a tenant may
  name one. Config themes and built-ins share one registry and one validator.
- **The TUI reads the same resolved theme as the web**, so a tenant's accent is its accent in both.
- **`tix theme ls`** lists what is resolvable, built-in and custom, with the palette.
- **`tix tenant set --theme`** names a tenant's theme, and refuses a name that does not resolve.
- **`tix completion install`** writes the script where the running shell looks for it, reports the path,
  says whether anything still has to be sourced, and takes `--dry-run`, `--shell` and `--uninstall`.
- **Statistics, on all three surfaces.** Throughput over a window, lead time, who closed what, and where
  the work is sitting. `tix stats`, a `/stats` screen, and a TUI view, all over one `Service` method.

## What is deliberately not here

Charts. The web screen draws bars with `<div>` widths and the TUI draws them with block characters; a
plotting library for six numbers would be the largest dependency in the project. Burndown, cycle-time
percentiles and per-sprint reporting are not in this change either: none of them mean anything until
someone has used tix for a few weeks, and inventing them now would be guessing at a workflow.

"Top performers" is counted as tasks moved to a terminal state, attributed to the actor who moved them.
That is the honest description of the number, and the screen says so, because a leaderboard that looks
like a productivity measure and is really a ticket-closing count is worse than no leaderboard.

## Impact

- `internal/core`: a new `Theme` type and registry, and one field on `Tenant`. The contract widens; nothing
  in it changes meaning.
- Migration `0012_tenant_theme.sql`, forward only, one nullable column.
- `internal/web` stops hashing when a theme is set, and keeps hashing when it is not.
- `internal/tui` takes its accent from the resolved theme instead of a constant.
- `internal/config` gains `themes`, with the same precedence every other key has.
- `internal/service`: one new read-only method, `Stats`, and the store queries behind it.
- Capability registry entries for every new operation, each with CLI, HTTP and Web bindings.
- `docs/` gains a theming page and a statistics page; `ROADMAP.md` moves shell completion and theming out
  of "planned" and into what shipped.
