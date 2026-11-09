## ADDED Requirements

### Requirement: Import keeps task numbering ahead of imported tasks

An import that writes a task under its own number SHALL, in the same transaction, raise its project's task number counter to at least that number, on both engines, counting trashed tasks and tombstones. A task created after the import SHALL take the number above the highest one any task of the project holds.

#### Scenario: Creating a task after importing into a fresh tenant

- **GIVEN** a snapshot of a project whose tasks are numbered up to 5, the fifth in the trash
- **WHEN** it is imported into a fresh tenant in merge or replace mode and a task is then created there
- **THEN** the new task is numbered 6
