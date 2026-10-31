# Tasks

## 1. Core and store

- [x] 1.1 `internal/core`: `DeleteProjectInput`, `ProjectDeletion`, `StatusMove`; `DeleteProject` takes the input
      and returns the result; `PlanProjectDeletion`
- [x] 1.2 `internal/store`: `MoveTask` on SQLite and PostgreSQL, with a conformance test

## 2. Service

- [x] 2.1 `internal/service/projectdelete.go`: resolution, plan, status mapping, move, delete with tasks, audit and
      events per task, authority on the target
- [x] 2.2 Tests for refusal, empty project, delete path, move path (numbers, relations, mapping, soft deleted,
      scoped tags), authority, and the plan

## 3. Surfaces

- [x] 3.1 `cmd/project.go`: `--move-tasks-to`, `--delete-tasks`, `--dry-run` plan
- [x] 3.2 `internal/httpapi`, `internal/client`, `internal/wire`: parameters, `200` result, plan route
- [x] 3.3 `internal/web`: confirmation screen, danger zone link, preview redirect, typed key check, tip
- [x] 3.4 `internal/i18n`: Bulgarian entries for every new string
- [x] 3.5 `internal/tui`: tasks row on the removal form, plan fetched before the confirmation
- [x] 3.6 `internal/capability`: `project.delete-plan` entry
- [x] 3.7 Callers: demo seed and any other `DeleteProject` caller pass a choice

## 4. Docs

- [x] 4.1 `docs/api.md`, `docs/scripting.md`, `docs/web-ui.md`, `docs/tui.md`, `skills/tix/SKILL.md`
