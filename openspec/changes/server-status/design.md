# Design

## Why the table cannot be tenant-scoped

Every other table in tix carries `tenant_id`, every query is built by the scoped builder in
`internal/store/sql`, and `AGENTS.md` states both as invariants. `servers` breaks the first and therefore
the second, and it has to.

A server process is not owned by a tenant. One `tix serve` answers requests for every tenant the database
holds: the HTTP router resolves the tenant per request from the Host header or the token, and the same
process serves the next request for a different tenant a millisecond later. There is no value that could go
in a `tenant_id` column. Writing the tenant that happened to be configured as the default would be a lie
that reads as truth, and the first thing anybody would do with it is filter by it, which would then hide
servers from the tenants they are serving.

The precedent is already here and this change follows it rather than inventing a second shape.
`FindSSHKeysByFingerprint` reaches the store through `store.UnscopedTx` because an SSH client proves a key
before any tenant is known, so there is no scope to build with yet. `servers` is the same kind of exception
for a different reason: not "the tenant is not known yet" but "there is no tenant". Both are installation
facts, and the store already has exactly one door for installation facts.

What does *not* follow is treating this as a licence. The confinement requirement below is the price:
the door is narrow, it is named, and a test fails the build if anything else walks through it.

## What a row may carry, and what it may not

| Column | Why a reader needs it |
| --- | --- |
| `id` | tells two servers apart; generated with `internal/id` like every other identifier |
| `address` | the advertised listen address, so an operator can reach the process |
| `version` | which build, so a rolling upgrade is visible as a version skew |
| `surfaces` | which of `api`, `ws`, `web`, `ssh` this process serves |
| `started_at` | uptime |
| `last_seen_at` | the heartbeat, and therefore whether it is attached |

Nothing else. In particular: no tenant identifier, no tenant key, no hostname that resolves to a tenant, no
actor, no connection breakdown, no count of anything a tenant owns. The rule is stated as a requirement and
asserted by a test that reflects over the struct rather than reviewing the migration, because a column
added later is exactly the failure mode the tenant-isolation suite cannot see: that suite is hand-written
method by method and a new table is invisible to it.

`surfaces` is a comma-joined `TEXT` rather than JSON. The Postgres translation maps a column to `JSONB`
only when `schema.go` names it, and a short closed set of four words does not need a document type to hold
it or two scanners to read it.

`address` is the address the server advertises, which is what it was told to listen on. A server bound to
`0.0.0.0:8080` advertises that, because inventing a reachable address it was never given would be a guess,
and a guess in this column is worse than the literal truth.

## What row-level security means for a table that has none

On PostgreSQL every tenant-scoped table carries `ENABLE`/`FORCE ROW LEVEL SECURITY` and a
`tix_tenant_isolation` policy whose predicate reads `current_setting('tix.tenant_id')`, and
`internal/store/postgres/tx.go` sets that setting per transaction with `set_config(..., true)` so it cannot
outlive the transaction. The policies are emitted by `rowLevelSecurity(version)`, which walks
`sqlb.ScopedTables()`.

`servers` is deliberately absent from `sqlb.ScopedTables()`, so:

- **No policy is created for it, and RLS is never enabled on it.** That is the correct outcome, not an
  oversight. A policy over `tenant_id = current_setting('tix.tenant_id')` cannot be written for a table with
  no `tenant_id`, and enabling RLS with no policy while `FORCE ROW LEVEL SECURITY` is on would make the
  table return nothing to anybody, including the server trying to register itself.
- **It therefore needs no escape hatch.** `FindSSHKeysByFingerprint` needs the `tix.ssh_auth` flag and its
  own `SELECT`-only policy precisely because `ssh_keys` *is* scoped and forced, so an unscoped read there
  returns zero rows rather than failing loudly. `servers` has no policy to get past, so it reads the same on
  both engines with the same statement, which is one fewer engine difference rather than one more.
- **`tenants` and `users` already sit in this category.** This is not a new class of table; it is a third
  member of an existing one. `tenant_domains` was named here too when this was written, and that was
  wrong: it carries `tenant_id NOT NULL REFERENCES tenants(id)` and has since migration 1. It is now
  scoped, forced and policied like any other tenant-owned table, and its one cross-tenant read goes
  through an escape hatch of the `ssh_keys` shape.
- **The `tix_app` role still applies.** `grantAppRole` runs after the migrations at store open and grants
  on `ALL TABLES IN SCHEMA public`, so a table created by migration 8 is grantable by the time a
  transaction enters the role.

The guard for this is the mirror of `TestEveryTenantTableIsIsolated`: a test asserting `servers` has
row-level security *disabled* and carries no isolation policy. Without it, somebody adding
`servers` to `ScopedTables()` in good faith would produce a table nothing can read, and the failure would
appear on PostgreSQL only.

## Who may read what, and why the answer is in two halves

The report has two halves with two different authorization questions, and collapsing them would leak.

