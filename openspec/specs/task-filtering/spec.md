# task-filtering Specification

## Purpose

Defines the one filter grammar every surface parses, so the same expression means the same thing on the command line, in the terminal and in the browser.

## Requirements

### Requirement: A deadline has one classification

The system SHALL classify a task's deadline against a stated present as exactly one of: no deadline, a
deadline further off than the notable window, a deadline inside that window, or a deadline that has
passed. A deadline falling exactly at the stated present SHALL classify as passed, which is the boundary
the stores filter on.

Every surface that draws or filters by a deadline SHALL read that one classification.

#### Scenario: A deadline in the past has passed

- **WHEN** a task's deadline is earlier than the present
- **THEN** it classifies as overdue

#### Scenario: A deadline exactly at the present has passed

- **WHEN** a task's deadline is the present instant
- **THEN** it classifies as overdue
- **AND** a listing bounded at that instant returns the task

#### Scenario: A deadline inside the window is near

- **WHEN** a task's deadline is after the present and within the notable window
- **THEN** it classifies as near

#### Scenario: A deadline beyond the window is not notable

- **WHEN** a task's deadline is further off than the notable window
- **THEN** it classifies as later
- **AND** it is reported as not notable

#### Scenario: A task with no deadline is not notable

- **WHEN** a task carries no deadline
- **THEN** it classifies as having none
- **AND** it is reported as not notable

### Requirement: The filter language carries a deadline term

The filter expression language SHALL accept a `due:` term taking either a named window — `overdue` (or
`late`), `today`, `week`, `month` — or a bound spelled `due:<DATE` or `due:>DATE`.

#### Scenario: The bounded form takes a date

- **WHEN** a filter expression carries `due:<2026-04-01`
- **THEN** the upper deadline bound is that date
- **WHEN** a filter expression carries `due:>2026-04-01`
- **THEN** the lower deadline bound is that date

### Requirement: A named deadline window is resolved when the expression is answered

A named window SHALL be resolved against the moment the expression is answered. `due:overdue` SHALL set
the upper bound to that moment.

The term SHALL set the same upper and lower bounds `due-before:` and `due-after:` set, and those spellings
SHALL keep working unchanged.

#### Scenario: The shorthand bounds the listing at now

- **WHEN** a filter expression carries `due:overdue`
- **THEN** the upper deadline bound is the moment the expression was answered
- **AND** no lower bound is set

#### Scenario: A window bounds the listing at its far end

- **WHEN** a filter expression carries `due:week`
- **THEN** the upper deadline bound is seven days after the moment the expression was answered

### Requirement: The deadline term is not negatable and refuses a value it does not know

The term SHALL NOT be negatable, because it names a shape of the listing rather than a set of tasks.

A value that is neither a known window nor a parsable bound SHALL be refused as invalid input, naming the
windows it accepts, rather than being dropped.

#### Scenario: An unknown window is refused

- **WHEN** a filter expression carries `due:tomorrow`
- **THEN** the expression is refused as invalid input
- **AND** the message names the windows the term accepts

#### Scenario: The term cannot be negated

- **WHEN** a filter expression carries `-due:overdue`
- **THEN** the expression is refused as invalid input

### Requirement: The command line selects tasks by deadline

`tix task ls` SHALL offer `--overdue`, `--due-before` and `--due-after`. The two dated flags SHALL accept
the date forms `--due` accepts on `tix task add`.

`--overdue` SHALL set the upper bound to the present. Each flag SHALL take precedence over the same bound
named in a `--filter` expression.

`--overdue` given beside an explicit `--due-before` SHALL be refused as a usage error, because the two name
one bound. A pair of bounds that cannot hold anything SHALL be refused as invalid input.

#### Scenario: The shorthand lists what has run out of time

- **GIVEN** one task whose deadline has passed, one whose deadline is far off, and one with no deadline
- **WHEN** `tix task ls --overdue` runs
- **THEN** only the task whose deadline has passed is listed

#### Scenario: The expression says the same thing

- **WHEN** `tix task ls --filter 'due:overdue'` runs against the same tasks
- **THEN** the same one task is listed

#### Scenario: A flag replaces the expression's bound

