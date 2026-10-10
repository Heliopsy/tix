## Context

tix is a greenfield Go project. It borrows its repository conventions from `sshroute`
(Go 1.26, Cobra, `just`, OpenSpec, GoReleaser, strict CI) but shares no code: sshroute is a local
CLI with no server component, so every transport here is new.

The product must serve two very different callers over one dataset. A human wants a board, a
workflow, and a comment thread. An agent wants an atomic queue pop, a lease that survives its own
crash, machine-readable output, and a durable event stream. Designing for the agent first and adding
the human affordances on top works; the reverse does not, which is why existing trackers cannot be
adapted.

Target scale is 1M+ tasks, multi-tenant, with tenants addressable by domain.

## Goals / Non-Goals

**Goals:**

- One authoritative implementation of every business rule, reachable from CLI, TUI, HTTP, WebSocket, and web.
- An agent can claim, work, and release a task safely, and its failure is self-healing.
- Zero configuration for single-user local use; full multi-tenant deployment from the same binary.
- Web UI functionally equal to the CLI, enforced mechanically.
- Tenant isolation that cannot be defeated by forgetting a `WHERE` clause.
- Single static binary, `CGO_ENABLED=0`, no JavaScript build toolchain.

**Non-Goals:**

- SSO/OIDC. The schema columns and the `Authenticator` chain exist; no provider ships in v1.
- MCP server mode. The `Service` interface is transport-neutral so it is an adapter later, not a rewrite.
- ACME/automatic certificates. Operators terminate TLS at a proxy or supply certificate files.
- Bidirectional sync with Jira/OpenProject. v1 imports and refreshes one way; the external identity
  table, per-system cursors, and the adapter interface all ship now so v2 adds push rather than a rewrite.
- Real-time collaborative editing, time tracking, billing, sprint/velocity reporting.
- Full-text search parity between engines. PostgreSQL gets `tsvector`; SQLite gets `LIKE`.

## Decisions

### `internal/core` is standard-library only

`core` holds the domain types, the `Service` interface, `TenantScope`, and the error taxonomy. The
local implementation and the HTTP client both depend on `core`; neither depends on the other. Because
`core` imports nothing, accidental coupling between the two transports becomes a compile error rather
than a review comment. This is the mechanical form of "one authoritative code path".

### Two implementations of one interface

`service.Local` owns every rule: validation, authorization, transition legality, audit, event
emission. `client.Client` is pure marshalling with no validation whatsoever. The HTTP handlers wrap a
`*service.Local`, so a request arriving over the network executes the same method a direct-database
CLI would call. Duplicating a rule in the client is therefore never necessary, and a rule that exists
only in the client would be trivially bypassable.

`connect.Open()` is the single place that decides local versus remote, in order: raw `--server`/`--db`
override, explicit `--ctx`, auto-detected per-directory context, `current_context` from config, then a
default SQLite file created and migrated on first use.

**Trade-off:** two transports mean the equivalence has to be proven, not assumed. A dedicated test
table runs every scenario twice — once against `Local`, once against `Client` fronting an httptest
server wrapping the same `Local` — asserting identical results and identical error codes.

### Events are a transactional outbox, not an in-memory bus

Every mutation writes domain rows, an `audit_entries` row, and an `events` row in one transaction.
A server is a reader of that table, never the producer.

This single decision buys three things that an in-memory bus cannot: a CLI writing directly to the
database still feeds every connected WebSocket (a running server's tailer picks the event up within
one poll interval); `since_seq` reconnect is gap-free because history is durable; and webhook delivery
survives process death. The cost is a table that grows with every mutation, which is why `retention`
is a v1 capability rather than an afterthought.

Tailer wake-up on SQLite is a plain re-read: the tailer re-runs its `seq > cursor` query on a 250ms
ticker and sleeps again when nothing came back. There is no change notification involved on that engine.
PostgreSQL adds `LISTEN/NOTIFY`, where a committing transaction announces on a channel the tailer holds a
dedicated connection for. A direct-database `Subscribe` therefore has up to 250ms latency on SQLite and
sees only committed events, which is correct and sufficient for a TUI refresh.

### Webhook delivery outside a server

Delivery rows are claimed with a lock column and drained by whichever process is available: a
dispatcher goroutine when a server runs, or a bounded opportunistic drain in the CLI after commit
(`hooks.mode: inline|server|off`). If nothing drains, rows stay pending until something does —
deliveries are delayed, never lost — and `doctor` reports the backlog. Webhook POSTs always happen
after commit, never inside a write transaction, so network latency can never hold a database lock.

### Claim is a compare-and-swap, with a lease token

Claiming is a single conditional `UPDATE` that succeeds only if the task is unclaimed or its lease has
expired. Zero rows affected means someone else won. This is race-free on both engines with no advisory
locking and, critically, no server — two agents racing against the same SQLite file resolve correctly.

