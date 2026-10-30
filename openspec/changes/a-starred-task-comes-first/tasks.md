# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [x] 2.1 Migration 0013 `task_stars` (both engines), scoped table list, PostgreSQL policy version
- [x] 2.2 Store methods `StarTask`, `UnstarTask`, `StarredTasks` on SQLite and PostgreSQL
- [x] 2.3 Task listing joins the reader's stars, orders by the star rank first, and carries it in the keyset
- [x] 2.4 `is:starred` predicate in both engines

## 3. Contract and service

- [x] 3.1 `core.Task.Starred`, `core.TaskFilter.Starred` / `StarredBy`, `core.Cursor.Starred`, event types
- [x] 3.2 `authz.ActionTaskStar`
- [x] 3.3 `Service.StarTask` / `UnstarTask`: one transaction, audit and event, idempotent no-op
- [x] 3.4 `ListTasks` sets the reader and the cursor's starred state; `GetTask` reports `starred`
- [x] 3.5 Filter language `is:starred`, `is:unstarred`, `starred:`; in-memory match

## 4. Surfaces

- [x] 4.1 Capability registry entries `task.star`, `task.unstar`
- [x] 4.2 HTTP routes, `starred` list parameter, client methods
- [x] 4.3 CLI `tix task star|unstar`, `--starred`, list marker, show line
- [x] 4.4 Web star toggle on list rows, board cards and the task screen; starred first
- [x] 4.5 TUI `*` key, card marker

## 5. Docs and tests

- [x] 5.1 features, commands, web-ui, tui, filtering, api, README, agent skill
- [x] 5.2 Store pagination across the starred boundary on both engines; per-actor and tenant isolation
- [x] 5.3 Service, HTTP, CLI, web and TUI tests; mutation-checked guards
- [x] 5.4 `just check` passes
