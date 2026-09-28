# The whole installation in one screen, servers included

## Why

Somebody running tix cannot find out what is running. "Do I have two servers up, or one?" has no answer
today, from any surface.

`internal/connections` is explicitly in-process, and says so in its own package comment: nothing there is
persisted, a restart empties it, and one process can only ever describe itself. `tix connection --help`
already warns that a deployment running several servers gets a partial view from each, which is the honest
statement of a missing capability rather than a design. The schema holds twenty-four tables and not one of
them is about a running process, so there is nowhere for a second server to be seen from.

The work counts have the same problem from the other end. A reader who wants to know what the installation
holds has to run `tix doctor` for the store, `tix connection ls` for one server's sockets, `tix stats` for
throughput and `tix webhook deliveries` for the queue, and then assemble the answer by hand.

`tix doctor` is not the place for this. Doctor answers "is my installation sound" and exits non-zero when a
check fails. "What exists and what is running" is a different question with a different failure mode: a
second machine being down is not a fault in the installation this command was run against, and it must not
decide the exit code of the command an operator runs to find out about it.

## What Changes

- **A server registers itself in the store.** `tix serve` writes a row when it starts, refreshes it on a
  ticker beside the lease sweeper, the webhook dispatcher and the retention pruner, and removes it on a
  graceful shutdown. The row carries what a reader needs and nothing that belongs to a tenant.
- **The table is installation-scoped, and reached unscoped.** A server serves every tenant, so its row can
  carry no `tenant_id`. It is the second unscoped thing in the system after the SSH fingerprint lookup, and
  it gets the same treatment: a confinement guard naming the files that may reach it and failing the build
  anywhere else.
- **Staleness is the reader's judgement, never a sweeper's.** A reader treats a row last seen longer ago
  than the threshold as not heartbeating, the way an expired lease is already read. A crash leaves the row
  behind on purpose: a dead server an operator can see is the whole point, and hiding it would need the
  cleanup job the lease design deliberately does not depend on.
- **`tix status` prints the installation.** What version and schema, which store and engine, which servers
  are attached and which have stopped answering, and what work exists. It takes `-o table|json|yaml` like
  every other command, and its JSON shape is a contract.
- **It works with no server at all.** The common single-user case opens the database directly, finds no
  rows and says so. That is the correct answer, not an error.
- **The browser gets the same screen**, and the HTTP API the same report.

## Impact

- `internal/store/migrations/0008_servers.sql`: one new table, the first with no tenant column since the
  initial schema.
- `internal/store/store.go`, `internal/store/{sqlite,postgres}/server.go`: five methods on `UnscopedTx`.
- `internal/store/sql/builder.go`: `servers` joins the tables the builder knows carry no tenant predicate.
- `internal/core`: `Server`, `StatusReport` and the staleness rule. Stdlib only.
- `internal/presence`: the registrar and its heartbeat loop, plus the confinement guard.
- `internal/service`: `Status`, the one new `core.Service` method.
- `internal/authz`: `server.read`, on the existing tenant administration scope.
- `internal/server`: the registrar runs as a worker beside the other three.
- `cmd/status.go`, `internal/client`, `internal/httpapi`, `internal/web`: the four surfaces.
- `internal/capability/registry.go`: one operation, bound on the command line, the API and the browser,
  recorded as a terminal gap. The registry grows from 88 operations to 89 and the terminal gap count from
  31 to 32, so `docs/tui.md` states both new figures.
- `docs/commands.md`, `docs/deployment.md`, `docs/scripting.md`: the command, the registration and the
  JSON contract.
