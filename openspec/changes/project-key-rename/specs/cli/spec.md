## ADDED Requirements

### Requirement: The project edit command changes the key

`tix project edit KEY` SHALL accept `--key NEW` to change the project's key, alongside the flags that
change its other attributes. An invalid key SHALL exit with the validation code and a key held by
another project SHALL exit with the conflict code. Every command that takes a project or a task
reference SHALL accept an old key of a renamed project, and SHALL print the current key and
reference.

#### Scenario: Renaming from the command line

- **WHEN** `tix project edit infra --key ops` runs
- **THEN** the project is printed with key `ops`

#### Scenario: An old reference on the command line

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** `tix task show infra-8` runs
- **THEN** the task is printed with reference `ops-8`

#### Scenario: A taken key exits with the conflict code

- **WHEN** `tix project edit web --key infra` runs and `infra` belongs to another project
- **THEN** the command exits with code 4