**The installation half** — version, schema, store engine, the server list, the tenant count — is
installation-level. It needs `tenant:admin`, through a new `server.read` action. No new scope is minted:
`tenant:admin` is already what administration of the installation means here, and `retention.write` already
maps onto it. A reader without it gets `403`, not a thinner report, because a partial answer to "what is
running" is the kind of half-truth the capability registry records limitations for.

**The work half** — projects, tasks, claims, expired leases, webhook queue — is read inside the caller's
own tenant scope, through the ordinary scoped path, and reports that tenant only. Tenant A's administrator
running `tix status` sees tenant A's task count. This is the same data `tix stats` already returns and is
subject to the same isolation.

The tenant count is the one figure that crosses the line, and it is defined as *the number of tenants this
reader can already see through `ListTenants`*, not the number of rows in `tenants`. That keeps it exactly as
disclosive as a capability the reader already has, and no more. An installation-wide count would tell an
administrator of one tenant that six others exist, which nothing else in the product tells them, and it
would do it from a report they ran to count their own tasks.

This is not theoretical. The first implementation read the count through the unscoped face of the
transaction, because the table it is counting has no tenant column to scope by and the unscoped read was
right there. `ListTenants` turns out to return the session's own tenant and nothing else, so on a store
holding two tenants the report said two where the reader could list one. The test asserting the two agree
is what found it, which is the argument for pinning a figure to an existing capability rather than to a
table.

The server rows themselves disclose nothing about tenants by construction, which is what the "a row carries
no tenant-identifying data" requirement is for: it is what allows the server list to be shown to a
tenant-admin at all.

## Lazy expiry, because a crash is the case that matters

The lease design already got this right and is copied rather than reinterpreted. `lease_expires_at` is
authoritative the moment it passes; the sweeper exists to tidy rows and to emit the event, and a reader that
waited for it would be wrong for up to a sweep interval.

So: a reader computes `now.Sub(last_seen_at) > core.ServerStaleAfter` and calls the server not
heartbeating. No sweeper is consulted and none is required for the answer to be correct.

Staleness is three heartbeat intervals. One missed beat is a busy machine or a slow write; three is
a process that has stopped. At the default 30-second interval that is 90 seconds.

The three are three of *that server's* interval, read from its own row, not three of the default.
`--heartbeat-interval` moves the writing, so a threshold taken from the default is wrong for anybody who
sets it: too short for a server beating every five minutes, which reads a live process as down two and a
half minutes after its last beat, and too long for one beating every second, which reads a dead process as
up for ninety. A row written before the cadence was recorded declares none, and is judged against the
default, which is exactly what it was judged against before.

A graceful shutdown deletes the row, so a planned stop leaves nothing behind. A crash, a `SIGKILL`, a power
cut and a partitioned network all leave the row, and the reader judges it stale. That asymmetry is the
design: the output says `not heartbeating` rather than quietly omitting the server, because an operator
looking for the second server needs to be told it is gone, not shown a list that silently became shorter.

Rows are not kept forever. A registrar deletes rows unseen for `core.ServerForgetAfter` (seven days) as it
registers, so an installation that cycles through identifiers does not accumulate them. This is hygiene and
correctness does not rest on it: a row that outlives the purge still reads as stale, and a purge that never
runs changes nothing a reader sees in the first week.

## Why a heartbeat writes no audit entry and no event

`AGENTS.md` requires every mutation to write its domain rows, its audit entry and its outbox event in one
transaction. A heartbeat is not a mutation in that sense, and the reason is structural before it is about
volume.

`audit_entries` and `events` are tenant-scoped tables. They carry `tenant_id`, they are under row-level
security on PostgreSQL, and they are partitioned and pruned per tenant. A heartbeat has no tenant. Writing
one would mean choosing a tenant to attribute process liveness to, which is the same lie the `tenant_id`
column would have been, and it would put a row a tenant can read into that tenant's own audit trail
describing infrastructure that is none of their business.

The volume argument stands behind it and is not small. Two servers beating every thirty seconds is 5,760
audit entries and 5,760 events a day, every day, against an installation whose task table may hold a few
thousand rows for its whole life. `docs/retention.md` exists because events outgrow tasks already; this
would make the ratio absurd and would push real history out of a retention window to make room for the
statement that a process is still running.

Registration and deregistration are rarer and the same argument applies to them for the same structural
reason, so they are not audited either. What an operator gets instead is a structured log line at the level
the rest of the server's lifecycle is logged at, and the row itself, which is the record.

This is written down here rather than assumed because the invariant it steps around is a real one. The
narrow reading is the correct one: every *tenant-scoped domain mutation* writes rows, audit and outbox
together. An installation-scoped liveness row has no tenant to audit into, so the invariant does not reach
it, and the place that says so is here.

## Connection counts, and being honest about what is known

A connection is held by one process in memory. `internal/connections` says so in its package comment, and a
row in the store cannot carry a count that is only true inside another process's address space and only
until the next socket closes.

So the report separates two things:

- **Per server, from the store:** identity, address, version, uptime, last seen, surfaces. True for every
  server, however the report was obtained.
