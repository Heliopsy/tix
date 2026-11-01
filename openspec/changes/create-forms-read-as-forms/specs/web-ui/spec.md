## ADDED Requirements

### Requirement: The new project form starts from the name

The projects screen SHALL offer "New project" as a button in its header. The form it opens SHALL put
the name first and focus it, then the key, the workflow and the description, and SHALL group the
colour and icon controls under "Appearance". Name and key SHALL be marked as required. With scripting
on, the key SHALL be suggested from the name until the reader edits it, SHALL avoid keys already in
use, and the form SHALL say what the project's tasks will be numbered. The project settings on a
board SHALL use the same layout and the same Appearance group.

#### Scenario: Opening the form

- **WHEN** the reader opens New project
- **THEN** the name field is first and focused, the key follows it, and colour and icon sit under Appearance

#### Scenario: Suggesting a key

- **GIVEN** a project with the key `web`
- **WHEN** the reader types the name "Web shop"
- **THEN** the key field reads `web2` until the reader types a key of their own

### Requirement: A refused project form keeps what was typed

A project create or settings save the service refuses for a reason the reader can correct SHALL be
answered with the same screen, the form drawn open, every submitted value kept, and the message under
the field the refusal names. The response SHALL carry the refusal's status.

#### Scenario: A key already in use

- **GIVEN** a project with the key `infra`
- **WHEN** the reader creates a project named "Infra two" with the key `infra`
- **THEN** the projects screen is drawn with the form open, the name still "Infra two", and the refusal under the key field

#### Scenario: Clearing a project's name

- **WHEN** the reader saves a project's settings with an empty name
- **THEN** the board is drawn with its settings open and the refusal under the name field

### Requirement: Create and settings forms share their keyboard and actions

The add task, new project and project settings forms SHALL place their primary action at the bottom
right with Cancel beside it. With scripting on, Ctrl+Enter or Cmd+Enter SHALL submit the form through
its ordinary submission, and Escape SHALL cancel it. With scripting off every form SHALL still submit
and every refusal SHALL still be drawn in place.

#### Scenario: Submitting from the keyboard

- **WHEN** the reader presses Ctrl+Enter in the description of Add with details
- **THEN** the form is submitted as if Create task had been pressed

#### Scenario: Cancelling a form drawn in place

- **GIVEN** a refused new project form drawn in place
- **WHEN** the reader presses Escape
- **THEN** the browser follows the form's Cancel link