- **GIVEN** a `--filter` expression naming an upper deadline bound
- **WHEN** `--due-before` names a different upper bound on the same invocation
- **THEN** the flag's bound is the one applied

#### Scenario: Two names for one bound are refused

- **WHEN** `tix task ls --overdue --due-before 2050-01-01` runs
- **THEN** the command exits with a usage error

### Requirement: A deadline bound is inclusive wherever it is answered

A deadline bound SHALL include a task whose deadline falls exactly on it, and SHALL exclude a task
carrying no deadline, both in the stores and when a filter is answered against an already loaded page.

#### Scenario: The two answers agree at the boundary

- **GIVEN** a task whose deadline is exactly the bound
- **WHEN** the filter is answered in memory
- **THEN** the task matches, as it does when the store answers the same filter

#### Scenario: A task with no deadline answers no bound

- **GIVEN** a task carrying no deadline
- **WHEN** any deadline bound is applied
- **THEN** the task does not match

### Requirement: The filter surface is the same on every transport

Every field of the task filter contract SHALL be reachable from the command line, the terminal interface,
the browser and the HTTP API, or SHALL be recorded as deliberately absent from a named surface together
with the reason for its absence.

A field added to the contract and reachable from one transport and not another SHALL fail the build.

#### Scenario: A field reachable nowhere fails

- **WHEN** a field is added to the task filter contract
- **AND** no surface offers a way to set it and no exemption names it
- **THEN** the parity guard fails, naming the field

#### Scenario: A surface losing a filter fails

- **GIVEN** a field the command line reaches through a named flag
- **WHEN** that flag is removed
- **THEN** the parity guard fails, naming the field and the flag

#### Scenario: A field that does not survive the wire fails

- **GIVEN** a filter with every field set
- **WHEN** it is sent through the client to the router
- **AND** any field does not arrive at the service as it was sent
- **THEN** the parity guard fails, naming the field

### Requirement: The filter language selects starred tasks

The filter language SHALL accept `is:starred` and `is:unstarred`, the negated `-is:starred`, and
the boolean spelling `starred:true` / `starred:false`, selecting the tasks the reading actor has
or has not starred. The command line SHALL offer the same selection as `--starred`, and the HTTP
API as the `starred` query parameter. The terminal interface SHALL answer the term in memory the
same way the store does.

#### Scenario: Only starred tasks

- **GIVEN** a reader who has starred one of three tasks
- **WHEN** they list with `is:starred`
- **THEN** only the starred task is listed

#### Scenario: Excluding starred tasks

- **WHEN** the same reader lists with `-is:starred`
- **THEN** the two unstarred tasks are listed

### Requirement: Only the service names whose stars a listing reads

The task filter contract's field naming the reading actor SHALL be set by the service from the
caller and SHALL NOT be settable by a caller, so that no reader can list by another actor's
stars.

#### Scenario: Another reader's stars

- **GIVEN** a task starred by one actor
- **WHEN** a second actor lists with `is:starred`
- **THEN** the task is not listed

### Requirement: The filter language selects recurring tasks

The filter expression language SHALL accept `is:recurring`, selecting tasks that carry a recurrence rule,
and its negation `-is:recurring`, selecting tasks that carry none. The CLI, the terminal interface, the
browser and the in-memory matcher SHALL answer it the same way, and the HTTP task listing SHALL accept the
same selection as `recurring=true` or `recurring=false`.

#### Scenario: Recurring only

- **WHEN** tasks are listed with `is:recurring`
- **THEN** only tasks carrying a rule are returned

#### Scenario: Excluding recurring tasks

- **WHEN** tasks are listed with `-is:recurring`
- **THEN** no task carrying a rule is returned

### Requirement: A project term may name an old key

A `project:` filter term SHALL accept a kept alias of a project and SHALL select that project's
tasks, exactly as its current key does.

#### Scenario: Filtering by an old key

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** tasks are listed with `project:infra`
- **THEN** the tasks of `ops` are returned

### Requirement: The filter language selects the trash

