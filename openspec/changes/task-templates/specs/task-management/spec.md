## ADDED Requirements

### Requirement: Save a task as a template

The service SHALL save a live task as a named template holding its title, description,
priority, tags, custom fields and repeat rule and, when asked, its live subtasks recursively.
The template SHALL belong to the task's project unless the caller asks for a tenant-wide
template. A template name SHALL be unique in the tenant regardless of case.

#### Scenario: Save with subtasks

- **WHEN** a member saves OPS-3, which has two subtasks, as "release" with subtasks
- **THEN** template "release" belongs to OPS and holds OPS-3's title, tags and fields and two subtasks

#### Scenario: Duplicate name

- **WHEN** a template "Release" exists and a member saves another task as "release"
- **THEN** the save is refused as a conflict and nothing is written

### Requirement: Templates are not tasks

Templates SHALL NOT appear in task listings, task trees, claims, filters, statistics or
snapshots.

#### Scenario: Listing after save

- **WHEN** a project holds three tasks and one of them is saved as a template
- **THEN** listing the project's tasks returns three tasks

### Requirement: Create a task from a template

The service SHALL create a task, and a subtask for every subtask the template holds, from a
template in one transaction. The task SHALL go to the template's project or, for a tenant-wide
template, the project the caller names. The caller MAY replace the title and give a due offset
that sets the due date relative to now; otherwise the template's own offset applies. Status,
custom fields and tags SHALL follow the rules a copy into that project follows. The template's
use counter SHALL go up by one.

#### Scenario: Use with an offset

- **WHEN** a member uses template "release" with a due offset of 3d
- **THEN** a new OPS task with the template's subtasks exists, due three days from now, in the initial state

#### Scenario: Tenant-wide template needs a project

- **WHEN** a tenant has two projects and a member uses a tenant-wide template naming no project
- **THEN** the use is refused naming the project field

### Requirement: Title placeholders

Every title created from a template SHALL have its placeholders replaced, regardless of case:
DATE in braces by today's date as YYYY-MM-DD, N in braces by the use number and PROJECT in
braces by the target project's key. Any other braced word SHALL stay as written.

#### Scenario: Numbered standup

- **WHEN** template "standup" with title "Standup {N} on {DATE}" has been used four times and is used again on 2026-11-01
- **THEN** the new task's title is "Standup 5 on 2026-11-01"

### Requirement: Manage templates

The service SHALL list the templates a caller can read, keyset-paginated by name, optionally
narrowed to one project and the tenant-wide ones; show one by name or identifier; rename it,
change its scope, due offset, title, description, priority, tags, custom fields and repeat rule;
and delete it.

#### Scenario: Rename

- **WHEN** a member renames template "release" to "ship"
- **THEN** showing "ship" returns the template and showing "release" is not found

#### Scenario: Pinned token and tenant-wide templates

- **WHEN** a token pinned to OPS tries to delete a tenant-wide template
- **THEN** the call is refused as forbidden
