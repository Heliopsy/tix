## Why

Humans and AI agents need to work the same task queue, and today they cannot.

People use Jira, OpenProject, or a notes app. Agents use ad-hoc JSON files, a scratch directory,
or nothing at all. The two never share state, so an agent cannot see what a person already did,
a person cannot see what an agent is working on, and any coordination is manual copy-paste that
drifts within hours.

Bolting agents onto an existing human tracker does not work either. Those systems assume an
interactive human: there is no atomic "give me the next unblocked task" primitive, no lease that
returns work to the queue when a worker dies mid-task, no machine-readable output on every
operation, and no way for a worker to write a structured result back. An agent that crashes while
holding a Jira ticket leaves that ticket assigned and stalled until a person notices.

tix starts from the assumption that a worker may be a process. Claiming is a compare-and-swap with
a TTL lease, so a dead agent's task becomes available again on its own. Every operation is
scriptable and emits JSON. Every mutation produces a durable event that other workers can subscribe
to. Humans get the boards, workflows, and custom fields they expect, over exactly the same data.

## What Changes

A new Go project, `tix`, distributed as one binary that provides five access paths over one service
layer:

- **CLI** — UNIX-composable, `-o table|json|yaml`, meaningful exit codes, works with zero configuration
  against a local SQLite file.
- **TUI** — a terminal board for interactive human use.
- **HTTP REST API** — the same operations, for remote clients and agents.
- **WebSocket event stream** — subscribe to task events with gap-free resume after reconnect.
- **Web UI** — required to cover 100% of CLI/TUI/API functionality, enforced by a capability registry
  test rather than by promise.

Core behaviour being introduced:

- **Claim/lease for agents**: atomic compare-and-swap claim, `claim next` that pops the highest-priority
  unblocked task, an opaque lease token required for renew/transition/release so a zombie worker cannot
  corrupt re-claimed work, and lazy expiry so correctness never depends on a sweeper running.
- **Configurable workflows**: per-project state machines with allowed transitions, plus typed custom
  field definitions. Sensible defaults mean a fresh install needs no configuration.
- **Multi-tenancy**: tenants own projects, workflows, and tags; domains map to tenants; isolation is
  structural (scoped query builder, lint ban on raw queries, reflection test, PostgreSQL row-level
  security) rather than a filter anyone has to remember.
- **Dual transport over one implementation**: the CLI may talk directly to the database or to a remote
  server, and both paths execute the identical service code. Business rules exist in exactly one place.
- **Transactional outbox**: every mutation writes domain rows, an audit entry, and an event in one
  transaction, which is what lets a direct-database CLI write feed every connected WebSocket.
- **Operational surface**: outgoing webhooks with HMAC signatures and retries, append-only audit log,
  configurable retention, JSON/YAML import/export, and layered configuration (flags > env > .env >
  config file > defaults) with named contexts and per-directory auto-detection.
- **External import**: one-way import and refresh from Jira, OpenProject, and a generic CSV/JSON format.
  Their statuses map onto tix workflows and their fields onto tix custom fields, so imported data is
  native rather than second-class. Every imported record keeps an external identity reference, which
  makes re-import an update rather than a duplicate and is the foundation for bidirectional sync in v2.

Storage is SQLite for local and development use and PostgreSQL for shared deployments, against one
schema and one migration path. The design target is 1M+ tasks.

## Capabilities

### New Capabilities

- `data-model`: Entities, relationships, schema, migrations, identifiers, soft delete, optimistic concurrency.
- `multi-tenancy`: Tenants, memberships, the structural isolation guarantee and its enforcement layers.
- `domains`: Hostname-to-tenant routing, domain verification, per-domain TLS configuration.
- `storage-engines`: SQLite and PostgreSQL configuration, dialect abstraction, connection pooling, health checks.
- `configuration`: Named contexts, the layered precedence chain, environment mapping, .env, discovery, first-run bootstrap.
- `service-layer`: The Service contract, error taxonomy, actor and tenant context, transaction boundaries.
- `transports`: Local versus remote resolution, zero-config behaviour, transport equivalence.
- `workflows`: State machines, transition validation, custom field definitions and typing.
- `task-management`: Task CRUD, subtasks, dependencies, tags, comments, artifacts, filtering, keyset pagination.
- `claim-lease`: Claim semantics, claim next, lease tokens, renewal, release, expiry, the exec wrapper.
- `auth`: No-auth mode, users, passwords, sessions, API tokens, scopes, roles, the single enforcement point.
- `cli`: UNIX compliance, exit codes, output formats, composability, shell completion.
- `http-api`: Routes, request and response shapes, error envelope, pagination, health endpoints.
- `event-stream`: Outbox semantics, event taxonomy, WebSocket protocol, resume, subscription filters.
- `webhooks`: Endpoint configuration, HMAC signing, retry schedule, delivery states, redelivery.
- `audit-log`: Append-only guarantees, before/after capture, source attribution, history rendering.
- `retention`: Per-tenant retention policy, pruning, partition management, subscriber-cursor safety.
- `import-export`: Snapshot format, merge versus replace, identifier remapping, round-trip fidelity.
- `external-sync`: One-way import and refresh from Jira, OpenProject, and a generic CSV/JSON format; external identity mapping; field and status mapping; the seam for bidirectional sync in v2.
- `web-ui`: Every screen, the parity requirement as a testable requirement, no-JS-build constraint.
- `tui`: Views, keybindings, live updates, transport agnosticism.
- `server`: Serve lifecycle, TLS, bind safety, graceful shutdown, background workers.

### Modified Capabilities

None — this is a new project.

## Impact

- New repository `github.com/thereisnotime/tix`; no existing code is affected.
- New direct dependencies: `spf13/cobra`, `jedib0t/go-pretty/v6`, `gopkg.in/yaml.v3`,
  `modernc.org/sqlite`, `jackc/pgx/v5`, `coder/websocket`, `golang.org/x/crypto`,
  `charmbracelet/bubbletea`, `charmbracelet/bubbles`, `charmbracelet/lipgloss`.
- `modernc.org/sqlite` is required rather than a cgo driver so builds stay `CGO_ENABLED=0` and the
  cross-compilation matrix keeps working.
- Operators gain a deployable server with a network surface; it binds loopback by default and refuses
  a non-loopback bind without TLS or an explicit opt-out.
- Licensed AGPL-3.0 with a contributor licence agreement.
