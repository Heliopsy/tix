## ADDED Requirements

### Requirement: A moved task emits task.updated

Each task a move takes SHALL write a `task.move` audit entry and emit `task.updated` with `ref`,
`from_ref`, `project`, `from_project`, `status` and `from_status`, as a deleted project's moved tasks do,
and with `from_parent` when the move detached it from a parent.

#### Scenario: Payload of a move

- **WHEN** OPS-3 is moved to WEB and becomes WEB-5
- **THEN** a `task.updated` event carries `ref` WEB-5, `from_ref` OPS-3, `project` WEB and `from_project` OPS
