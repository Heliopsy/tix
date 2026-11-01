## ADDED Requirements

### Requirement: Add with details leads with the title and folds the rare fields

The Add with details form SHALL put the title first, as the largest control of the form, and SHALL
focus it when the form opens. It SHALL offer the description, project, priority, due date, assignee
and tags without further disclosure, and SHALL offer the repeat rule, the parent task and the
dependencies behind a "More options" disclosure. That disclosure SHALL be drawn open when any of its
fields holds a value or a refusal. The title SHALL be marked as required. Every control other than
the title SHALL be `--control-h` tall.

#### Scenario: Opening the form

- **WHEN** the reader opens Add with details on the task list
- **THEN** the title field is first and focused, and the repeat rule is inside a closed More options disclosure

#### Scenario: Creating with a due date, an assignee and a parent

- **GIVEN** a task `infra-1` and a member `bob`
- **WHEN** the reader adds a task due 2026-12-01, assigned to `bob`, with parent `infra-1`
- **THEN** the task is created with that due date, assignee and parent

### Requirement: The add forms preselect the listing's project

When the task list is filtered to exactly one project, both the one-line add box and Add with details
SHALL preselect that project.

#### Scenario: A filtered list

- **GIVEN** the task list filtered to `project:web`
- **WHEN** it is drawn
- **THEN** both add forms have `web` selected

### Requirement: A refused add keeps what was typed

A create the service refuses for a reason the reader can correct SHALL be answered with the task list
and the Add with details form drawn open in place, every submitted value kept, and the message under
the field the refusal names, or above the form when it names none. The response SHALL carry the
refusal's status. Adding that succeeds SHALL still return to the listing unless the browser chose to
open new tasks.

#### Scenario: An unknown parent

- **WHEN** the reader adds a task titled "write it down" with parent `infra-99`, which does not exist
- **THEN** the task list is drawn with the form open, the title still "write it down", and the refusal under the parent field

#### Scenario: A bad repeat rule

- **WHEN** the reader adds a task with the repeat rule `whenever`
- **THEN** the form is drawn open with More options open and the refusal under the repeat field
