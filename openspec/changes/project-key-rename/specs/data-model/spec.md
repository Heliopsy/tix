## ADDED Requirements

### Requirement: A project's key can be changed and its old keys stay aliases

A project update SHALL accept a new key. The new key SHALL satisfy the same rules a key at creation
satisfies, SHALL be stored in lower case, and SHALL NOT be the current key or a kept alias of another
project in the tenant; such a key SHALL be refused with a conflict error. The rename SHALL write the
project row, record the previous key as an alias of the project, and write its audit entry and a
`project.updated` event naming the previous key, all in one transaction.

Old keys SHALL be kept as aliases for as long as the project exists. A lookup of a project or of a
task reference by an alias SHALL resolve to the renamed project, and the returned record SHALL carry
the current key and the current reference. A project MAY take back one of its own aliases, which then
stops being an alias. Deleting a project SHALL remove its aliases.

#### Scenario: Renaming a key renames every reference

- **GIVEN** a project `infra` holding task `infra-8`
- **WHEN** the project's key is changed to `ops`
- **THEN** reading the task returns reference `ops-8`
- **AND** the audit log and the event stream each record one project update naming `infra` as the previous key

#### Scenario: Old references still resolve

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a caller reads project `infra` or task `infra-8`
- **THEN** the project `ops` and the task `ops-8` are returned

#### Scenario: A key in use is refused

- **GIVEN** projects `infra` and `web`
- **WHEN** `web` is renamed to `infra`
- **THEN** the rename is refused with a conflict error and nothing changes

#### Scenario: An alias of another project is refused

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a new project is created with key `infra`, or another project is renamed to `infra`
- **THEN** the write is refused with a conflict error

#### Scenario: A project takes back its own old key

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** `ops` is renamed back to `infra`
- **THEN** the rename succeeds and `ops` becomes an alias of the project

#### Scenario: An invalid key is refused

- **WHEN** a project is renamed to `9lives` or to `a b`
- **THEN** the rename is refused with a validation error

#### Scenario: Aliases are isolated per tenant

- **GIVEN** tenant A renamed its project `infra` to `ops`
- **WHEN** tenant B creates a project `infra`
- **THEN** the creation succeeds and tenant A's alias does not resolve in tenant B

#### Scenario: A snapshot naming an old key updates the renamed project

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a snapshot whose project is keyed `infra` is imported
- **THEN** the project keyed `ops` is updated and keeps the key `ops`
