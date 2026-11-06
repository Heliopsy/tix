## ADDED Requirements

### Requirement: Deleting a project from the command line names the fate of its tasks

`tix project rm KEY` SHALL accept `--move-tasks-to OTHER` and `--delete-tasks`, which SHALL be mutually exclusive.
Without either, deleting a project that holds tasks SHALL exit 6 with a message naming the task count and both
flags. `--dry-run` SHALL print the plan, including the status mapping for a move, and write nothing. A successful
deletion SHALL report how many tasks were moved or deleted.

#### Scenario: Refused without a choice

- **GIVEN** a project holding tasks
- **WHEN** `tix project rm KEY` runs without either flag
- **THEN** it exits 6 and the message names `--move-tasks-to` and `--delete-tasks`

#### Scenario: Moving from the command line

- **WHEN** `tix project rm OLD --move-tasks-to NEW` runs
- **THEN** OLD is gone and its tasks are listed under NEW

#### Scenario: Both flags are a usage error

- **WHEN** both flags are passed
- **THEN** the command exits 2 and writes nothing
