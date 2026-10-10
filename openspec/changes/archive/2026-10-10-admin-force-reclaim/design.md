# Design

## Reclaim frees the lease; it does not hand it to the administrator

"Force reclaim" could mean two things: the administrator becomes the new holder, or the lease is ended
and the task returns to the queue. It ends the lease.

Becoming the holder would mint a lease token for a person, which is the thing the browser and the
terminal cannot be given -- the existing web exemptions for `ClaimTask`, `RenewLease` and `ReleaseLease`
all say so, and each of them says it about a token a browser session does not hold. An operation whose
result is a secret the operator cannot use is an operation that cannot be bound on every surface, and
this one has to be: a stranded claim is noticed in a browser or a terminal at least as often as in a
script.

Ending the lease also matches what the operator actually wants. The task is not theirs to do; it is
somebody else's work that stopped. Freeing it lets the next `tix claim next` pick it up, which is the
outcome, and it needs no second call to release what the reclaim just took.

## The status does not move, and that is the whole of the decision

The sweeper does two things when a lease lapses: it clears the lease, and it consults
`revert_on_lease_expiry` to put the task back where a dropped task belongs. Reclaim does only the first.

This is deliberate and it is the part most likely to be argued with, so: a reclaim that also moved the
task would put a transition in the audit trail under an action whose name does not mention one. An
operator reading the history afterwards would see the task in a state nobody chose, attributed to the
administrator who pressed the button. Worse, it would make "get this task unstuck" and "reopen this
finished task" one gesture, so the second could not be refused separately, could not be scoped
separately, and could not be read separately.

So a terminal task can be reclaimed and stays terminal. That is not an oversight: a finished task
holding a live lease is precisely a case where the lease is stuck and the status is correct.

## The scope is its own, and it is not `tenant:admin`

`ActionServerRead` reuses `tenant:admin` with a note saying a scope of its own would be a second name
for the same authority. That reasoning does not reach here. Reading the installation *is* administering
the installation. Ending one task's lease is not administering the tenant: it touches one row in one
project and nothing else the tenant owns.

`task:reclaim` can therefore be granted to an on-call operator, or to an orchestrator that supervises
its own agents, without also granting the authority to rewrite retention, mint tokens or read every
project. The admin role holds it anyway through `*`, so an administrator needs nothing new.

It is not folded into `task:claim` for the opposite reason: every worker holds that one, and a worker
able to end another worker's lease can end work it knows nothing about.

It is project-confinable. Everything a reclaim reaches belongs to one project, and `Policy.Can` holds a
pinned token inside its own project on the way through, so the allow-list entry is safe in the sense
that list's own comment means.

## The evidence is the audit entry, not a pair of columns

`ClearClaim` writes `lease_expired_at` and `lease_expired_by` in the same statement that clears the
lease, so a task an agent took and abandoned does not read like one nobody ever touched. Migration
0007's own comment says what those columns are for: spotting an agent that keeps dying.

A forced reclaim writes neither. Recording an operator's decision in the "the holder stopped answering"
columns would poison exactly the signal they exist to carry: a team that reclaims stuck tasks by hand
would watch its agents appear to be failing. The row after a reclaim reads as unclaimed, which is what
it is.

What happened lives in the audit entry and the outbox event, which is where an operator reads history
anyway:

- action `task.reclaim`, distinct from `task.lease_expire`
- event `core.EventTaskReclaimed` (`task.reclaimed`), distinct from `task.lease_expired`, so a
  subscriber is not forced to read every override as an agent failure
- payload `forced: true`, `previous_holder`, `previous_holder_handle`, `status` (the state the task
  kept), and `reason` when the operator gave one
- the before/after snapshots the audit writer already records, which show the lease columns emptied and
  the status unchanged

The trade-off accepted: answering "was this task reclaimed recently" from a listing costs an audit
query, where the same question about an expiry is a column on the row. The listing question that
matters -- "is work being dropped" -- is the expiry one, and it keeps its cheap read.

## The compare-and-swap re-asserts the holder

Claim is a compare-and-swap. So is this. The predicate is:

    id = ?  AND deleted_at IS NULL
      AND claimed_by_actor_id = <the holder the caller read>
      AND lease_expires_at IS NOT NULL AND lease_expires_at > <now>

Keying on the row identifier alone is the sweeper bug that was fixed earlier: a write that does not
re-assert what the caller believed reports success over a state the caller never saw.

What each race does:

- **Two administrators force at once.** Both read holder A. The first commits and the row becomes
  unclaimed. The second matches nothing, and is told the lease is no longer the one held by A. One
  winner, and the loser learns something true rather than being congratulated.
- **An administrator forces while the holder renews.** A renew moves `lease_expires_at` and leaves
  `claimed_by_actor_id` and `lease_token` alone, so whichever order the two transactions serialize in,
  the force still matches and wins. This is the right way round: a healthy agent renewing on a timer
  must not be able to starve an administrator out of the override.
- **The holder released and somebody else claimed in between.** `claimed_by_actor_id` is now B, the
  predicate fails, and the administrator is refused rather than silently ending B's brand-new lease.
- **The lease lapsed between the read and the write.** `lease_expires_at > now` fails and the caller is
  refused. The task is already free; there is nothing to force.

A task that holds no live lease at all is refused before the statement runs, as a conflict naming that
fact, rather than being quietly treated as a successful reclaim of nothing.

## The previous holder's token

Nothing special is written for this. After the reclaim the row carries no `lease_token` and no
`claimed_at`, so `requireLeaseToken` takes its unclaimed branch, sees a token offered anyway, and
returns `staleLease`, which is a lease expiry. That is the single surviving guard and the semantics it
already states: a token presented for a task the caller does not hold is a lease expiry, answered by
claiming again rather than by retrying. The forced-away holder meets exactly the path an expired holder
meets, which is the point -- there is no second way for a token to die.
