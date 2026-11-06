## ADDED Requirements

### Requirement: Project export dialog

A project's page SHALL offer an Export button in its header to readers holding the export scope, opening a
dialog with the format, date style, CSV separator, filter, inclusion switches and an ordered list of
columns, each column with a move up and a move down button, and SHALL download the file on submit.

#### Scenario: Exporting from the board

- **WHEN** a member opens Export on a project, ticks title and ref, and submits
- **THEN** the browser downloads the project's tasks with those columns

#### Scenario: Viewer

- **WHEN** a viewer opens a project's page
- **THEN** there is no Export button

### Requirement: Refused export keeps the dialog

A refused export from the dialog SHALL draw the project again with the dialog open, every choice as
submitted, and the reason beside the field it concerns.

#### Scenario: Filter that does not parse

- **WHEN** a reader submits the export dialog with a filter that does not parse
- **THEN** the dialog is open again holding that filter and the refusal
