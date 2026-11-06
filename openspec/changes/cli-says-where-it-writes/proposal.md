# The command line says where it writes

## Why

A user on 0.43.0 worked for days through Claude Code and the tix skill, then found nothing on the server.
With nothing configured, tix had created `~/.local/share/tix/tix.db` and written every task there without a
word. Once she added a context with `tix ctx add --server ... --token ...`, most commands still answered
"no credential presented" until she exported `TIX_TOKEN`.

The second is a plain bug: `globals.credential()` read the resolved configuration only if something had
resolved it already, and `overrides()` is built before `dialWith` resolves anything. `task ls` and `task add`
resolve first for their own reasons, so they presented the token; `task show`, `claim next`, `status` and
most others did not.

## What Changes

- Every command presents the token the configuration resolves to (context, `server.token`,
  `TIX_SERVER_TOKEN`, `.env`), not only the commands that happened to read the configuration first.
- A command with nothing configured no longer creates the zero-configuration database by itself. On a terminal
  it asks (local database or server); elsewhere it exits 6 naming `tix ctx add ... --use` and `tix init`.
- Opting in: a database already at the default path, `target: local` (`TIX_TARGET`), `tix init`, `--db`,
  `TIX_DATABASE_DSN`, or a context naming a database. Existing installs and scripts keep working.
- While the default database is in use, each command prints one line naming it on standard error, past
  `--quiet`, never on standard output. `cli.target_notice: false` drops it.
- New `tix whoami`: server URL or local database path, context, tenant, user, and whether a token is presented
  and from where. `tix status` starts with the same lines. `tix ctx ls` gains a `TOKEN` column (set or not).
- A server refusing a command that had no token says so and names `tix ctx add ... --token`.
- The skill and docs tell an agent to run `tix whoami` before anything and stop when it says local while a
  server was expected. Skill version 19.

## Impact

- `cmd/root.go`, `cmd/target.go`, `cmd/status.go`, `cmd/ctx.go`, `cmd/demo.go`; `internal/connect`
  (`Target.Implicit`, `NeedsOptIn`); `internal/config` (`target`, `cli.target_notice`).
- Capability registry: `identity.whoami` is bound to `tix whoami` instead of `tix doctor`.
- Behaviour change for a fresh machine with no configuration: the first command asks or refuses instead of
  creating a database.
