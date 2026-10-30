# Tasks

## 1. Core

- [x] 1.1 `internal/core/recurrence.go`: `Recurrence`, `ParseRecurrence`, `String`, `Next`, `Successor`
- [x] 1.2 `internal/core/recurrence_test.go`: syntax, refusals, round trip, month end, leap year, weekdays, DST
- [x] 1.3 `internal/core/types.go`: `Task.Recurrence`, `Task.RecurrenceNextID`, `WorkflowDefinition.CompletesWork`
- [x] 1.4 `internal/core/input.go`, `filter.go`: create and update inputs, `TaskFilter.Recurring`

## 2. Storage

- [x] 2.1 `internal/store/migrations/0013_task_recurrence.sql`
- [x] 2.2 `internal/store/sqlite/task.go`, `internal/store/postgres/task.go`: read, write, filter
- [x] 2.3 Store tests for both engines

## 3. Service

- [x] 3.1 Create and update validate and canonicalise the rule
- [x] 3.2 `TransitionTask` and a finishing release spawn the next occurrence in the same transaction
- [x] 3.3 Cancel, create-as-done, import and sync do not spawn; a linked task never spawns again
- [x] 3.4 Snapshot import and export carry the rule and the link
- [x] 3.5 Service tests, including the event pair and a forced failure rolling both back

## 4. Filter and transports

- [x] 4.1 `internal/query`: `is:recurring` parse and match
- [x] 4.2 `internal/httpapi`, `internal/client`: `recurring` list parameter

## 5. Surfaces

- [x] 5.1 `cmd/task.go`: `--repeat`, `--no-repeat`, show and list output
- [x] 5.2 `internal/web`: Repeat field on the edit and add forms, marker on rows and cards, rule and successor on the task screen
- [x] 5.3 `internal/tui`: card marker and detail line

## 6. Documentation

- [x] 6.1 `docs/features.md`, `docs/commands.md`, `docs/filtering.md`, `docs/api.md`, `docs/web-ui.md`, `docs/tui.md`
- [x] 6.2 `README.md` feature list, `skills/tix/SKILL.md`

## 7. Gates

- [x] 7.1 Mutation checks watched fail, each restored with `git checkout --` and `git diff` empty: the
      successor-link guard removed, done widened to any terminal state, the spawn error swallowed, the
      import dropping the rule and not remapping the link, the release not spawning, the late-finish skip,
      the month clamp, clamping that accumulates, the count not decremented, both web markers, the web
      form clearing an unnamed rule and swallowing a refusal, and both terminal markers. Writing the
      successor in a separate transaction deadlocks the single SQLite writer rather than failing a test,
      so the rollback test is evidence for error propagation inside one transaction, not for that mutation
- [x] 7.2 `just check` green once the smoke test's header was updated (smoke, jstest, spec and docs-check
      re-run after the fix); the PostgreSQL store and connect suites run against a local container
