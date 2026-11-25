# Group lanes by more than the project

## Why

Lanes draw one lane per project. On a project page, which is the task screen narrowed to one project,
that is a single lane holding everything, which is the list drawn sideways. The lanes are most useful
when a lane is a question somebody asks of a project: who has what, what is urgent, what is tagged
for which area, what is due when.

## What Changes

- A **Group lanes by** control, drawn only in the Lanes view, picks what a lane is: Project, Assignee,
  Priority, Tag or Due. It is a GET form, so it works without scripting, and its choice is the `lanes`
  query parameter, kept by the view switch, the filter form, Clear, the pager and the live refresh.
- The task list defaults to Project and remembers the last choice per browser (`tix_lanes_group`)
  the way it remembers the view. A project page defaults to Assignee, never offers Project, and never
  reads or writes the remembered choice.
- Assignee lanes are one per person on the page, plus Unassigned; a person named by an `assignee:`
  term gets a lane even when empty. Priority lanes run highest to lowest. Tag lanes draw each card once,
  in the leftmost drawn lane among its tags, plus No tag. Due lanes are the deadline buckets My work
  uses.
- Status is not offered: lanes by status are the board's columns, and the board already merges
  workflows, refuses ones that disagree and moves cards by drag.
- Arrangements are kept per grouping: `lanes` stays the project lanes, and `lanes:assignee`,
  `lanes:priority`, `lanes:tag` and `lanes:due` are new arrangeable views, on the service, the CLI and
  the HTTP API.
- Lanes have no drag and drop today, so none is added.

## Capabilities

### Modified Capabilities

- `web-tasks`: lanes grouped by assignee, priority, tag and due, the control, its defaults and the
  address.
- `service-layer`: the `lanes:` views and their groups.

## Impact

`internal/core/view.go`, `internal/service/view.go`, `internal/web` (lanes, the view switch partial,
the group menu), `cmd/view.go` help, `docs/web-ui.md`, `docs/api.md`, the tix skill, Bulgarian strings.
No migration: the view is a free-form column already.
