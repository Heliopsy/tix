## ADDED Requirements

### Requirement: The terminal asks what happens to a deleted project's tasks

The terminal interface's project removal form SHALL, when deletion is chosen, ask whether the project's tasks are
deleted with it or moved to one of the other live projects, offering each by key. Before confirming a deletion the
interface SHALL read the plan, and the confirmation SHALL state the number of tasks and either that they are deleted
permanently or the target and the status mapping. Agreeing SHALL perform the deletion with the choice made.

#### Scenario: The confirmation states the move

- **GIVEN** a project holding tasks and another live project OPS
- **WHEN** the reader chooses delete and "move to OPS" and agrees to the form
- **THEN** the confirmation names the task count, OPS and the status mapping
- **AND** agreeing moves the tasks to OPS and deletes the project

#### Scenario: The confirmation states the permanent deletion

- **WHEN** the reader chooses delete and "delete them"
- **THEN** the confirmation says the tasks are deleted permanently
