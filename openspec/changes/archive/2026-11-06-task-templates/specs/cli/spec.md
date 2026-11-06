## ADDED Requirements

### Requirement: Template commands

The CLI SHALL offer `tix template ls`, `tix template save REF --name NAME [--tenant-wide]
[--with-subtasks] [--due-in D]`, `tix template show NAME`, `tix template edit NAME` with a flag
per editable field, `tix template rm NAME` and `tix template use NAME [--project P] [--title T]
[--due-in D]`.

#### Scenario: Use from the command line

- **WHEN** a user runs `tix template use release --due-in 3d`
- **THEN** the command prints the new task's reference and exits 0
