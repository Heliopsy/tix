# Tasks

## 1. Spec

- [ ] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [ ] 2.1 Migration 0016 `tasks.copied_from_id` on both engines
- [ ] 2.2 Store task columns read and write `copied_from_id` on SQLite and PostgreSQL

## 3. Contract and service

- [ ] 3.1 `core.CopyTaskInput`, `core.Task.CopiedFromID`, `TaskService.CopyTask`
- [ ] 3.2 `Service.CopyTask`: one transaction, audit and event per task, options, authority
- [ ] 3.3 Snapshots carry `copied_from_id`

## 4. Surfaces

- [ ] 4.1 Capability registry entry `task.copy`
- [ ] 4.2 HTTP route and client method
- [ ] 4.3 CLI `tix task copy`, "Copied from" in `tix task show`
- [ ] 4.4 Web "Copy to…" action, form page and dialog, toast, "Copied from" on the About card, i18n
- [ ] 4.5 TUI `Y` key and copy form

## 5. Docs and tests

- [ ] 5.1 features, commands, web-ui, tui, api, README, agent skill
- [ ] 5.2 Service tests on both engines; HTTP, CLI, web and TUI tests; mutation-checked guards
- [ ] 5.3 `just check` passes
