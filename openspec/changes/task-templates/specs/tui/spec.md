## ADDED Requirements

### Requirement: Templates in the terminal

On a board with an open project, `B` SHALL load the templates the session can read for that
project and open a form choosing a template and an action: create a task (with an optional title
and due offset), show, rename or delete, offering only the actions the session's authority
reaches. Create a task SHALL use the template in the board's project and name the new task on
the status bar; delete SHALL ask for `y` first. `E` SHALL open a form saving the selected task as
a template with a name, a scope (this project or the whole tenant) and whether subtasks are kept.

#### Scenario: Use from the board

- **WHEN** a user presses `B` on the OPS board, picks "release" and create a task, and applies
- **THEN** a new OPS task exists and the status bar names it

#### Scenario: Save the selected task

- **WHEN** a user selects OPS-3, presses `E`, types "release" and applies
- **THEN** template "release" exists in OPS
