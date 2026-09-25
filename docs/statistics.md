# Statistics

Throughput, lead time, where the work is sitting, who moved it, and what has waited longest. One window,
one tenant, optionally one project, on all four surfaces.

```sh
tix stats                        # the whole tenant, last 14 days
tix stats -p infra --window 30d  # one project, a longer window
tix stats --since 2026-01-01     # from a fixed instant instead of a length
tix stats -o json                # every figure the screen shows
```

The browser has the same numbers at `/stats`, and the terminal interface opens them with `S`.

`--window` takes Go's duration syntax plus a day of exactly 24 hours, so `30d`, `720h` and `16d 1h` all read,
and `--since` takes RFC 3339 or `YYYY-MM-DD`. Passing both is a usage error, since they contradict each other.
That is the same vocabulary everywhere the product accepts a duration, and it includes everything the product
prints: a lead time shown as `3d 22h` can be typed back. There is no week unit, because nothing renders one.
The machine formats keep Go's own rendering (`"94h0m0s"`), because a snapshot has to round-trip through it.

## What each figure means

| Figure | Definition |
| --- | --- |
| completed | Tasks that reached a terminal state inside the window |
| created | Tasks created inside the window |
| per day | The completed count, split by the UTC day it happened on |
| median lead time | Creation to terminal state, over tasks that reached one in the window |
| slowest | The longest of those |
| where the work is | How many tasks are *presently* in each state category |
| most active | Actors ranked by tasks they moved to a terminal state |
| waiting longest | The oldest tasks that have not reached a terminal state |

The window always ends now. Soft-deleted tasks appear in no figure. Nothing crosses a tenant.

## What the leaderboard actually counts

**Tasks moved to a terminal state.** Not work done, not effort, not value.

Every surface prints that sentence next to the numbers, and the wording is a constant in the domain
package so the three cannot drift apart. A ticket-closing count presented as a productivity measure is
worse than no leaderboard, and this is the one number on the page that invites being read as something it
is not.

Two consequences worth knowing before anyone reads a ranking:

- Attribution comes from the audit trail, which is the only place that records *who* moved a task. A
  transition old enough to have been pruned by retention reports no actor rather than the wrong one.
- A task created directly into a terminal state is attributed to nobody, because nobody moved it.

## Completion is a moment, not a state

A task counts as completed on the timestamp it entered a terminal state. Moving it back out clears that,
so a task completed and then reopened inside the window is not counted, and only the most recent
completion counts.

This is the same field the board reads, so "where the work is" and "completed" can never disagree about
whether a given task is done.

## An empty window

Every count is zero, every list is empty, and the call succeeds. A window in which nothing happened is an
answer, not an error.

`where the work is` still reports each category, including the zeroes: a board that hides "0 in progress"
reads as a missing figure rather than an empty one.

## What is deliberately not here

**Charts.** The browser draws bars as `div` widths and the terminal draws them with block characters. Six
numbers do not justify the largest dependency in the project, and a static export has no build step to
feed one.

**Burndown, cycle-time percentiles, per-sprint reporting.** None of them mean anything until a team has
used tix for a few weeks, and inventing them now would be guessing at a workflow nobody has yet.

**Estimates and velocity in points.** tix has no estimate field. Counting tasks is honest about what it
measures; counting points nobody entered would not be.

## See also

- [filtering.md](filtering.md) for narrowing a board, which is a different question from measuring one
- [scripting.md](scripting.md) for `-o json` and piping figures into something else
