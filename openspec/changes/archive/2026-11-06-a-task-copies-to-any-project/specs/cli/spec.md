## ADDED Requirements

### Requirement: Copying from the command line

The CLI SHALL offer `tix task copy REF` with `--to PROJECT`, `--title`, `--keep-status`,
`--with-comments`, `--with-subtasks`, `--with-deps`, `--link` and `--dry-run`, and SHALL print
the copy the way `tix task show` prints a task. `tix task show` SHALL name the task a copy was
copied from.

#### Scenario: Copy to another project

- **WHEN** `tix task copy OPS-3 --to WEB --with-subtasks` is run
- **THEN** the output names a new WEB task and its subtasks exist in WEB

#### Scenario: Dry run

- **WHEN** `tix task copy OPS-3 --to WEB --dry-run` is run
- **THEN** the plan is printed and no task is created
