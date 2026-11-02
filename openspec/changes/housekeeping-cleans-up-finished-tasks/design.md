# Design

## Archive or trash

Tasks have no archived state; only projects do. Adding one would touch every listing, filter, board and
export. The trash already is "out of default listings, restorable", so a finished task that ages out is soft
deleted into it. Restoring it from the Trash undoes the cleanup.

## The trash being off

With the tenant's trash off a soft delete is permanent, so moving finished tasks "to the trash" would destroy
them. Housekeeping never does that:

- `PutHousekeeping` refuses to turn on the done or cancelled rule, at either level, while the trash is off,
  as `precondition_failed` with `reason: trash_off`.
- The trash can still be turned off later. Every rule then pauses, the trash-emptying one included, because
  the trash setting promises that tasks kept while it is off come back when it is turned on again. The
  preview and both screens say it is paused.

## Policy shape

One row per tenant (`project_id` empty) and one per overriding project. Each rule is a nullable integer of
days: NULL inherits (project rows only; on the tenant row it means off), 0 is off, N is on after N days. Days
rather than durations because the rule is about calendar age, and the screens ask for a number. A project
with no row inherits everything.

## Selection

- Finished: live tasks of the project whose status is a terminal state of the rule's category,
  `completed_at` older than N days, and no live child. A finished parent with open children stays; once the
  children go, it goes on a later tick.
- Trash: soft-deleted tasks deleted more than N days ago. Each is purged with its trashed and expired
  descendants, deepest first; a descendant that is live or deleted more recently is left (its parent link is
  cleared by the existing foreign key).

## Ticks, replicas and bounds

The policy runs after retention pruning on the same ticker, in one write transaction per tick, at most
`limit` tasks per rule (default 200). Every statement is conditional on what selected the row (`deleted_at IS
NULL` and the status for a trash move, `deleted_at < cutoff` for a purge), so a second replica that selected
the same task affects no row, writes no audit entry, and moves on. A tenant whose rules are all off returns
before reading any task.

## Audit and inbox

Each task writes `task.delete` and `task.deleted`, with `hard`, `cleanup` (the rule) and `after_days` in the
payload, under the system actor and source. Deletes never create inbox items, so nobody is notified.

## Considered and skipped

- Stale claims: the lease sweeper already releases them.
- Snoozes that passed: the snooze waker already clears them.
- Webhook deliveries, events, audit: retention already prunes them.
- Read inbox items: they belong to a person, not a project, so a per-project setting does not fit; left for a
  per-person preference.
