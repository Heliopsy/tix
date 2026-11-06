## ADDED Requirements

### Requirement: Quick-add on the command line

`tix task add` SHALL take `--parse`, which reads the title through the quick-add parser,
resolving relative dates in the local time zone. Without `--parse` the title SHALL be stored
literally, so a script passing a title it did not write is never rewritten. An explicit flag
that contradicts the line SHALL be refused with exit code 2; the configured default project
SHALL apply only when the line names none.

#### Scenario: Parsed add

- **WHEN** `tix task add --parse 'Fix login !high #auth'` is run
- **THEN** the task is titled `Fix login`, has priority high and carries the tag `auth`

#### Scenario: Literal by default

- **WHEN** `tix task add 'Fix login !high #auth'` is run
- **THEN** the task is titled `Fix login !high #auth` with normal priority and no tags

#### Scenario: Unknown handle

- **WHEN** `tix task add --parse 'Fix login @nobody'` is run and no actor has that handle
- **THEN** the command exits 3 and creates nothing
