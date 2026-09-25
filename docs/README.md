# Documentation

Guides for using and operating tix. [`openspec/`](../openspec/) is the normative behaviour contract and the
[README](../README.md) is the front door; these pages are the part in between, explaining how to actually work
with the thing.

`tix <command> --help` documents every flag, and `tix docs` emits the whole command tree as Markdown.

## Guides

| Guide | Covers |
| --- | --- |
| [agents.md](agents.md) | Claiming work, leases and lease tokens, `tix claim exec`, token scopes, exit codes, why a zombie cannot write |
| [workflows.md](workflows.md) | Custom state machines, transitions, terminal states, custom fields, indexed versus scanned filters |
| [configuration.md](configuration.md) | Contexts, the five layers, `TIX_*` variables, per-directory discovery |
| [tenancy.md](tenancy.md) | Tenants, domains, roles, and how isolation is enforced |
| [api.md](api.md) | The HTTP API, the WebSocket event stream, and resuming with `since_seq` |
| [filtering.md](filtering.md) | The filter expression language: negation, weak matching, and what each engine does |
| [scripting.md](scripting.md) | NDJSON everywhere, output formats, piping, exit codes |
| [web-ui.md](web-ui.md) | The browser interface: lease badges, multi-step moves, the directory, per-browser preferences |
| [deployment.md](deployment.md) | Running `tix serve`, TLS, reverse proxies, the non-loopback bind guard |
| [scaling.md](scaling.md) | SQLite versus PostgreSQL, keyset pagination, partitioning, when to move |
| [migrating.md](migrating.md) | Importing from Jira and OpenProject, mapping files, snapshots, bundles |
| [shell-completion.md](shell-completion.md) | Installing completions for bash, zsh and fish |
| [upgrading.md](upgrading.md) | `tix update`, what the checksum proves, and what it refuses to replace |
| [theming.md](theming.md) | Tenant accents, built-in and custom themes, what is deliberately not themed |
| [statistics.md](statistics.md) | Throughput, lead time, ageing, and what the leaderboard actually counts |
| [testing.md](testing.md) | Running the suite, what a partial run announces, the coverage floors |

## Where to start

- Running an AI agent against tix: [agents.md](agents.md), then [scripting.md](scripting.md).
- Building a queue view to replace a Jira filter: [filtering.md](filtering.md).
- Setting up a team: [configuration.md](configuration.md), [workflows.md](workflows.md),
  [deployment.md](deployment.md).
- Coming from another tracker: [migrating.md](migrating.md).
- Building a client: [api.md](api.md).
- Showing tix to somebody: `tix demo seed` and [web-ui.md](web-ui.md).
- Changing the code: [AGENTS.md](../AGENTS.md), then [testing.md](testing.md).

## Elsewhere

| Document | Contents |
| --- | --- |
| [openspec/changes/tix-v1/specs/](../openspec/changes/tix-v1/specs/) | The normative behaviour contract |
| [AGENTS.md](../AGENTS.md) | Coding standards and architecture invariants |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | Build, test and pull request process |
| [SECURITY.md](../SECURITY.md) | Reporting a vulnerability, and deployment notes |
