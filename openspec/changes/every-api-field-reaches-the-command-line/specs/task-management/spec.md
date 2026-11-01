## ADDED Requirements

### Requirement: Transitions honour an expected version

A transition or a route that names a version SHALL be refused as a conflict when the task is at
another version, and SHALL NOT move the task. A route checks the version before its first hop. A
transition that names no version SHALL apply to whatever version is current.

#### Scenario: Stale version on a transition

- **WHEN** a transition names version 3 and the task is at version 4
- **THEN** the transition is refused with a conflict and the task keeps its status

#### Scenario: Stale version on a route

- **WHEN** a route of two hops names a version the task is no longer at
- **THEN** no hop is applied and the route is refused with a conflict