Each claim mints a fresh opaque `lease_token`. Renew, transition, and release all require it. This is
the property that makes agent failure safe: a worker paused past its lease expiry, whose task has been
re-claimed by someone else, cannot renew, cannot transition, and cannot write a result. It is told the
lease expired and must re-claim. Without the token, a stale process would silently overwrite another
worker's progress.

**Lazy expiry is authoritative.** Every read and every claim treats `lease_expires_at <= now` as
unclaimed, so correctness never depends on a sweeper. The sweeper only materializes expiry — clearing
columns, emitting `task.lease_expired`, optionally reverting status. A CLI-only install with no server
is fully correct; it simply gets those events later.

### Custom fields: typed definitions, JSON values

`workflows.definition` is a JSON document; `field_defs` are typed rows; `tasks.custom_fields` is one
JSON column.

The read pattern is always "load a task, render all of its fields", never "join across a dozen
attribute rows". EAV costs a join per field, complicates multi-field filtering and sorting, and doubles
the write path. A JSON column keeps the task row atomic, makes import/export round-trip trivially, and
indexes on both engines — an expression index on SQLite, GIN on PostgreSQL — for fields marked
`indexed`.

**Trade-off, stated plainly:** filtering a non-indexed custom field is a table scan, and the database
cannot enforce custom-field types — validation lives in the service against `field_defs`, so a
hand-edited database can hold values the service would have rejected. `doctor --check-fields` reports
that drift. Hot fields (`status`, `priority`, `assignee`, `due_at`) stay typed columns, so a fresh
install is one default workflow, zero field definitions, and an all-NULL JSON column.

### Multi-tenancy: shared schema, three enforcement layers

`tenant_id` sits on every tenant-owned table and leads every composite index. Not schema-per-tenant
and not database-per-tenant: at 1M+ rows a single well-indexed table set outperforms thousands of
schemas, migrations stay one operation, and connection pooling stays sane.

The cost is that isolation lives in code rather than in the database's namespace. A cross-tenant leak
is the one bug class here that cannot be walked back, so it gets three independent layers:

1. Every statement is built by a helper that requires a `TenantScope`. There is no API to build an
   unscoped query, and a lint rule forbids raw `db.Query`/`db.Exec` outside that helper.
2. A reflection test asserts every generated statement carries a `tenant_id` predicate.
3. PostgreSQL row-level security with a per-transaction `tix.tenant_id` setting. If the first two
   layers are defeated by a bug, the database still refuses.

SQLite has no row-level security, which is an additional reason shared deployments belong on
PostgreSQL; `doctor` says so explicitly. A leak suite seeds two tenants with identical-looking data
and asserts every service method, route, and web handler returns nothing from the other.

Local no-auth mode creates an implicit `default` tenant, so single-user use never encounters the concept.

### Authorization in exactly one place

`authz.Policy.Can(actor, action, resource)` is called only from `service.Local`. The HTTP layer performs
authentication (resolving identity and tenant) and nothing else; the client performs neither. Actor and
tenant travel in `context.Context`, populated differently per transport but consumed identically.

No-auth mode synthesizes a local actor with full scope and still runs the policy — it simply always
allows. Keeping the policy in the path for every mode means there is no untested "auth enabled" branch
waiting to fail the first time someone turns it on.

### Keyset pagination everywhere

Every list endpoint paginates by `(sort_key, id)` cursor. `OFFSET` at depth scans every skipped row and
is the classic reason a tracker becomes unusable once it holds real data. This is stated as a
requirement rather than an implementation note so it cannot quietly regress, and a benchmark job against
a seeded 1M-row fixture catches it if it does.

### Web UI without a JavaScript build step

Server-rendered `html/template`, `go:embed`, vendored htmx, and a few hundred lines of hand-written
JavaScript for the event stream and drag-and-drop. Every form works without JavaScript; htmx makes it
feel live.

A framework would be more pleasant for a Jira-grade board with smooth drag-and-drop and inline rich
editing. The requirement here — move a card, receive a re-rendered column — is well under 200 lines of
the HTML5 drag-and-drop API plus `hx-post`. Avoiding npm keeps the build a single `go build` and the
artifact a single binary. If hand-written JavaScript passes roughly 1500 lines, that is the signal to
revisit; adding a bundler pre-emptively is not.

**Parity is enforced structurally.** `internal/capability` declares every operation once, with its
service method, scopes, and CLI/HTTP/Web/TUI bindings. Three tests then assert: every exported `Service`
method appears in the registry; every operation has CLI, HTTP, and Web bindings; and every binding
resolves against the real mux, the real Cobra tree, and the real embedded templates. A new service
method wired to CLI and API but forgotten in the web UI fails the build. Genuinely inapplicable
operations carry an explicit exemption marker with a justification, so exemptions are reviewed rather
than silent.

### Storage engines

SQLite via `modernc.org/sqlite` — pure Go, because a cgo driver would break `CGO_ENABLED=0` and the
cross-compilation matrix. WAL mode, `busy_timeout`, `BEGIN IMMEDIATE` for write transactions, and a
single-connection writer pool alongside a multi-connection reader pool. SQLite permits one writer at a
time; these settings make concurrent writers serialize rather than fail, and short transactions with no
network I/O inside keep that contention negligible.

