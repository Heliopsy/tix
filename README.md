# tix

Task management for humans and AI agents, in one binary.

People get boards, workflows, and comments. Agents get an atomic queue pop, a lease that
returns work to the queue when a worker dies, machine-readable output everywhere, and a
durable event stream. Both work the same tasks in the same store.

> Status: in development. The behaviour contract lives in `openspec/`; see
> [`openspec/changes/tix-v1/`](openspec/changes/tix-v1/).

## Why

Existing trackers assume an interactive human. There is no atomic "give me the next
unblocked task", no lease that expires when a worker crashes, and no structured way to
write a result back. An agent that dies holding a ticket leaves it assigned and stalled
until somebody notices.

tix starts from the assumption that a worker may be a process:

- **Claiming is a compare-and-swap** with a TTL lease. A dead agent's task becomes
  available again on its own, with no supervisor.
- **A lease token** is required to renew, transition, or release. A zombie worker whose
  task was re-claimed cannot corrupt the new holder's work.
- **Every mutation emits a durable event**, so other workers can subscribe and a
  reconnect resumes without gaps.
- **Everything is scriptable** and speaks JSON.

## Access paths

One binary, one service layer, five ways in:

| | |
| --- | --- |
| CLI | UNIX-composable, `-o table\|json\|yaml`, meaningful exit codes, zero config |
| TUI | terminal board for interactive use |
| HTTP API | `/api/v1`, keyset paginated, consistent error envelope |
| WebSocket | subscribe to events, resume from a cursor |
| Web UI | required to cover 100% of the CLI, enforced by a build-failing test |

## Quick start

```sh
tix task add "buy milk"        # creates the database, tenant, and project on first run
tix task ls
tix claim next                 # an agent grabs the next unblocked task
tix serve                      # HTTP API, WebSocket, and web UI on 127.0.0.1:8080
```

No configuration file is required. Everything is also settable by environment variable
(`TIX_DATABASE_DSN`, `TIX_SERVER`, ...) or a `.env` file.

## Storage

SQLite by default, for local and single-user use. PostgreSQL for anything shared, which
adds row-level security, full-text search, and partitioned event retention. One schema,
one migration path, and the CLI works against either directly or through a server.

## Install

```sh
go install github.com/thereisnotime/tix@latest
```

Binaries and container images are published per release.

## Development

```sh
just build     # bin/tix
just test      # race detector, shuffled
just check     # fast pre-push gate set
just ci        # the entire CI suite locally, in containers
```

Requires Go and [just](https://just.systems/). Linters, scanners, and PostgreSQL run in
containers via podman, so nothing needs installing on the host.

Coding standards are in [AGENTS.md](AGENTS.md). Contribution process is in
[CONTRIBUTING.md](CONTRIBUTING.md). What comes after v1 is in [ROADMAP.md](ROADMAP.md).

## Licence

[AGPL-3.0](LICENSE). Contributions are accepted under the [CLA](CLA.md).
