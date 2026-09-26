## ADDED Requirements

### Requirement: The registry records the four daily operations the terminal now reaches

The capability registry SHALL bind `audit.list`, `artifact.put`, `task.restore` and `actor.list` to the
terminal interface, and SHALL no longer record a terminal gap against any of them.

The recorded terminal gap count SHALL fall by exactly those four, so that closing a gap and lowering the
count happen in one change and neither can drift from the other. The gate SHALL fail both when the count
grows and when it falls without the constant being lowered with it.

`audit.list` SHALL name a terminal view distinct from the one `event.subscribe` names, because the two are
offered on different scopes and a shared view would be offered on either.

The terminal binding for `artifact.put` SHALL record a limitation naming the parts of an artifact the
interface does not gather, because a surface that serves part of an operation while the registry says it
serves all of it is the same untruth as a missing binding.

#### Scenario: None of the four records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the four operations is among them
- **AND** each of them declares a terminal binding

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

#### Scenario: The live tail and the stored log name different views

- **WHEN** the registry is asked which view each is bound to
- **THEN** the two names differ

#### Scenario: The artifact binding says how far it reaches

- **WHEN** the terminal binding for recording an artifact is read
- **THEN** it carries a limitation naming the payload and the inline blob
