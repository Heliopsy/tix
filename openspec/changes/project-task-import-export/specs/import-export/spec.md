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

### Requirement: Project task import

The system SHALL read a CSV, JSON or NDJSON file of tasks, or a list of quick-add lines, into one project,
detecting the format and recognising files written by tix, Jira, GitHub, Trello and Linear.

#### Scenario: A Jira export

- **WHEN** an admin imports a Jira CSV export into OPS with no mapping
- **THEN** each issue becomes a task with its summary as title, its status mapped to a workflow state and its labels as tags

#### Scenario: A list of lines

- **WHEN** a list `Fix login #bug !high` is imported as lines
- **THEN** a task titled `Fix login` with tag bug and high priority is created

### Requirement: Suggested mapping

An import without a mapping SHALL map every column to a field by its header, case, accent and language
insensitively, and every status and priority value by its meaning, and SHALL report the mapping it used and
whether every column was recognised.

#### Scenario: Round trip

- **WHEN** a project's export in any of CSV, JSON or NDJSON is imported into another project with no mapping
- **THEN** titles, descriptions, statuses, priorities, tags, due dates, repeat rules, custom fields, parents and dependencies arrive unchanged

### Requirement: Mapping file

An import SHALL accept a mapping file in YAML or JSON naming, for each column, its field, for each field, a
translation of values and what to do with a value that has none, and the date format, separator, source
name and mode, and SHALL report the mapping in force in the same form.

#### Scenario: A value mapped by hand

- **WHEN** a mapping translates the status `In Progress` to `blocked`
- **THEN** a row with that status becomes a blocked task

### Requirement: Import modes

An import SHALL match a row to a task by external id within its source, or by reference within the
project, and SHALL update a matched task in upsert mode, leave it alone in skip mode and create a new task
in create mode.

#### Scenario: Importing the same file twice

- **WHEN** a file is imported twice in upsert mode with one title changed in between
- **THEN** the second import updates that one task and creates none

### Requirement: Import is all or nothing

An import SHALL write nothing when any row is in error, and SHALL report every such row by line. A dry run
SHALL do everything an import does and write nothing, reporting the first rows as they would be written.

#### Scenario: A bad date in the middle

- **WHEN** the third row of five has a due date that does not parse
- **THEN** no task is created and the result names line 4 and why

### Requirement: Import links within the file

Parent and dependency columns SHALL name another row of the file by its external id, another id or its
reference, or a task of the project, and SHALL be linked after every row is written.

#### Scenario: Subtask before its parent

- **WHEN** a subtask's row comes before its parent's row
- **THEN** the subtask is linked to the parent created from the later row

### Requirement: Import authority and audit

A task import SHALL require the `import` scope, keep a pinned token to its own project, record each task
created or updated as a task change, and record one `task.import` entry with the counts and the mapping,
never the file's contents.

#### Scenario: One entry per import

- **WHEN** a file of three tasks is imported
- **THEN** three task.create entries and one task.import entry are recorded, and no entry holds the file
