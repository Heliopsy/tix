## ADDED Requirements

### Requirement: Snapshots do not carry stars

A snapshot SHALL NOT carry stars, and an exported task SHALL NOT carry a starred flag. A star is
a personal view preference of an actor, and a snapshot carries no actors.

#### Scenario: Export is unchanged by a star

- **GIVEN** a tenant exported once
- **WHEN** a task is starred and the tenant is exported again
- **THEN** the two documents are byte-identical
