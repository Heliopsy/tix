## ADDED Requirements

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

A named window SHALL be resolved against the moment the expression is answered. `due:overdue` SHALL set
the upper bound to that moment.

The term SHALL set the same upper and lower bounds `due-before:` and `due-after:` set, and those spellings
SHALL keep working unchanged.

The term SHALL NOT be negatable, because it names a shape of the listing rather than a set of tasks.

A value that is neither a known window nor a parsable bound SHALL be refused as invalid input, naming the
windows it accepts, rather than being dropped.

#### Scenario: The shorthand bounds the listing at now

- **WHEN** a filter expression carries `due:overdue`
- **THEN** the upper deadline bound is the moment the expression was answered
- **AND** no lower bound is set

#### Scenario: A window bounds the listing at its far end

- **WHEN** a filter expression carries `due:week`
- **THEN** the upper deadline bound is seven days after the moment the expression was answered

#### Scenario: The bounded form takes a date

- **WHEN** a filter expression carries `due:<2026-04-01`
- **THEN** the upper deadline bound is that date
- **WHEN** a filter expression carries `due:>2026-04-01`
- **THEN** the lower deadline bound is that date

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
