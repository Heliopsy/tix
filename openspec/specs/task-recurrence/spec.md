# task-recurrence Specification

## Purpose
TBD - created by archiving change tasks-repeat-when-done. Update Purpose after archive.

## Requirements

### Requirement: A task may carry a recurrence rule

A task SHALL optionally carry a recurrence rule, settable when the task is created and changeable or
removable by an update. The rule SHALL be accepted as text in the documented syntax and SHALL be stored,
returned and shown in its single canonical spelling. A task without a rule SHALL behave exactly as before.

An update that does not mention the rule SHALL leave it unchanged; an update supplying an empty rule SHALL
remove it.

#### Scenario: Create a recurring task

- **WHEN** a task is created with the rule `weekly on thu,mon`
- **THEN** the task is returned carrying the rule `every week on mon,thu`

#### Scenario: Remove a rule

- **WHEN** an update supplies an empty rule for a task carrying `every day`
- **THEN** the task is returned carrying no rule

#### Scenario: An unrelated update keeps the rule

- **WHEN** an update changes only the title of a task carrying `every month`
- **THEN** the task still carries `every month`

### Requirement: The rule syntax is strict and canonical

The parser SHALL accept `every [N] UNIT` with UNIT one of day, week, month or year (singular or plural),
N from 1 to 1000, the shorthands `daily`, `weekly`, `monthly` and `yearly`, an optional `on` clause
naming weekdays for a weekly rule or `on day D` (1 to 31) for a monthly rule, an optional `after done`
anchor, and at most one end: `until YYYY-MM-DD` or `for N times` (1 to 1000). The leading `every` MAY be
omitted only in the form `N UNIT after done`. Words SHALL be matched case-insensitively.

Anything else SHALL be refused as invalid input with a message naming the problem: an unknown word, a
clause given twice, weekdays on a rule that is not weekly, a day of month on a rule that is not monthly,
both `until` and `for`, or a number out of range.

Rendering a parsed rule SHALL produce its canonical spelling, and parsing a canonical spelling SHALL
reproduce it exactly.

#### Scenario: Shorthand and long form agree

- **WHEN** `monthly` and `every 1 month` are parsed
- **THEN** both render as `every month`

#### Scenario: After done without every

- **WHEN** `3 days after done` is parsed
- **THEN** it renders as `every 3 days after done`

#### Scenario: Weekdays on a monthly rule

- **WHEN** `every month on mon` is parsed
- **THEN** it is refused, naming that weekdays apply only to a weekly rule

#### Scenario: Two ends

- **WHEN** `every day until 2027-01-01 for 3 times` is parsed
- **THEN** it is refused, naming that a rule takes one end

#### Scenario: Round trip

- **WHEN** any canonical rule is parsed and rendered
- **THEN** the result equals the input

### Requirement: Finishing a recurring task creates the next occurrence in the same transaction

When a task carrying a rule and no successor moves into a state that completes work, the service SHALL
create the next occurrence in the same transaction as the move, writing its row, its audit entry and its
`task.created` event alongside the move's own. A state completes work when it is terminal and its category
is `done` or unset. If any part of the write fails, neither the move nor the new task SHALL be stored.

This SHALL apply to an ordinary transition, to every hop of a route, and to a lease release that names a
final status.

#### Scenario: Completing a weekly task

- **WHEN** a task due Monday 2 November carrying `every week` is moved to `done` on that Monday
- **THEN** a new task with the same title exists in the workflow's initial state, due Monday 9 November, carrying `every week`
- **AND** the completed task names the new task as its successor

#### Scenario: A failed write stores neither

- **WHEN** creating the next occurrence fails inside the transaction
- **THEN** the original task is still in its previous state and no new task exists

#### Scenario: A release that finishes the task

- **WHEN** a lease holder releases a recurring task with final status `done`
- **THEN** the next occurrence exists

### Requirement: Cancelling a recurring task ends the series

Moving a recurring task into a terminal state whose category is `cancelled`, or any other category than
`done`, SHALL NOT create an occurrence. Neither SHALL creating a task directly in a done state, importing a
snapshot, an external sync, or a lease expiry.

