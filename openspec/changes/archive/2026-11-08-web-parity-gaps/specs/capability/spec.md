## REMOVED Requirements

### Requirement: A binding that reaches less far than the operation records the shortfall

**Reason**: Its scenarios named the three browser shortfalls this change closes, the subtask depth, the
self-scoped key list and the scope-widened token list, and none of them is recorded any more.

**Migration**: Replaced by "A narrower binding records its shortfall" below, which keeps the rule and
states what the registry holds now.

### Requirement: Parity gaps are recorded where they stand

**Reason**: Its scenario stated that the browser carries two gaps, and this change binds both.

**Migration**: Replaced by "Parity gaps are recorded on the surface that lacks them" below, which keeps
the rule and states that the browser carries none.

## ADDED Requirements

### Requirement: A narrower binding records its shortfall

An operation whose binding on one surface is present but narrower than the same operation elsewhere SHALL
record that shortfall against that surface with its reason. A shortfall SHALL name a surface the operation
binds, and SHALL NOT be recorded against a surface the operation is exempted from.

#### Scenario: The browser records no shortfall

- **WHEN** the registry is read
- **THEN** no operation records a shortfall against the browser
- **AND** the task tree, token list and ssh key list operations bind the browser without one

#### Scenario: A narrower terminal binding is recorded

- **WHEN** the registry is read
- **THEN** the domain add operation records that the terminal adds a hostname with no certificate of its own

### Requirement: Parity gaps are recorded on the surface that lacks them

The registry SHALL record a missing binding as a gap on the surface that lacks it, and the recorded count
per surface SHALL be held by a guard so it can neither grow quietly nor be mistaken for zero. A browser
route that calls an operation only for its error, or only to decorate another screen, SHALL be recorded as a
gap rather than as a binding.

#### Scenario: The browser carries no gap

- **WHEN** the recorded gaps are counted by surface
- **THEN** the browser carries none, and the identity and actor lookups bind browser screens

#### Scenario: Resolving an actor identifier in the terminal is not a gap

- **WHEN** the registry is read
- **THEN** the actor lookup is bound to the terminal's detail view
