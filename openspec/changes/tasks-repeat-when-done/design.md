# Design

## Repeat on completion, not a calendar

Two models exist. A calendar model materialises every future occurrence (or a
window of them) up front; a completion model creates the next one when the
current one is finished. tix takes the second, the way Google Tasks does:

- There is never more than one open occurrence of a series, so a listing, a
  board and an agent's `claim next` see one task, not a fan of copies.
- No scheduler is needed. The spawn happens inside a write that is already
  happening, so it inherits that write's transaction, audit and outbox.
- An occurrence that is never finished never produces a successor. That is the
  right answer: the series is stalled, and the stalled task says so.

## Syntax

A rule is one line of lowercase words. The grammar, informally:

```text
[every] [N] UNIT [on DAYS | on day D] [after done] [until YYYY-MM-DD | for N time(s)]
```

| Piece | Accepted | Notes |
| --- | --- | --- |
| `every` | required, except before `N UNIT after done` | `daily`, `weekly`, `monthly`, `yearly` stand for `every day` and so on |
| `N` | 1 to 1000 | omitted means 1 |
| `UNIT` | `day(s)`, `week(s)`, `month(s)`, `year(s)` | singular and plural both accepted after any N |
| `on DAYS` | `mon`..`sun` or full names, comma separated | weekly rules only |
| `on day D` | 1 to 31 | monthly rules only |
| `after done` | | anchors the next date to completion, see below |
| `until DATE` | `YYYY-MM-DD` | inclusive, last date an occurrence may fall on |
| `for N times` | 1 to 1000 | occurrences left including this one; `for 1 time` |

The parser is strict: an unknown word, a repeated clause, weekdays on a
non-weekly rule, a day-of-month on a non-monthly rule, both `until` and `for`,
or a number out of range is refused with an error naming what was wrong. It
does not guess, because a rule that is read differently from how it was meant
silently produces the wrong task weeks later.

Every rule has one **canonical spelling**, which is what is stored, returned
and shown: `every` first, the count omitted when it is 1, the unit pluralised
to match, weekdays as three-letter names in Monday-first order with no spaces,
then `after done`, then the end. `weekly on thu,mon` is stored as
`every week on mon,thu`; `3 days after done` as `every 3 days after done`.
Parsing a canonical string yields the same string.

A string rather than structured JSON on the wire and in storage: it is the
form a person types and reads, it round-trips through a snapshot and a filter
with no schema, and `internal/core` owns its meaning in one parser.

## Anchors

- **Schedule** (the default). The next due date is the first occurrence of the
  rule, counted from the previous due date, that falls after both the previous
  due date and the moment the task was completed. Finishing a weekly task three
  weeks late produces one task due next week, not three overdue copies. Skipped
  occurrences do not consume a `for N times` count, which counts tasks made.
- **After done** (`after done`). The rule is counted from the completion date
  instead: `every 3 days after done` completed on the 10th is due on the 13th.
- A task with **no due date** is counted from its completion date whichever
  anchor it carries, because there is nothing else to count from.

When counting from the completion date, the time of day is the previous due
date's, or midnight when there was none, so a date-only due date (which the
command line stores as midnight UTC) stays date-only down the series.

## Calendar arithmetic, months and DST

Occurrences are computed in the location of the timestamp they are counted
from, preserving its wall-clock time, with `time.Date` normalisation for days
and weeks. Due dates are stored and entered in UTC, so in practice that is UTC
and there is no daylight saving shift to cross; the core function is still
written and tested against a zoned time, so a caller that hands it local time
keeps the local wall clock across a DST change rather than drifting an hour.

Occurrence *k* is computed from the origin rather than from occurrence *k-1*,
so clamping does not accumulate:

- **Months** keep the origin's day of month, or the `on day D` day, clamped to
  the last day of a shorter month. `every month` from 31 January runs 28 (or
  29) February, 31 March, 30 April. Across a series, though, each occurrence
  becomes the next one's origin, so a plain `every month` that has once been
  clamped to the 28th stays on the 28th. `on day 31` pins it; the docs say so.
- **Years** keep month and day; 29 February clamps to 28 February in a
  non-leap year.
- **Weekdays**: weeks start on Monday. The origin's week is week 0 and the
  rule's weeks are 0, N, 2N…; within each, the listed weekdays in order. The
  next occurrence is the first listed weekday in that sequence after the
  bound, so `every 2 weeks on mon,thu` from a Monday is due that Thursday,
  then the Monday two weeks after the first.

## When a task spawns

Spawning is decided in `internal/service` only, by one function both writers
call: `TransitionTask` (and so every hop of a route) and a release that names
a final status. It spawns when all of these hold:

1. The task carries a rule.
2. The task carries no successor link.
3. The target state **completes work**: a terminal state whose category is
   `done`, or a terminal state with no category, which the rest of the product
   already reads as done. A terminal `cancelled` state does not: cancelling a
   recurring task ends the series, which is how both Google Tasks and a person
   reading the board understand it.
4. The rule has not run out: a `for 1 time` rule, or a next date after the
   `until` date, produces nothing.

Creating a task directly in a done state does not spawn (it records something
already finished), and neither does a snapshot import, an external sync, or a
lease expiry, which only ever reverts to a non-terminal state.

## Never twice

The finished task records its successor in `recurrence_next_id`. The column is
written by the same `UpdateTask` that moves the status, after the successor is
inserted, inside one transaction; if any of it fails, none of it lands.

The link is a plain column with **no foreign key**. A key with `ON DELETE SET
NULL` would let a purge of the successor clear the guard, and re-completing the
predecessor would spawn a second time. Reopening a task leaves the link, so
reopen-and-finish never duplicates; editing the rule on a task that already has
a successor changes the stored rule and spawns nothing more. The link is not
editable through any input.

## What the successor carries

| Copied | Not copied |
| --- | --- |
| project, title, description | comments, artifacts, history |
| priority, assignee | lease, claim count, claim evidence |
| parent | dependencies (either direction) |
| tags | started and completed times |
| custom field values | the successor link |
| the rule (count decremented) | |

The status is the workflow's initial state. The creator is the actor who
finished the predecessor, because the audit entry for the creation names them:
a creator nobody acting in that transaction would be a record that disagrees
with its own history. Dependencies are not copied because a dependency is a
statement about one piece of work waiting on another, and the next occurrence
has not been planned against anything yet.

## Events

The successor is created through the ordinary path, so a `task.created` event
and a `task.create` audit entry are written in the transaction, carrying
`recurs_from` (the predecessor's reference) in the payload. The predecessor's
`task.transitioned` event gains `next`, the successor's reference, so a
subscriber reading either one can find the other.

## External sync and import

An external sync owns the status of what it imports; a remote system closing an
issue is not somebody finishing a tix occurrence, and the remote system has
its own idea of repetition. Sync therefore never spawns and never reads or
writes the rule: an update it applies keeps whatever rule the task carried.

A snapshot carries `recurrence` and `recurrence_next_id` as task fields. Import
stores them as found (the rule re-validated, an unparsable one rejecting that
record), and never spawns. Because the link is not a foreign key, a successor
that was filtered out of a partial export does not break the import.

## Filtering

`is:recurring` selects tasks carrying a rule and `-is:recurring` those without,
on every surface through the one parser; the HTTP listing takes
`recurring=true|false`. A rule is a property of a task, not a value to search
by, so there is no `repeat:` term matching rule text.
