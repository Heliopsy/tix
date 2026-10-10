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

A recorded shortfall SHALL describe the narrowing the code actually performs. Where a narrowing is lifted
or replaced, the record SHALL be rewritten rather than left describing the former behaviour, and any spec
stating the former narrowing as deliberate SHALL be corrected, including an archived one.

#### Scenario: The browser's subtask depth is recorded

- **WHEN** the registry is read
- **THEN** the task tree operation records that the browser asks for one level of depth

#### Scenario: The browser's self-scoped credential lists are recorded

- **WHEN** the registry is read
- **THEN** the ssh key list records that the browser lists the signed-in actor's own only

#### Scenario: The browser's token list records the scope that widens it

- **WHEN** the registry is read
- **THEN** the token list records that the browser reaches another actor's tokens only for a reader holding
  the tenant administration scope, where the command line and the API reach them on the token
  administration scope alone

#### Scenario: A lifted narrowing is not left recorded as it was

- **GIVEN** a spec stating that the browser deliberately lists the signed-in actor's own tokens only
- **WHEN** the browser begins listing the tenant's tokens for an administrator
- **THEN** that spec is corrected rather than left contradicting the code

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

### Requirement: The registry records the status operation and the one surface it does not reach

The capability registry SHALL hold exactly one operation for the new `Status` service method, bound to the
command line, the HTTP API and the browser.

It SHALL record a terminal gap rather than an exemption, because the terminal interface has no
administration screen at all and the server is one more thing it cannot show, which is a parity defect kept
visible rather than an operation a terminal cannot serve.

The declared scope SHALL be the scope the service actually enforces, which the authority gate proves
against the real service rather than taking on trust.

The recorded terminal gap count SHALL rise by exactly one, so that opening a gap and raising the count
happen in one change and neither can drift from the other. The operation count SHALL rise by exactly one
for the same reason.

The prose in `docs/tui.md` SHALL state the new figures, both the gap count and the operation count, because
a page that names the old one is a page that has quietly stopped being true.

#### Scenario: The operation is declared once

- **WHEN** the registry is asked for the operation bound to `Status`
- **THEN** exactly one operation names that method
- **AND** it declares a command line binding, an HTTP route and a browser screen

#### Scenario: The terminal absence is recorded as a gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** the status operation is among them
- **AND** its reason is marked as a gap rather than as an operation the terminal cannot serve

#### Scenario: The counts move together

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** the gate fails if the gap is closed without lowering the number

#### Scenario: The page states the figures the registry holds

- **WHEN** the terminal documentation's gap section is read
- **THEN** every number it states in digits is one the registry holds
- **AND** the shares it breaks the gaps into still add up to the gap count

### Requirement: The browser screen serves the same report

The browser interface SHALL offer a status screen reading the same operation, so the capability is bound on
the web rather than exempted.

The screen SHALL show the installation figures, the servers with their attached state, and the work counts
for the tenant the session is pinned to.

A server that has stopped heartbeating SHALL be visibly distinguished on the screen rather than omitted.

#### Scenario: The screen is served and names its operation

- **WHEN** the browser routes are enumerated
- **THEN** the status route names the `Status` service method
- **AND** the template it declares is one the build embeds

#### Scenario: A stale server is visible in the browser

- **WHEN** the screen renders a server that stopped heartbeating
- **THEN** that server appears
- **AND** it is marked as not heartbeating