The filter language SHALL accept `is:trashed`, selecting only soft-deleted tasks, and its negation
`-is:trashed`, selecting only live tasks. When either form is present it SHALL decide whether deleted
tasks are listed, whatever `is:deleted` says, so `is:deleted -is:trashed` SHALL list live tasks only.
Both storage engines SHALL answer it in the listing query with keyset pagination unchanged, and the
in-memory matcher SHALL answer it the same way.

#### Scenario: Only deleted tasks

- **GIVEN** three tasks, one of them soft-deleted
- **WHEN** tasks are listed with `is:trashed`
- **THEN** only the deleted task is returned

#### Scenario: The trash pages like any listing

- **GIVEN** three soft-deleted tasks and two live ones
- **WHEN** tasks are listed with `is:trashed` one per page, following the cursor
- **THEN** the three deleted tasks are returned, each once, and no live task is

#### Scenario: Live only, whatever is:deleted says

- **WHEN** tasks are listed with `is:deleted -is:trashed`
- **THEN** no deleted task is returned

### Requirement: me in actor terms

The filter language SHALL accept `me` as the value of `assignee:`, `creator:` and `claimed-by:`, negated
or not, meaning the reader, on every surface that answers a filter.

#### Scenario: The terminal answers assignee:me

- **WHEN** a reader types `assignee:me` in the terminal interface's filter bar
- **THEN** the board shows the tasks assigned to that reader

### Requirement: Checklist filter terms

The filter language SHALL accept `has:checklist`, selecting tasks whose description carries a checklist
marker (a `-`, `*` or `+` bullet, a space, `[ ]`, `[x]` or `[X]`, and a space), and `-has:checklist` for the rest;
`checklist:open`, selecting those carrying an unticked marker, and `-checklist:open` for the rest; and
`checklist:done`, selecting those carrying a marker and no unticked one. `checklist:done` SHALL NOT be
negatable.

#### Scenario: Open and done are told apart

- **GIVEN** one task with an open item, one with every item ticked, and one with no checklist
- **WHEN** the listing is filtered by `checklist:open`, then by `checklist:done`
- **THEN** the first lists only the task with an open item, and the second only the one with every
  item ticked

### Requirement: Every engine answers a checklist term alike

Both storage engines and the in-memory matcher SHALL select the same tasks for each term.

#### Scenario: The terminal matches as the stores do

- **GIVEN** the shared corpus the storage engines are tested against
- **WHEN** each checklist term is matched in memory
- **THEN** it selects the same tasks the storage engines select

### Requirement: Default listings leave out snoozed tasks

A task listing that names no snooze term SHALL leave out every task whose wake time is after the
present. Internal readers that walk every task, such as export, copy and project deletion, SHALL
NOT be narrowed by it.

#### Scenario: A snoozed task is not listed

- **GIVEN** OPS-3 is snoozed until tomorrow
- **WHEN** a listing is read with no filter
- **THEN** OPS-3 is not in it

#### Scenario: Export keeps a snoozed task

- **GIVEN** OPS-3 is snoozed until tomorrow
- **WHEN** the tenant is exported
- **THEN** the export holds OPS-3 with its wake time

### Requirement: The filter language selects snoozed tasks

The filter language SHALL accept `is:snoozed` and `snoozed:true` selecting only snoozed tasks;
`-is:snoozed`, `is:awake` and `snoozed:false` selecting only tasks not snoozed; and `snoozed:any`
listing both. The CLI SHALL take `--snoozed`, `--snoozed=false` and `--include-snoozed` for the
same three, and the HTTP API `snoozed=true`, `snoozed=false` and `snoozed=any`. The terminal
interface SHALL answer the same terms against a loaded page.

#### Scenario: Only snoozed tasks

- **GIVEN** OPS-3 is snoozed and OPS-4 is not
- **WHEN** a listing is read with `is:snoozed`
- **THEN** it holds OPS-3 and not OPS-4

#### Scenario: Both

- **GIVEN** OPS-3 is snoozed and OPS-4 is not
- **WHEN** a listing is read with `snoozed:any`
- **THEN** it holds both

#### Scenario: Unknown snoozed value

- **WHEN** a listing is read with `snoozed:maybe`
- **THEN** it fails as a usage error
