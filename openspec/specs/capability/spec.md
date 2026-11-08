# capability Specification

## Purpose

Holds the one registry that records every operation with the scope it needs and its CLI, HTTP, Web and TUI bindings, so a surface cannot quietly omit an operation.

## Requirements

### Requirement: Every operation declares the scope the service enforces for it

The capability registry SHALL record, for each operation, the scope an actor must hold for the service to
permit it, or record that the operation needs an authenticated session and nothing more. A declared scope
SHALL be the scope the service actually refuses for, proved against the running service rather than
asserted.

#### Scenario: A declared scope that nothing enforces fails the build

- **WHEN** an operation declares a scope the service does not refuse for
- **THEN** the guard reports the scope declared and the scope enforced

#### Scenario: An operation needing no scope is proved to need none

- **WHEN** an operation declares no scope
- **THEN** calling it as an actor holding no scope is not refused for want of one

### Requirement: An operation whose scope varies by input names no scope and no view

An operation whose required scope is decided by its input rather than by the operation SHALL be named as
such with the reason, and SHALL be bound to no terminal view, because no single scope could gate it.

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

### Requirement: A recorded shortfall describes the narrowing the code performs

A recorded shortfall SHALL describe the narrowing the code actually performs. Where a narrowing is lifted
or replaced, the record SHALL be rewritten rather than left describing the former behaviour, and any spec
stating the former narrowing as deliberate SHALL be corrected, including an archived one.

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

#### Scenario: None of the four records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the four operations is among them
- **AND** each of them declares a terminal binding

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

### Requirement: The stored audit log and the live tail name different terminal views

`audit.list` SHALL name a terminal view distinct from the one `event.subscribe` names, because the two are
offered on different scopes and a shared view would be offered on either.

#### Scenario: The live tail and the stored log name different views

- **WHEN** the registry is asked which view each is bound to
- **THEN** the two names differ

### Requirement: The terminal artifact binding records the parts it does not gather

The terminal binding for `artifact.put` SHALL record a limitation naming the parts of an artifact the
interface does not gather, because a surface that serves part of an operation while the registry says it
serves all of it is the same untruth as a missing binding.

#### Scenario: The artifact binding says how far it reaches

- **WHEN** the terminal binding for recording an artifact is read
- **THEN** it carries a limitation naming the payload and the inline blob

### Requirement: The registry records the project screen's bindings and the two absences it does not close

The capability registry SHALL bind `project.show`, `project.update`, `project.archive`, `project.delete`,
`workflow.get`, `field.put` and `field.delete` to the terminal interface, and SHALL no longer record a
terminal gap against any of them.

#### Scenario: None of the nine records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the nine operations is among them

#### Scenario: The view the bindings name exists and can be reached

- **WHEN** the registry names a terminal view
- **THEN** the interface has a view of that name
- **AND** at least one read is bound to it, so a reader can be offered it

### Requirement: The registry records the two workflow writes as terminal absences

The registry SHALL record `workflow.put` and `workflow.delete` as absences the terminal cannot serve rather
than as gaps, each with the reason: a state machine is not gatherable by a single line of text or a form of
fixed lists, and the only workflow a terminal names is one the service refuses to delete.

#### Scenario: The seven declare a terminal binding and the two declare a reason

- **WHEN** each of the nine is read from the registry
- **THEN** seven of them name a terminal view
- **AND** the other two record an exemption carrying a reason

### Requirement: The project screen lowers the terminal gap count by nine

The recorded terminal gap count SHALL fall by exactly the nine operations the project screen binds or records
as absences, so that closing a gap and lowering the count happen in one change and neither can drift from the
other.

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

### Requirement: The registry records the tenant screen's bindings and the two absences it does not close

The capability registry SHALL bind `tenant.show`, `tenant.list`, `tenant.update`, `domain.add`,
`domain.list`, `domain.remove`, `member.add`, `member.list` and `member.remove` to the terminal interface,
and SHALL no longer record a terminal gap against any of them.

The recorded terminal gap count SHALL fall by exactly those nine, so that closing a gap and lowering the
count happen in one change and neither can drift from the other.

#### Scenario: None of the nine records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the nine operations is among them

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

### Requirement: The terminal domain binding records what it cannot gather

`domain.add` SHALL record a terminal limitation naming what the screen cannot gather, because a binding that
serves part of an operation while the registry says it serves all of it is the same untruth as a missing
binding.

#### Scenario: The partial binding says how far it reaches

- **WHEN** `domain.add` is read from the registry
- **THEN** it declares a terminal binding
- **AND** records a limitation naming what the terminal cannot gather

### Requirement: The registry records the status operation and the one surface it does not reach

The capability registry SHALL hold exactly one operation for the new `Status` service method, bound to the
command line, the HTTP API and the browser.

The declared scope SHALL be the scope the service actually enforces, which the authority gate proves
against the real service rather than taking on trust.

#### Scenario: The operation is declared once

- **WHEN** the registry is asked for the operation bound to `Status`
- **THEN** exactly one operation names that method
- **AND** it declares a command line binding, an HTTP route and a browser screen

### Requirement: The status operation raises the gap count and the operation count by one

The recorded terminal gap count SHALL rise by exactly one, so that opening a gap and raising the count
happen in one change and neither can drift from the other. The operation count SHALL rise by exactly one
for the same reason.

#### Scenario: The counts move together

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** the gate fails if the gap is closed without lowering the number

### Requirement: The terminal documentation states the status operation's figures

The prose in `docs/tui.md` SHALL state the new figures, both the gap count and the operation count, because
a page that names the old one is a page that has quietly stopped being true.

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

