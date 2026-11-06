## ADDED Requirements

### Requirement: The command line sets a lease when it adds a task

`tix task add --lease D` SHALL create the task with lease D. A lease above the effective maximum SHALL exit 2 and
create nothing, as `tix task edit --lease` does.

#### Scenario: Adding a task with a lease

- **WHEN** `tix task add "build" --lease 2h` runs and the task is claimed without `--ttl`
- **THEN** the claim lasts 2h and reports the task as its source

#### Scenario: Adding a task with a lease above the maximum

- **WHEN** `tix task add "build" --lease 48h` runs where the maximum is 24h
- **THEN** it exits 2 and no task is created
