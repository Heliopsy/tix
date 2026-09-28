# Tasks

## 1. Schema and store

- [x] 1.1 `internal/store/migrations/0008_servers.sql`: the table with no tenant column, and the index the
      listing orders on
- [x] 1.2 `internal/store/migrations/migrations_test.go`: name `servers` in `unscopedTables`, which is the
      guard that forces this decision to be made deliberately
- [x] 1.3 `internal/store/sql/builder.go`: `servers` joins the tables the builder knows carry no tenant
      predicate, and stays out of `ScopedTables()` so no isolation policy is emitted for it
- [x] 1.4 `internal/store/store.go`: the five methods on `UnscopedTx`
- [x] 1.5 `internal/store/sqlite/server.go` and `internal/store/postgres/server.go`: both engines, keyset
      paged through the existing page helpers
- [x] 1.6 `internal/store/postgres/rls_test.go`: assert `servers` has row-level security disabled and no
      isolation policy, the mirror of the guard over the scoped tables

## 2. Contract

- [x] 2.1 `internal/core/status.go`: `Server`, `StatusReport`, `StatusService`, the staleness threshold and
      the pure `Attached(now)` judgement. Stdlib only
- [x] 2.2 `internal/core/service.go` and `contract_test.go`: embed the new part and name it in `serviceParts`
- [x] 2.3 `internal/core/status_test.go`: a row carries no tenant-identifying field, asserted by reflection
      over the struct rather than by reading the migration

## 3. Registrar

- [x] 3.1 `internal/presence/registrar.go`: register on start, heartbeat on a ticker, deregister on a
      graceful stop, purge rows nobody has seen for a week
- [x] 3.2 `internal/presence/registrar_test.go`: table-driven, on `clock.Fake`, never `time.Sleep`
- [x] 3.3 `internal/presence/confinement_test.go`: the unscoped server methods are named in exactly five
      files, asserted by reflection over the interface and by walking the tree
- [x] 3.4 `internal/server/assemble.go` and `workers.go`: the registrar runs as a supervised worker beside
      the sweeper, the dispatcher and the pruner, and its shutdown removes the row
- [x] 3.5 The registrar hands its identifier to `connections.Default`, so a connection listing and a status
      report name the same server

## 4. Service and authorization

- [x] 4.1 `internal/authz/action.go`: `server.read` on the tenant administration scope, in all four places
- [x] 4.2 `internal/service/status.go`: authorize, read the servers unscoped and the work counts scoped, in
      one transaction
- [x] 4.3 `internal/service/status_test.go`: the refusal without the scope, the work counts staying inside
      the tenant, the tenant count matching what the reader can list, staleness decided with no sweeper

## 5. Surfaces

- [x] 5.1 `internal/wire/wire.go`: `RouteStatus`
- [x] 5.2 `internal/httpapi/handlers_status.go` and its registration in `router.go`
- [x] 5.3 `internal/client/status.go`: marshal and nothing else
- [x] 5.4 `cmd/status.go`: flags and wiring only, with the table rendered where the prose belongs
- [x] 5.5 `internal/web/status.go`, its route const, its entry in `routes()` and
      `internal/web/templates/status.html`
- [x] 5.6 `internal/capability/registry.go`: one operation, three bindings, one recorded terminal gap
- [x] 5.7 `internal/capability/parity_test.go`: the terminal gap count up from 31 to 32

## 6. Documentation

- [x] 6.1 `docs/commands.md`: regenerate the command table block
- [x] 6.2 `docs/tui.md`: the gap count, the operation count and the administration share
- [x] 6.3 `docs/deployment.md`: what a server registers, what a graceful stop removes, and what a crash
      leaves behind
- [x] 6.4 `docs/scripting.md`: the JSON contract