- **Per process, from the answering server:** the connection counts `internal/connections` holds. Present
  only when the report came from a server over HTTP, and attributed to exactly one row.

The attribution works because the registrar hands the registered identifier to `connections.Default`, so
the server identifier in a connection listing and the server identifier in a status report are the same
string. Before this change that identifier was a hostname and four random bytes, which told a reader which
process answered but could not be joined to anything.

Every other row reports its connection count as unknown, and the table output prints that as a dash rather
than a zero. A zero would be a claim, and it would be the wrong one.

Against a local database with no server running, there is no answering process, so no row has a count and
the field is absent from the JSON entirely rather than present and zero.

## Why this is not `tix doctor`

`tix doctor` diagnoses *this* installation from *this* machine and exits 1 when a check fails. That is what
makes it usable in a startup script.

`tix status` reports what exists and what is running, and exits 0 whenever it could produce the report. A
server that has stopped heartbeating is information, not a failure of the command: the machine that died is
not the machine the command ran on, and nothing the operator can do here changes it. Making the exit code
depend on another machine being up would mean a monitoring hook could not distinguish "I cannot reach the
database" from "one of four servers is rebooting", and the first is actionable where the second is often
routine.

They stay separate for the ordinary reason two commands stay separate: they have different failure
semantics, and merging them would force one of the two to lose its own.

## The JSON shape, as a contract

```json
{
  "installation": {
    "version": "0.9.0",
    "schema_version": 7,
    "store": "postgres://…/tix",
    "engine": "postgres",
    "observed_at": "2026-09-28T10:00:00Z"
  },
  "servers": [
    {
      "id": "01H…",
      "address": "10.0.0.4:8080",
      "version": "0.9.0",
      "surfaces": ["api", "ws", "web", "ssh"],
      "started_at": "2026-09-22T08:00:00Z",
      "last_seen_at": "2026-09-28T09:59:57Z",
      "heartbeat_interval": "30s",
      "attached": true,
      "connections": 14
    }
  ],
  "work": {
    "tenants": 3,
    "projects": 12,
    "tasks": 8412,
    "claimed": 6,
    "leases_expired_unswept": 2,
    "webhooks_pending": 0,
    "webhooks_failed": 1
  }
}
```

Decisions inside that shape:

- `attached` is a boolean the *reader* computed, not a column. A consumer must not have to know the
  threshold or re-derive it from `last_seen_at`, and two consumers deriving it differently is how two
  dashboards disagree.
- `last_seen_at` and `heartbeat_interval` are present anyway, so a consumer that wants a finer judgement
  can make one against the cadence the server actually keeps.
- `connections` is **omitted**, not null and not zero, for a server whose count is unknown. `omitempty` on
  a pointer says "not known"; a zero would say "none".
- `surfaces` is an array here even though the column is a joined string. The storage shape is not the
  contract.
- `servers` is `[]` and never `null` when there are none, because a consumer iterating it should not have
  to nil-check.
- Every timestamp is RFC 3339 in UTC, as everywhere else in the product.

## Keyset pagination for a list of at most a handful

`AGENTS.md` admits no exceptions, so `ListServers` takes a `core.Page` and pages by keyset on `id` through
the same `resolvePage`/`apply` helpers every other listing uses. The service reads the pages to exhaustion
because a status report is not a paged surface, with a bound on the number of pages so a pathological table
cannot make the command hang. The alternative, a hand-written unpaged `SELECT`, would be the only listing in
the tree that could not be reviewed by the same rule as the others.

## Where the registrar lives

`internal/presence`, beside `internal/lease`, `internal/retention` and `internal/webhook`, which are the
three loops it joins. It owns registration, the ticker, the graceful removal and the purge, and it is a
`server.Worker` like the others, so a panic in it is supervised and restarted rather than silently ending
the only thing that keeps the row fresh.

It does not go through `core.Service`. Registration is not an operation an operator invokes, and putting it
on the service contract would demand CLI, HTTP, web and terminal bindings for a thing no reader should be
able to call: a `POST` that registers a server nobody is running is a way to put a lie in the table. The
webhook dispatcher already reaches the store directly for the same reason.

## Confinement

Five methods reach `servers` without a tenant, and exactly five files may name them:

| File | Why |
| --- | --- |
| `internal/store/store.go` | declares them |
| `internal/store/sqlite/server.go` | implements them |
| `internal/store/postgres/server.go` | implements them |
| `internal/presence/registrar.go` | writes: register, heartbeat, deregister, purge |
| `internal/service/status.go` | reads: list |

`internal/presence/confinement_test.go` asserts this in both shapes, copying
`internal/sshd/confinement_test.go` because one shape catches only one of the two ways the claim stops being
true. Reflecting over `store.UnscopedTx` catches a sixth door being cut — a `ListServersEverywhere` would
compile and be spelled nowhere a search for the existing names looks. Walking the tree catches a sixth
caller reaching through a door that already exists, which reflection cannot see because the interface is
unchanged.
