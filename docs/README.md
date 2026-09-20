# Documentation

Guides for using and operating tix. [`openspec/`](../openspec/) is the normative behaviour contract and the
[README](../README.md) is the front door; these pages are the part in between, explaining how to actually work
with the thing.

`tix <command> --help` documents every flag, and `tix docs` emits the whole command tree as Markdown.

## Guides

| Guide | Covers |
| --- | --- |
| [agents.md](agents.md) | Claiming work, leases and lease tokens, `tix claim --exec`, token scopes, exit codes, why a zombie cannot write |
| [workflows.md](workflows.md) | Custom state machines, transitions, terminal states, custom fields, indexed versus scanned filters |
| [configuration.md](configuration.md) | Contexts, the five layers, `TIX_*` variables, per-directory discovery |
| [tenancy.md](tenancy.md) | Tenants, domains, roles, and how isolation is enforced |
| [api.md](api.md) | The HTTP API, the WebSocket event stream, and resuming with `since_seq` |
| [scripting.md](scripting.md) | NDJSON everywhere, output formats, piping, exit codes |
| [deployment.md](deployment.md) | Running `tix serve`, TLS, reverse proxies, the non-loopback bind guard |
| [scaling.md](scaling.md) | SQLite versus PostgreSQL, keyset pagination, partitioning, when to move |
| [migrating.md](migrating.md) | Importing from Jira and OpenProject, mapping files, snapshots, bundles |
| [shell-completion.md](shell-completion.md) | Installing completions for bash, zsh and fish |

## Where to start

- Running an AI agent against tix: [agents.md](agents.md), then [scripting.md](scripting.md).
- Setting up a team: [configuration.md](configuration.md), [workflows.md](workflows.md),
  [deployment.md](deployment.md).
- Coming from another tracker: [migrating.md](migrating.md).
- Building a client: [api.md](api.md).

## Screenshots

[screenshots/](screenshots/README.md) shows the command line, the terminal
interface and the browser interface over the same store.

## Elsewhere

| Document | Contents |
| --- | --- |
| [openspec/changes/tix-v1/specs/](../openspec/changes/tix-v1/specs/) | The normative behaviour contract |
| [AGENTS.md](../AGENTS.md) | Coding standards and architecture invariants |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | Build, test and pull request process |
| [SECURITY.md](../SECURITY.md) | Reporting a vulnerability, and deployment notes |
