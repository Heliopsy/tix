# Task templates

## Why

Recurring kinds of work (a release checklist, an onboarding task with its subtasks, a bug triage
with fixed tags and fields) get retyped or copied from an old task each time. Copying an old
task drags its history along and needs the old task to stay around. A template is a named,
reusable shape of a task that is not itself work.

## What Changes

- New entity **task template**: a name, a scope (one project, or the whole tenant), an optional
  due offset, a use counter and a snapshot of a task: title, description (checklists included),
  priority, tags, custom fields, repeat rule and, optionally, its subtasks, recursively.
- **Save as template** snapshots an existing task. **Use** creates a task (and its subtasks) from
  a template, in the template's project or, for a tenant-wide template, any project the caller
  names. A title may carry placeholders for today's date, the use number and the project key.
  An offset such as `3d` sets the due date.
- Templates are not tasks. They live in their own table (migration 0025) and never appear in
  task listings, claims, statistics, filters, snapshots or the event stream's task events.
- Creating from a template reuses the copy machinery behind `CopyTask`, so fields, tags, status
  and the audit trail follow the same rules as a copy.
- Surfaces: `tix template ls|save|show|edit|rm|use`, `/v1/templates` routes, a Templates screen
  under Configure, Save as template in the task actions, New from template in the add box menu
  and the Add with details form, and a template picker in the terminal board.

## Impact

- `core.TaskTemplate`, `core.TemplateTask`, inputs and `TemplateService` on `core.Service`.
- Migration 0025 `task_templates` on both engines; store methods.
- New event type `template.updated`; new audit actions `template.save`, `template.update`,
  `template.delete`, `task.template`.
- New authz action `template.tenant.write` for tenant-wide templates.
