# A lease expiry that reverts a status announces the transition

## Why

The lease sweeper clears an expired claim and, when the workflow state carries `revert_on_lease_expiry`,
moves the task back. It emitted one event for that: `task.lease_expired`, with the destination tucked
into the payload as `reverted_to`. It never emitted `task.transitioned`.

A consumer of the event stream rebuilds task status from transitions. That is what the taxonomy promises:
a task that moves from one workflow state to another emits a task transitioned event rather than a
generic update. The sweeper's revert is exactly such a move, and it was the one status change in the
system that no transition event described. A webhook subscriber filtered to transitions, an external sync
mirroring status, or anything replaying the stream to reconstruct a board, all went on holding the task
in the state its dead holder left it in. There was no way to notice without knowing to read another
event type's payload for a field that only sometimes exists.

The alternative considered was keeping one event and calling its payload sufficient. It is not:
`reverted_to` is a field on an event type whose name says "a lease expired", so every consumer would have
to special-case it, and a consumer that does not know the field exists cannot be told by the stream that
it is missing something. A status change that is announced only to readers who already know about it is
not announced.

## What Changes

- **A revert emits `task.transitioned` as well as `task.lease_expired`**, in that order, in the same
  transaction as the row and the audit entry. The expiry says the holder stopped answering; the
  transition says where the task went.
- **The transition payload is the ordinary one**: `from`, `to` and `ref`, so an existing consumer needs no
  new case, plus `reason: "lease_expiry"` for one that wants to tell a sweep apart from a person.
- **An expiry that reverts nothing emits only `task.lease_expired`.** A task in a state without
  `revert_on_lease_expiry` did not move, and announcing a transition to the state it is already in would
  be a lie the consumer has to filter.
- **`reverted_to` stays on the expiry payload.** Consumers already read it and it is still true.
- **No second audit entry.** The audit log records operator actions, and the expiry entry already carries
  both snapshots of this one change. A second `task.transition` entry would read as a second action, and
  the browser history view groups consecutive `task.transition` entries into a single route, so the sweep
  would have rendered as somebody walking the task through states.

## Impact

- Affected specs: `event-stream`
- Affected code: `internal/service/claim.go` (`sweepOne`), `internal/service/claim_test.go`
- No schema change, no new event type, no new payload field on an existing event.
- Behaviour change on the wire: a sweep that reverts now publishes two events where it published one. A
  consumer that counted events per sweep sees a different number; one that applies them sees the truth.
