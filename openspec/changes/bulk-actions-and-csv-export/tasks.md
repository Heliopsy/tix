# Tasks

## 1. Spec

- [ ] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Contract and service

- [ ] 2.1 `core.BulkTaskInput`, `core.BulkResult`, `TaskService.BulkTasks`
- [ ] 2.2 `Service.BulkTasks`: per-task calls, outcomes, up-front authority, cap and dedupe

## 3. Surfaces

- [ ] 3.1 Capability registry entry `task.bulk` with the TUI exemption
- [ ] 3.2 HTTP `POST /v1/tasks/bulk` and client method
- [ ] 3.3 CLI `tix task bulk`; many refs on `task edit`, `task copy`, `tag rm`, `tag add`
- [ ] 3.4 Web selection, action bar, bulk route, toast with refusals, batch Undo, i18n
- [ ] 3.5 CSV writer with formula guard; `tix task ls -o csv`; API `format=csv`; web Export CSV

## 4. Docs and tests

- [ ] 4.1 features, commands, web-ui, api, filtering or export docs, agent skill
- [ ] 4.2 Service tests on both engines; HTTP, CLI, web and JS tests; mutation-checked guards
- [ ] 4.3 `just check` passes