#### Scenario: Cancel

- **WHEN** a task carrying `every day` is moved to `cancelled`
- **THEN** no new task is created

#### Scenario: Created already done

- **WHEN** a task is created with status `done` and a rule
- **THEN** no further task is created

### Requirement: A task spawns at most once

A finished occurrence SHALL record its successor, and a task that records one SHALL NOT create another,
however many times it is reopened and finished again, and whatever its rule is later changed to. The
record SHALL survive the successor being deleted, soft or hard, and SHALL NOT be settable through any
input.

#### Scenario: Reopen and finish again

- **WHEN** a recurring task that already spawned is moved back to `todo` and then to `done` again
- **THEN** no second successor is created

#### Scenario: Successor purged

- **WHEN** the successor is hard-deleted and the predecessor is reopened and finished again
- **THEN** no new task is created

### Requirement: The next due date follows the rule's anchor

For a rule without `after done` and a task with a due date, the next due date SHALL be the first
occurrence of the rule counted from the previous due date that falls after both the previous due date and
the completion moment. For an `after done` rule, or a task with no due date, the rule SHALL be counted
from the completion date, at the previous due date's time of day or at midnight when there was none.

Occurrences SHALL be computed from their origin in the origin's location, preserving its wall-clock time.
A monthly rule SHALL keep the origin's day of month, or the `on day` day, clamped to the end of a shorter
month; a yearly rule SHALL keep month and day, clamping 29 February. Weeks SHALL start on Monday, and a
weekly rule with weekdays SHALL take the listed weekdays of every Nth week counted from the origin's week.

#### Scenario: Finished late on a schedule

- **WHEN** a task due 1 October carrying `every week` is finished on 20 October
- **THEN** the next occurrence is due 22 October

#### Scenario: After done

- **WHEN** a task due 1 October carrying `every 3 days after done` is finished on 20 October
- **THEN** the next occurrence is due 23 October

#### Scenario: Undated

- **WHEN** a task with no due date carrying `every 2 weeks` is finished on 20 October
- **THEN** the next occurrence is due 3 November at midnight

#### Scenario: Month end

- **WHEN** a task due 31 January carrying `every month` is finished that day
- **THEN** the next occurrence is due 28 February, or 29 February in a leap year

#### Scenario: Weekdays

- **WHEN** a task due Monday carrying `every 2 weeks on mon,thu` is finished that Monday
- **THEN** the next occurrence is due that Thursday

#### Scenario: Daylight saving

- **WHEN** a daily occurrence at 09:00 local time is advanced across a daylight saving change in that location
- **THEN** the next occurrence is at 09:00 local time

### Requirement: A rule may end

A rule ending `until DATE` SHALL produce no occurrence whose due date falls on a later calendar date. A
rule ending `for N times` SHALL count the occurrences left including the current one: finishing an
occurrence with N greater than 1 SHALL create a successor carrying `for N-1 times`, and finishing one with
`for 1 time` SHALL create nothing. Occurrences skipped by a late completion SHALL NOT count.

#### Scenario: Count runs out

- **WHEN** a task carrying `every day for 2 times` is finished, and then its successor is finished
- **THEN** the successor carries `every day for 1 time`, and finishing it creates nothing

#### Scenario: Until passed

- **WHEN** a task due 30 June carrying `every week until 2027-07-05` is finished
- **THEN** no new task is created

### Requirement: The successor copies the work and not its history

The successor SHALL carry the predecessor's project, title, description, priority, assignee, parent, tags,
custom field values and rule. It SHALL start in its workflow's initial state, be created by the actor who
finished the predecessor, and SHALL NOT carry comments, artifacts, dependencies, a lease, claim evidence,
started or completed times, or a successor link.

#### Scenario: What is copied

- **WHEN** a recurring task with tags, an assignee, a custom field value, a comment and a dependency is finished
- **THEN** the successor has the same tags, assignee and field value, and no comment and no dependency
