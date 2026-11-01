# Tasks repeat when they are done

## Why

Plenty of work comes back: the weekly report, the monthly invoice run, the
certificate that needs rotating every ninety days. Today the only way to track
it in tix is to remember to create the next one by hand, which is exactly the
kind of thing a tracker is supposed to remember for you.

The model people already know is Google Tasks: a task carries a repeat rule,
and finishing it puts the next one on the list. Nothing is generated ahead of
time, so there is no calendar of phantom tasks to page through, and the next
occurrence exists only once the current one is actually done.

## What Changes

- A task MAY carry a **recurrence rule**, written in a short human syntax
  such as `every week`, `every 2 weeks on mon,thu`, `every month on day 31`,
  `every 3 days after done`, with an optional end, `until 2027-06-30` or
  `for 5 times`. `internal/core` parses it strictly and renders it back in one
  canonical spelling.
- When a recurring task moves into a **done** state of its workflow, the
  service creates the **next occurrence in the same transaction**: same
  project, title, description, priority, assignee, parent, tags and custom
  field values, the workflow's initial state, and a due date advanced by the
  rule. The rule travels to the new task; the finished task keeps it and
  records which task succeeded it, so it can never spawn twice.
- Moving into a **cancelled** state does not spawn. Neither does creating a
  task already done, importing a snapshot, or an external sync.
- Every surface reaches it: `tix task add --repeat`, `tix task edit
  --repeat/--no-repeat`, the `recurrence` field on the HTTP API and in JSON
  output, a Repeat field in the browser's add and edit forms, a repeat marker
  on list rows, board cards and terminal cards, and an `is:recurring` filter
  term.
- Export carries the rule and the successor link, so a snapshot round-trips.

## Impact

- `internal/core`: `recurrence.go` (rule type, parser, next-date arithmetic),
  `Task.Recurrence`, `Task.RecurrenceNextID`, the create and update inputs,
  `TaskFilter.Recurring`, and `WorkflowDefinition.CompletesWork`.
- `internal/store/migrations/0013_task_recurrence.sql`: two columns on
  `tasks`, both engines through the existing translator.
- `internal/store/sqlite`, `internal/store/postgres`: read, write and filter.
- `internal/service`: validation on create and update, the spawn on
  transition and on a release that finishes the task, the import path.
- `internal/query`: `is:recurring`. `internal/httpapi`, `internal/client`:
  the `recurring` list parameter.
- `cmd/task.go`, `internal/output`: the flags and the rendered field.
- `internal/web`, `internal/tui`: the form fields and the marker.
- `docs/`, `README.md`, `skills/tix/SKILL.md`.

No new `Service` method and so no new capability registry entry: the rule is a
field on operations that already exist on every surface.

**Migration.** `0013`. A sibling branch may also claim 0013; the runner refuses
two migrations sharing a number (`migrations.allFrom`), so whichever lands
second renumbers.