### Requirement: The registry records the project deletion plan on every surface

The capability registry SHALL declare `project.delete-plan` for `PlanProjectDeletion` with scope `project:read`,
bound to `tix project rm` on the command line, `GET /api/v1/projects/{ref}/deletion` over HTTP, the deletion
confirmation screen in the browser, and the project view in the terminal, with no gap recorded against it.

#### Scenario: The plan binds every surface

- **WHEN** the registry entry for `PlanProjectDeletion` is read
- **THEN** it names a binding on the command line, HTTP, the browser and the terminal, and records no gap

### Requirement: Copy is a registered capability

The capability registry SHALL hold `task.copy` bound to `CopyTask` with the `task:write` scope
and bindings on the CLI, HTTP, web and terminal surfaces.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `CopyTask` appears exactly once with a CLI command, an HTTP route, a web route and a TUI view

### Requirement: Bulk actions are a registered capability

The capability registry SHALL hold `task.bulk` bound to `BulkTasks` with CLI, HTTP and web
bindings, recorded as varying its scope by input, and exempted from the terminal interface with
the reason that the board acts on one card at a time.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `BulkTasks` appears exactly once with a CLI command, an HTTP route, a web route and a TUI exemption

### Requirement: Snooze is a registered capability

The capability registry SHALL hold `task.snooze` bound to `SnoozeTask` and `task.unsnooze` bound
to `UnsnoozeTask`, each with the `task:write` scope and CLI, HTTP, web and terminal bindings, and
`task.wake` bound to `WakeSnoozed` with CLI and HTTP bindings and the web and terminal recorded as
inapplicable, because waking is a background loop.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `SnoozeTask`, `UnsnoozeTask` and `WakeSnoozed` each appear exactly once with their bindings

### Requirement: Templates are registered capabilities

The capability registry SHALL hold `template.list`, `template.show`, `template.save`,
`template.update`, `template.delete` and `template.use`, each bound to its service method with
CLI, HTTP, web and terminal bindings.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** every template method appears exactly once with a binding on every surface

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

### Requirement: The registry binds the integrations operations to the terminal

The capability registry SHALL bind `webhook.list`, `webhook.put`, `webhook.delete`, `webhook.deliveries`,
`webhook.redeliver`, `sync.source.list`, `sync.source.put`, `sync.source.delete`, `sync.run`, `retention.get`,
`retention.put` and `retention.prune` to the terminal integrations view, and SHALL record no terminal gap
against any of them.

#### Scenario: None of the twelve records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the twelve operations is among them
- **AND** each of them names the integrations view

### Requirement: The integrations view lowers the terminal gap count by twelve

The recorded terminal gap count SHALL fall by exactly the twelve operations the integrations view binds, so that
closing a gap and lowering the count happen in one change.

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate

### Requirement: The registry binds server and global administration to the terminal

The registry SHALL bind `status.read`, `connection.list` and `connection.end` to the `server` terminal
view, and `tenant.create`, `tenant.delete`, `tenant.user.list`, `tenant.user.create`, `tenant.user.update`,
`tenant.user.delete`, `tenant.member.add`, `global_admin.list`, `global_admin.grant`, `global_admin.revoke`
and `account.log.any` to the `globaladmin` view.

#### Scenario: None of the fourteen records a terminal exemption

- **WHEN** the registry is read
- **THEN** each of the fourteen declares a terminal binding and no terminal exemption

### Requirement: The terminal tenant deletion records what it does not delete

`tenant.delete` SHALL record a terminal limitation saying the session's own tenant is not deleted from the
terminal.

#### Scenario: The shortfall is readable off the operation

- **WHEN** `tenant.delete` is read from the registry
- **THEN** it records a terminal limitation

### Requirement: An operation needing global admin is permitted on the service's report

`Operation.Permits` SHALL answer, for an operation whose scope is `global:admin`, whether the actor carries
the service's global admin report, because the policy decides global admin from the store and not from a
scope a session holds.

#### Scenario: A reported global admin is permitted

- **WHEN** an actor reported as a global admin and holding no scopes is asked about `ListGlobalAdmins`
- **THEN** it is permitted, and an actor without the report is not

### Requirement: Server and global administration lower the terminal gap count by five

The recorded terminal gap count SHALL fall by five, for `status.read`, `connection.list`,
`connection.end`, `tenant.create` and `tenant.delete`.

#### Scenario: The count follows the bindings

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records

### Requirement: The registry records the account screen's bindings

The capability registry SHALL bind `token.create`, `token.list`, `token.revoke`, `sshkey.enrol`, `sshkey.list`, `sshkey.revoke`, `totp.status`, `totp.enrol`, `totp.confirm`, `totp.codes` and `totp.remove` to the terminal account view, and SHALL no longer record a terminal gap against any of them.

#### Scenario: None of the eleven records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the eleven operations is among them
- **AND** the recorded terminal gap count is lower by eleven

### Requirement: The account log reaches the terminal

`account.log` SHALL be bound to the terminal account view rather than exempted, because the service decides who may read it and the browser reads it from any tenant session.

#### Scenario: The exemption is gone

- **WHEN** `account.log` is read from the registry
- **THEN** it declares a terminal binding and no terminal exemption

### Requirement: The terminal credential listings record that they list the reader's own

`token.list` and `sshkey.list` SHALL record a terminal limitation stating that the account view lists the signed-in actor's own, where the command line and the API take another actor.

#### Scenario: The partial binding says how far it reaches

- **WHEN** `token.list` is read from the registry
- **THEN** it declares a terminal binding and a terminal limitation
