# Design

## A table, not a flagged task

A template stored as a task with a flag would have to be excluded from every listing, claim
query, statistic, filter, export and search, and every new query would have to remember it.
`task_templates` holds one row per template with the task shape as JSON, so nothing that reads
tasks can see one.

| Column | Meaning |
| --- | --- |
| id, tenant_id | identity, tenant scope |
| name | unique per tenant, case-insensitively |
| project_id | the project the template belongs to, NULL for tenant-wide; deleting the project deletes its templates |
| body | JSON `core.TemplateTask`: title, body, priority, tags, custom fields, recurrence, subtasks |
| due_in | seconds from use to due date, 0 for none |
| uses | how many times the template was used, which numbers the use placeholder |
| created_by, created_at, updated_at | provenance |

## Snapshot on save

`SaveTemplate` reads the live task and, with subtasks, its live subtree (the walk `CopyTask`
uses). Status, assignee, due date, comments, dependencies, claims and history are not part of
a template: they describe one piece of work in flight rather than its shape. Subtasks nest to
the depth the task tree has.

## Use is a copy from memory

`UseTemplate` turns the snapshot into in-memory source tasks with synthetic identifiers and
feeds them to the copier `CopyTask` uses. That keeps one implementation of the rules for what a
new task in a project gets: the target's initial state, the custom fields the target defines
(with its defaults and required fields), tags attached by name, and an audit entry and a
`task.created` event per task. The audit action is `task.template` and the event payload names
the template.

Placeholders are replaced in every title of the tree, case-insensitively: DATE becomes today's
date (YYYY-MM-DD, UTC), N the use number (uses + 1), PROJECT the target project's key. Unknown
tokens stay as typed. Docs write the tokens in capitals so a rendered page never shows a bare
lowercase word in braces, which the site's smoke test treats as an unfilled token.

## Scope and authority

| Operation | Project template | Tenant-wide template |
| --- | --- | --- |
| list, show | task read on its project | anyone with task read |
| save, edit, delete | task create on its project | `template.tenant.write` (task:write, not confinable) |
| use | task create on the target project | task create on the target project |

A project-pinned token sees its own project's templates and the tenant-wide ones, can use
either, and cannot create, change or delete a tenant-wide template.

## Events

Saving, editing and deleting write one audit entry and one `template.updated` event in the same
transaction. Using writes the task rows, `task.template` audit entries, `task.created` events and
the use counter in one transaction.

## Not included

- Templates in snapshots and bundles: they are configuration and could ride in a bundle later.
- Editing a template's subtasks in place: save the task again to replace them.
