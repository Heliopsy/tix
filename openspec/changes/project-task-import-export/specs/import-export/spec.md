## ADDED Requirements

### Requirement: Project task export

The system SHALL export one project's tasks as CSV, JSON, NDJSON or a Markdown table, selected by a filter
in the task list's query language, and SHALL refuse a filter that names another project.

#### Scenario: Filtered export

- **WHEN** a member exports project INFRA with the query `tag:ops`
- **THEN** the file holds the INFRA tasks tagged ops and no other task

#### Scenario: Another project in the query

- **WHEN** an export of INFRA is asked with the query `project:ops`
- **THEN** the export is refused as invalid and nothing is written

### Requirement: Export inclusion switches

A task export SHALL include finished tasks and subtasks and leave out trashed tasks unless asked
otherwise, and SHALL honour a request to leave out finished tasks or subtasks, or to add the trash.

#### Scenario: Leaving out finished tasks

- **WHEN** an export is asked to leave out finished tasks
- **THEN** no task in a terminal state of the project's workflow is written

### Requirement: Export columns

A task export SHALL write the columns the caller names in the order named, from ref, id, project, title,
body, status, priority, assignee, creator, tags, due, parent, depends_on, recurrence, the created, updated,
started, completed and deleted times, comments, history and `custom:KEY`, and SHALL refuse any other name.

#### Scenario: Chosen order

- **WHEN** an export names the columns `title,ref`
- **THEN** the header row is `title,ref` and each row carries the title before the reference

#### Scenario: Default columns

- **WHEN** an export names no columns
- **THEN** the file carries the default columns followed by every custom field of the project

### Requirement: Export date styles

A task export SHALL write instants as RFC 3339 in UTC by default, the day alone on request, or in the
caller's named time format and zone on request, and SHALL refuse the relative time format.

#### Scenario: Display dates

- **WHEN** an export asks for display dates in the short format and the Europe/Sofia zone
- **THEN** a due date is written the way the short format renders it in that zone

### Requirement: Export formula guard

A CSV task export SHALL prefix with `'` every cell starting with `=`, `+`, `-`, `@`, a tab or a carriage
return, and JSON, NDJSON and Markdown exports SHALL carry text unchanged.

#### Scenario: A title that is a formula

- **WHEN** a task titled `=HYPERLINK("x")` is exported as CSV and as NDJSON
- **THEN** the CSV cell reads `'=HYPERLINK("x")` and the NDJSON title is unchanged

### Requirement: Streamed export

A task export SHALL write each task as it is read, a page at a time, and SHALL answer any refusal found
before the first byte as an error rather than a partial file.

#### Scenario: Unknown column

- **WHEN** an export names the column `nope`
- **THEN** the export is refused as invalid and no file is written

### Requirement: Export authority

A task export SHALL require the `export` scope, and a token pinned to a project SHALL export only that
project.

#### Scenario: Pinned token

- **WHEN** a token pinned to INFRA with the export scope exports OPS
- **THEN** the export is refused as forbidden
