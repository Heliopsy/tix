# Design

## The decision: two events, not a richer payload

Two shapes were available.

**One event, sufficient payload.** Keep `task.lease_expired` alone and document `reverted_to` as the
authoritative status change. Cheaper, and it is what the code already almost did. It fails on the reader:
a consumer that subscribes to transitions, which the spec invites it to do, receives nothing for a status
change, and the stream gives it no signal that it has missed one. The failure is silent and permanent,
which is the property that made this worth fixing rather than documenting.

**Two events.** The expiry keeps its meaning — the holder stopped answering — and the transition carries
the status change in the shape every other status change uses. A consumer that already handles
`task.transitioned` gets the revert for free, with no new field and no new code. Chosen.

The cost is that a sweep now publishes two events for one task instead of one, so a consumer counting
events per sweep sees a different number. That is a smaller and louder break than a status that quietly
does not move.

## Ordering

The expiry is emitted first, then the transition. The causal order is: the lease was cleared, and because
it was cleared the status reverted. Both are queued on the same mutation and flushed in order before the
commit, so they take consecutive sequence numbers and a consumer resuming from a cursor cannot see the
second without the first.

## Why no audit entry

`mutation.Record` exists so a caller cannot write an event and forget its audit entry, and this is the
first call site to emit an event without one. The reason is specific rather than convenient:

- The expiry audit entry already holds `before` and `after` snapshots spanning the whole sweep of that
  task, including the reverted status. The change is recorded.
- `internal/web/history.go` groups consecutive `task.transition` audit entries under one actor into a
  single route hop. A sweeper-written entry would have rendered a lease expiry as a person walking the
  task through states.
- The audit log is read as a list of actions somebody took. The sweep is one action; it should leave one
  entry.

## Only when it moved

`revertedFrom` is set only inside the branch that actually wrote a new status, which is already guarded on
the destination differing from the current status and existing in the workflow. An expiry on a state with
no revert rule, or one whose revert target is the state it is already in, emits the expiry alone.
