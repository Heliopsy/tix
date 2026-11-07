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

### Requirement: Import steps

A project's page SHALL offer Import to readers holding the import scope, leading through upload, column
mapping, value mapping, preview and result, each step at its own address with a Back that keeps what was
chosen and a Cancel that leaves the import.

#### Scenario: Back keeps the mapping

- **WHEN** a reader changes a column's field, goes on to the values and presses Back
- **THEN** the columns step shows the field they chose

### Requirement: The simple paths are offered first

The first import step SHALL say that a few tasks need no import and that the mapping is optional, SHALL
take a pasted list of quick-add lines, and the steps SHALL offer Skip to preview, said outright when every
column was recognised.

#### Scenario: Every column recognised

- **WHEN** a file whose every header is recognised is uploaded
- **THEN** the columns step says so and offers Skip to preview