PostgreSQL via `pgx/v5` for shared deployments, adding row-level security, `tsvector` search, and
monthly partitioning of `events` and `audit_entries` so retention is a partition drop rather than a mass
delete.

Times are stored as RFC3339 UTC strings on SQLite so lexicographic ordering equals chronological
ordering, which is what lets lease and cursor comparisons use identical SQL on both engines. Dialect
differences are confined to `store/postgres` and `store/sqlite`; placeholder rewriting happens once in
`store/sql`.

Migrations are hand-rolled over `go:embed`: forward-only, one transaction per file, recorded in a
table. Roughly 80 lines, which is less than the configuration a migration library would need.

### Dependencies kept deliberately small

Ten direct dependencies. Explicitly refused: an HTTP router (the standard library's `ServeMux` handles
method and wildcard patterns), a migration library, an ORM or query builder (which would fight the
dual-dialect requirement), a UUID library (~40 lines gives sortable identifiers that also order the
outbox), JWT (opaque revocable tokens in a table avoid an entire class of algorithm-confusion bugs), a
validation library, and any JavaScript toolchain.

### External import: one-way now, bidirectional later

Imports run through one `Importer` interface with three adapters: a generic CSV/JSON format, Jira, and
OpenProject. The generic adapter matters as much as the other two — it means GitHub Issues, Linear, or a
spreadsheet can be brought in by shaping a file, without anyone writing Go.

Mapping is where this succeeds or fails. tix already has configurable per-project workflows and typed
custom fields, so an import does not have to flatten the source: a Jira project's statuses become a tix
workflow, its issue types become a field or separate projects, and its custom fields become
`field_defs`. Mapping is declarative, lives in a YAML mapping file, and is previewable with a dry run
that reports exactly what would be created, updated, and skipped before anything is written.

Every imported entity gets a row in `external_refs`
(`tenant_id, entity_type, entity_id, system, external_id, external_url, external_version, last_synced_at`).
This is the decision that has to be made now rather than later: with it, re-running an import is an
idempotent refresh that updates changed records and leaves the rest alone; without it, the second import
duplicates everything, and retrofitting identity onto already-imported data is guesswork. Per-system
cursors are stored alongside so a refresh fetches only what changed.

Imports are ordinary service calls, so they emit audit entries and outbox events like any other mutation,
and an import is attributed to a `system` actor rather than to whoever ran it.

**Deliberately deferred to v2, with the seams built now:** writes back to the source system, conflict
resolution policy, change-origin tracking to prevent echo loops, and inbound webhooks from Jira and
OpenProject. `external_refs.external_version` exists in v1 and is populated but only read for
change detection — it becomes the conflict-detection field in v2. The `Importer` interface is defined so
that a `Syncer` extending it with `Push` is an additive change.

**Trade-off:** carrying an external identity table and per-system cursors is real schema and code that
v1 does not strictly need for a one-shot migration. It is worth it because "import once and abandon the
old system" and "run alongside the old system" are different products, and the second is what was asked
for.

## Risks / Trade-offs

- **Cross-tenant leak** is the unrecoverable failure. Mitigated by four independent layers; all must land
  with the code they protect, not afterwards.
- **SQLite is the weak corner at scale**: no row-level security, `LIKE` search, single writer. Acceptable
  for local and small deployments, and `doctor` is explicit about when to move. It must refuse a database
  file on a network filesystem outright, because that corrupts rather than degrades.
- **Surface area.** Twenty-one capabilities across five access paths is a large v1. The work is split into
  file-exclusive packages so it can be built in parallel, and into four milestones (local CLI, server, web,
  TUI) that are each independently usable. If schedule pressure arrives, the TUI is the cheapest cut — it
  has no unique capability — followed by collapsing to a single tenant, which is possible without schema
  change precisely because tenancy is structural from the start.
- **Contract churn** would be expensive under parallel development. `core` and the store interfaces are
  frozen early and owned by one package; later changes route through that owner.
- **Coverage against a large surface.** An 80% gate over 60+ command files needs golden tests budgeted from
  the start. The TUI is excluded with a documented reason, since testing `View()` output strings is low
  value; its logic lives in pure functions that are tested.
- **Containerized tooling** pins linter versions in a second place and costs a first-run image build, in
  exchange for a `just ci` that genuinely matches CI and a host with nothing installed on it.
- **AGPL with a CLA** preserves the option to dual-license later, at a measurable cost in drive-by
  contributions.
- **Import fidelity is never total.** Jira and OpenProject have concepts with no tix equivalent (sprints,
  epics-as-link-types, worklogs). The mapping file makes the lossy parts explicit and the dry run shows
  them before import; unmapped fields land in `custom_fields` rather than being silently dropped.
- **External APIs are rate-limited and paginated inconsistently.** Adapters must handle backoff and
  resumable cursors, and a failed import must be safely re-runnable — which the external identity table
  gives for free.
