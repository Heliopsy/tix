## ADDED Requirements

### Requirement: The registry records the tenant screen's bindings and the two absences it does not close

The capability registry SHALL bind `tenant.show`, `tenant.list`, `tenant.update`, `domain.add`,
`domain.list`, `domain.remove`, `member.add`, `member.list` and `member.remove` to the terminal interface,
and SHALL no longer record a terminal gap against any of them.

The registry SHALL go on recording a terminal gap against `tenant.create` and `tenant.delete`, each with the
reason that a session pinned to one tenant is neither where another is made nor where the one in use is
destroyed.

`domain.add` SHALL record a terminal limitation naming what the screen cannot gather, because a binding that
serves part of an operation while the registry says it serves all of it is the same untruth as a missing
binding.

The recorded terminal gap count SHALL fall by exactly those nine, so that closing a gap and lowering the
count happen in one change and neither can drift from the other.

#### Scenario: None of the nine records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the nine operations is among them

#### Scenario: The two that stay are still recorded

- **WHEN** the registry is asked for its terminal gaps
- **THEN** `tenant.create` and `tenant.delete` are among them
- **AND** each carries a reason

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

#### Scenario: The partial binding says how far it reaches

- **WHEN** `domain.add` is read from the registry
- **THEN** it declares a terminal binding
- **AND** records a limitation naming what the terminal cannot gather
