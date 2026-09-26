## ADDED Requirements

### Requirement: The registry records the terminal bindings the confirmation and the form made possible

The capability registry SHALL bind `task.delete`, `dependency.remove`, `comment.edit`, `comment.delete` and
`tag.list` to the terminal interface, and SHALL no longer record a terminal gap against any of them.

The recorded terminal gap count SHALL fall by exactly those five, so that closing a gap and lowering the count
happen in one change and neither can drift from the other.

#### Scenario: None of the five records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the five operations is among them
- **AND** each of them declares a terminal binding

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

#### Scenario: Every bound operation has a key that reaches it

- **WHEN** an operation declares a terminal binding for work on a task
- **THEN** the interface pairs a key with that operation
