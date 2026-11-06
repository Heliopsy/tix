## ADDED Requirements

### Requirement: Tables and text draw keys and references upper case

The CLI's table output and its human-readable text SHALL draw project keys and task references upper
case (`INFRA`, `INFRA-8`). JSON, YAML, NDJSON and CSV output SHALL carry the stored lower-case value
unchanged. Every argument, flag and filter term that names a project or a task SHALL accept any
case.

#### Scenario: Table and JSON

- **WHEN** a user runs `tix task ls` and `tix task ls -o json` in a project `infra`
- **THEN** the table shows `INFRA-1` and the JSON `ref` is `infra-1`

#### Scenario: Any case in

- **WHEN** a user runs `tix task show INFRA-1`, `tix task add --project INFRA` or `tix task ls --filter project:INFRA`
- **THEN** each reaches the project `infra` and its task
