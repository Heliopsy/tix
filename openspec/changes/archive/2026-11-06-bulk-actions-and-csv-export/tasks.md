# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Contract and service

- [x] 2.1 `core.BulkTaskInput`, `core.BulkResult`, `TaskService.BulkTasks`
- [x] 2.2 `Service.BulkTasks`: per-task calls, outcomes, up-front authority, cap and dedupe

## 3. Surfaces

- [x] 3.1 Capability registry entry `task.bulk` with the TUI exemption
- [x] 3.2 HTTP `POST /v1/tasks/bulk` and client method
- [x] 3.3 CLI `tix task bulk`; many refs on `task edit`, `task copy`, `tag rm`, `tag add`
- [x] 3.4 Web selection, action bar, bulk route, toast with refusals, batch Undo, i18n
- [x] 3.5 CSV writer with formula guard; `tix task ls -o csv`; API `format=csv`; web Export CSV

## 4. Docs and tests

- [x] 4.1 features, commands, web-ui, api, filtering or export docs, agent skill
- [x] 4.2 Service tests on both engines; HTTP, CLI, web and JS tests; mutation-checked guards
- [x] 4.3 `just check` passes
