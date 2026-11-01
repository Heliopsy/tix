# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [x] 2.1 Migration 0017 `tasks.copied_from_id` on both engines
- [x] 2.2 Store task columns read and write `copied_from_id` on SQLite and PostgreSQL

## 3. Contract and service

- [x] 3.1 `core.CopyTaskInput`, `core.Task.CopiedFromID`, `TaskService.CopyTask`
- [x] 3.2 `Service.CopyTask`: one transaction, audit and event per task, options, authority
- [x] 3.3 Snapshots carry `copied_from_id`

## 4. Surfaces

- [x] 4.1 Capability registry entry `task.copy`
- [x] 4.2 HTTP route and client method
- [x] 4.3 CLI `tix task copy`, "Copied from" in `tix task show`
- [x] 4.4 Web "Copy to…" action, form page and dialog, toast, "Copied from" on the About card, i18n
- [x] 4.5 TUI `Y` key and copy form

## 5. Docs and tests

- [x] 5.1 features, commands, web-ui, tui, api, README, agent skill
- [x] 5.2 Service tests on both engines; HTTP, CLI, web and TUI tests; mutation-checked guards
- [ ] 5.3 `just check` passes
