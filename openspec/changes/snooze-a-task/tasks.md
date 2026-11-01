# Tasks

## 1. Data and contract

- [x] 1.1 `internal/store/migrations/0023_task_snooze.sql`: `snoozed_until`, `snoozed_by`, the index
- [x] 1.2 `internal/core`: `Task.SnoozedUntil`, `SnoozedByActorID`, `SnoozedAt`; `SnoozeInput`; the filter's `Snoozed` and `IncludeSnoozed`; the three events; the service methods
- [x] 1.3 Both store engines: scan and write the columns, `SetSnooze`, `ExpiredSnoozes`, `WakeSnooze`, the snooze predicate, `claim next` skipping snoozed tasks

## 2. Service

- [x] 2.1 `internal/service/snooze.go`: snooze, unsnooze and the wake pass, each with audit and event in one transaction
- [x] 2.2 `ListTasks` hides snoozed tasks unless asked; `ClaimTask` refuses a snoozed task
- [x] 2.3 Import carries the snooze; export already does
- [x] 2.4 `internal/server`: a wake worker beside the lease sweeper

## 3. Filter language

- [x] 3.1 `internal/query`: `is:snoozed`, `is:awake`, `snoozed:true|false|any`, `Matches` with the default
- [x] 3.2 `internal/query/until.go`: presets, durations, dates and timestamps for a wake time

## 4. Surfaces

- [x] 4.1 HTTP: `PUT`/`DELETE /tasks/{ref}/snooze`, `POST /snoozes/wake`, `snoozed=` on listings; client
- [x] 4.2 CLI: `tix task snooze`, `unsnooze`, `wake`; `--snoozed`, `--include-snoozed`; `task show` names the wake time
- [x] 4.3 TUI: `b` to snooze or unsnooze, the `zz` marker, the detail line
- [x] 4.4 Web: task screen, lane card menu and list row, presets in the reader's zone, the badge
- [x] 4.5 Capability registry entries and web exemption for the wake pass

## 5. Guards

- [x] 5.1 Service, both stores, query, HTTP, CLI, TUI, server worker and transport matrix tests
- [x] 5.2 Web handler and template tests

## 6. Documentation

- [x] 6.1 `docs/filtering.md`, `docs/api.md`, `docs/features.md`, `docs/tui.md`, `docs/agents.md`
- [x] 6.2 `docs/web-ui.md`
- [x] 6.3 `skills/tix/SKILL.md` and `skills/tix/reference/queries.md`
