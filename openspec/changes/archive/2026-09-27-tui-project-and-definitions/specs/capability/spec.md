## ADDED Requirements

### Requirement: The registry records the project screen's bindings and the two absences it does not close

The capability registry SHALL bind `project.show`, `project.update`, `project.archive`, `project.delete`,
`workflow.get`, `field.put` and `field.delete` to the terminal interface, and SHALL no longer record a
terminal gap against any of them.

The registry SHALL record `workflow.put` and `workflow.delete` as absences the terminal cannot serve rather
than as gaps, each with the reason: a state machine is not gatherable by a single line of text or a form of
fixed lists, and the only workflow a terminal names is one the service refuses to delete.

The recorded terminal gap count SHALL fall by exactly those nine, so that closing a gap and lowering the count
happen in one change and neither can drift from the other.

#### Scenario: None of the nine records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the nine operations is among them

#### Scenario: The seven declare a terminal binding and the two declare a reason

- **WHEN** each of the nine is read from the registry
- **THEN** seven of them name a terminal view
- **AND** the other two record an exemption carrying a reason

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

#### Scenario: The view the bindings name exists and can be reached

- **WHEN** the registry names a terminal view
- **THEN** the interface has a view of that name
- **AND** at least one read is bound to it, so a reader can be offered it
