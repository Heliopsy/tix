## ADDED Requirements

### Requirement: Every input the API accepts can be set from the command line

Every field of an input the HTTP API decodes SHALL be settable from the command bound to the same
operation, by an argument or a flag, unless the field is set by the command itself for a recorded
reason. A list command that reports a next cursor SHALL accept `--cursor` to continue from it.

#### Scenario: A custom field default

- **WHEN** `tix field put infra points --type int --default 3` is run and a task is then added to
  `infra` without a value for `points`
- **THEN** the task carries `points` equal to 3

#### Scenario: A text default that looks like a number

- **WHEN** `tix field put infra code --default 42` is run for a string field
- **THEN** the default is stored as the text `42`

#### Scenario: A move at a stale version

- **WHEN** `tix task mv infra-1 doing --version 7` is run and the task is at another version
- **THEN** the command exits 4 and the task is not moved

#### Scenario: A version with several tasks

- **WHEN** `tix task mv infra-1 infra-2 doing --version 7` is run
- **THEN** the command exits 2 without moving either task

#### Scenario: Following a cursor

- **WHEN** `tix project ls --limit 1` reports a next cursor and is run again with `--cursor` set to it
- **THEN** the second run lists the next project rather than the first again
