# Tasks

## 1. Core

- [ ] 1.1 `internal/core/recurrence.go`: `Recurrence`, `ParseRecurrence`, `String`, `Next`, `Successor`
- [ ] 1.2 `internal/core/recurrence_test.go`: syntax, refusals, round trip, month end, leap year, weekdays, DST
- [ ] 1.3 `internal/core/types.go`: `Task.Recurrence`, `Task.RecurrenceNextID`, `WorkflowDefinition.CompletesWork`
- [ ] 1.4 `internal/core/input.go`, `filter.go`: create and update inputs, `TaskFilter.Recurring`

## 2. Storage

- [ ] 2.1 `internal/store/migrations/0013_task_recurrence.sql`
- [ ] 2.2 `internal/store/sqlite/task.go`, `internal/store/postgres/task.go`: read, write, filter
- [ ] 2.3 Store tests for both engines

## 3. Service

- [ ] 3.1 Create and update validate and canonicalise the rule
- [ ] 3.2 `TransitionTask` and a finishing release spawn the next occurrence in the same transaction
- [ ] 3.3 Cancel, create-as-done, import and sync do not spawn; a linked task never spawns again
- [ ] 3.4 Snapshot import and export carry the rule and the link
- [ ] 3.5 Service tests, including the event pair and a forced failure rolling both back

## 4. Filter and transports

- [ ] 4.1 `internal/query`: `is:recurring` parse and match
- [ ] 4.2 `internal/httpapi`, `internal/client`: `recurring` list parameter

## 5. Surfaces

- [ ] 5.1 `cmd/task.go`: `--repeat`, `--no-repeat`, show and list output
- [ ] 5.2 `internal/web`: Repeat field on the edit and add forms, marker on rows and cards, rule and successor on the task screen
- [ ] 5.3 `internal/tui`: card marker and detail line

## 6. Documentation

- [ ] 6.1 `docs/features.md`, `docs/commands.md`, `docs/filtering.md`, `docs/api.md`, `docs/web-ui.md`, `docs/tui.md`
- [ ] 6.2 `README.md` feature list, `skills/tix/SKILL.md`

## 7. Gates

- [ ] 7.1 Mutation checks: no double spawn, same transaction, cancel does not spawn, due advance, export round trip
- [ ] 7.2 `just check`
