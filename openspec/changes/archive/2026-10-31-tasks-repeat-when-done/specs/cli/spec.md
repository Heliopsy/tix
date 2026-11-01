## ADDED Requirements

### Requirement: The command line sets, clears and shows a repeat rule

`tix task add` SHALL accept `--repeat RULE`. `tix task edit` SHALL accept `--repeat RULE` to set or replace
the rule and `--no-repeat` to remove it, and SHALL refuse both together as a usage error. `tix task show`
SHALL state the rule and the successor's reference when there is one; the table listing SHALL mark a
recurring task; JSON and YAML output SHALL carry `recurrence` and `recurrence_next_id`.

#### Scenario: Add with a rule

- **WHEN** `tix task add "Pay rent" --due 2026-11-01 --repeat monthly` is run
- **THEN** the task is created carrying `every month`

#### Scenario: Both flags

- **WHEN** `tix task edit REF --repeat daily --no-repeat` is run
- **THEN** it exits with a usage error and the task is unchanged

#### Scenario: Show

- **WHEN** `tix task show` is run on a finished occurrence that spawned
- **THEN** the output states its rule and names the successor's reference
