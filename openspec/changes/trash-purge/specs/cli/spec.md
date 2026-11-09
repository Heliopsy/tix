## ADDED Requirements

### Requirement: The command line purges trashed tasks

`tix task purge REF...` SHALL permanently delete each named task from the trash, reporting per reference like the other bulk task commands, and SHALL exit 6 for a live task or a tenant whose trash is off, 3 for an unknown reference and 5 when permission is denied. `--dry-run` SHALL report the plan without writing.

#### Scenario: Purging from the command line

- **GIVEN** a task in the trash
- **WHEN** the user runs `tix task purge` with its reference
- **THEN** the command succeeds and the task can no longer be restored
