## ADDED Requirements

### Requirement: Every operation declares the scope the service enforces for it

The capability registry SHALL record, for each operation, the scope an actor must hold for the service to
permit it, or record that the operation needs an authenticated session and nothing more. A declared scope
SHALL be the scope the service actually refuses for, proved against the running service rather than
asserted.

An operation whose required scope is decided by its input rather than by the operation SHALL be named as
such with the reason, and SHALL be bound to no terminal view, because no single scope could gate it.

#### Scenario: A declared scope that nothing enforces fails the build

- **WHEN** an operation declares a scope the service does not refuse for
- **THEN** the guard reports the scope declared and the scope enforced

#### Scenario: An operation needing no scope is proved to need none

- **WHEN** an operation declares no scope
- **THEN** calling it as an actor holding no scope is not refused for want of one

#### Scenario: An operation whose scope varies by input names no scope and no view

- **WHEN** an operation is recorded as varying by input
- **THEN** it declares no scope and is bound to no terminal view

### Requirement: A route shared by two operations is addressed by its discriminator

A route the registry declares SHALL be able to name the query parameter that picks one operation out of
several sharing a pattern. No two operations SHALL be declared at the same address.

#### Scenario: Archiving and deleting a project hold different addresses

- **WHEN** the registry is read
- **THEN** archiving a project names a query discriminator and deleting one does not
- **AND** their addresses differ

#### Scenario: Two operations at one address fail the build

- **WHEN** two operations declare the same method, pattern and discriminator
- **THEN** the guard names both and the address they share

### Requirement: A binding that reaches less far than the operation records the shortfall

An operation whose binding on one surface is present but narrower than the same operation elsewhere SHALL
record that shortfall against that surface with its reason. A shortfall SHALL name a surface the operation
binds, and SHALL NOT be recorded against a surface the operation is exempted from.

#### Scenario: The browser's subtask depth is recorded

- **WHEN** the registry is read
- **THEN** the task tree operation records that the browser asks for one level of depth

#### Scenario: The browser's self-scoped credential lists are recorded

- **WHEN** the registry is read
- **THEN** the token list and the ssh key list record that the browser lists the signed-in actor's own only

### Requirement: Every terminal view the interface has carries an operation, and every view carries a read

Each view of the terminal interface SHALL have at least one registry operation bound to it, and each view
the registry names SHALL exist in the interface. Each such view SHALL have at least one read bound to it, so
no view is hidden from every reader.

#### Scenario: A view with capability and no attribution fails the build

- **WHEN** the interface has a view the registry binds nothing to
- **THEN** the guard names the view

#### Scenario: A view bound only to writes fails the build

- **WHEN** a view has operations bound to it and none of them reads
- **THEN** the guard reports that no reader can ever be offered it

### Requirement: Parity gaps are recorded where they stand

The registry SHALL record a missing binding as a gap on the surface that lacks it, and the recorded count
per surface SHALL be held by a guard so it can neither grow quietly nor be mistaken for zero. A browser
route that calls an operation only for its error, or only to decorate another screen, SHALL be recorded as a
gap rather than as a binding.

#### Scenario: The browser's two gaps are stated

- **WHEN** the recorded gaps are counted by surface
- **THEN** the browser carries two and the terminal carries fifty-eight

#### Scenario: Resolving an actor identifier in the terminal is not a gap

- **WHEN** the registry is read
- **THEN** the actor lookup is bound to the terminal's detail view
