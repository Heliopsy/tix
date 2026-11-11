## ADDED Requirements

### Requirement: Moving from the command line

The CLI SHALL offer `tix task move REF... --to PROJECT` with `--version` and `--dry-run`, and SHALL print
the moved task the way `tix task show` prints a task. `--version` SHALL be refused with several
references.

#### Scenario: Move to another project

- **WHEN** `tix task move OPS-3 --to WEB` is run
- **THEN** the output names the task under its new WEB reference
