# Design

## Per task, compared at read time

`tasks.snoozed_until` (an instant) and `tasks.snoozed_by` (the actor). A task is snoozed when
`snoozed_until > now`. Every predicate compares against the store clock at the moment of the read,
so nothing has to run for a task to reappear: a CLI working straight on a SQLite file with no
server sees the task come back on time.

Snoozing does not bump the task's version or rewrite its fields through `UpdateTask`; it is its own
narrow store write, like a claim. An edit form open on the task keeps working, and an `UpdateTask`
read before the snooze cannot clobber it.

## Where the default lives

The store answers `Snoozed` as a plain tri-state: `Either` adds no predicate. The default that
hides snoozed tasks is applied by `Service.ListTasks`, the listing every surface goes through, when
the caller neither asked for snoozed tasks nor asked to include them. Internal readers that walk
every task (export, copy, project deletion, workflow checks) call the store directly and keep
seeing all of them, so snoozing cannot make a task vanish from a backup or from a project move.

| Filter | Listing |
| --- | --- |
| nothing | tasks not snoozed |
| `is:snoozed`, `snoozed:true`, `--snoozed` | only snoozed tasks |
| `-is:snoozed`, `is:awake`, `snoozed:false`, `--snoozed=false` | tasks not snoozed (explicit) |
| `snoozed:any`, `--include-snoozed`, `snoozed=any` | both |

`is:snoozed` narrows, unlike `is:deleted`, which widens. "Show me what I parked" is the question
people ask about snoozes; there is no equivalent "show me only what I deleted" in that spelling,
so the widening spelling for snoozes is `snoozed:any`.

Reading one task by reference (`task show`, the task screen) always works on a snoozed task, and a
subtask list inside a task screen draws snoozed children with their badge.

## Waking

The server already runs a lease sweeper on a ticker under the system actor. A second sweeper built
from the same `lease.Sweeper` calls `WakeSnoozed`, which reads up to a batch of tasks whose
`snoozed_until <= now`, clears each with a conditional update (`snoozed_until` unchanged since the
read), and records `task.wake` with `task.woke` in the same transaction. A task unsnoozed or
re-snoozed between the read and the write is skipped. `tix task wake` and
`POST /api/v1/snoozes/wake` run one pass by hand, like `tix claim sweep`.

Without a server nothing emits `task.woke`, and the column keeps its past value until the next
sweep or snooze; every reader treats a past value as not snoozed.

## Claims

A snoozed task is not work anybody should start, so `ClaimNext` adds
`snoozed_until IS NULL OR snoozed_until <= now` to its candidate query and `ClaimTask` refuses a
snoozed task with a precondition error naming the wake time. Snoozing a task that is already
claimed is allowed and leaves the lease alone: the holder may finish or release it.

## Times

`query.ParseUntil(value, now, zone)` is shared by the CLI and the TUI. It takes the presets
`later` (three hours on, rounded up to the hour), `tomorrow` (09:00 tomorrow), `next-week` (09:00
next Monday), a duration (`90m`, `2h`, `3d`, `1w`), or a date or timestamp. A date alone means
09:00 that day, a timestamp without a zone is read in the given zone. The web resolves the same
presets in the reader's chosen time zone. The time must be in the future.

## Web

The task screen's title bar gets a Snooze menu (or Unsnooze, with the wake time, when snoozed).
The lane card's more actions menu carries the same presets. A list row has no per-row menu to hold them,
so it carries only the badge; bulk actions are a separate change. Each preset is
a plain form posting `until=<preset>`; "Pick a date" is a `datetime-local` input in the same form.
A snoozed task shown in a listing (only under `is:snoozed` or `snoozed:any`) and on its own screen
carries a badge "Snoozed until <time>".
