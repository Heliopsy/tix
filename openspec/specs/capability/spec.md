# capability Specification

## Purpose

Holds the one registry that records every operation with the scope it needs and its CLI, HTTP, Web and TUI bindings, so a surface cannot quietly omit an operation.

## Requirements

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
- **THEN** the ssh key list records that the browser lists the signed-in actor's own only
- **AND** the token list records that the browser reaches another actor's tokens only for a reader holding
  `tenant:admin`, where the command line and the API reach them on `token:admin` alone

> Amended by `tokens-and-webhook-secrets-in-the-browser`. As first written this scenario said the token list
> records that the browser lists the signed-in actor's own only, and the design note below called that a
> deliberate narrowing. It was decided before anybody considered incident response: the one case the browser
> has to serve is somebody else's token leaking and needing to be dead now, and in that case the screen was
> useless. The shortfall is still recorded, because it is still real for a reader who holds `token:admin` and
> not `tenant:admin`, but it is a different shortfall and this scenario no longer describes the old one.

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
