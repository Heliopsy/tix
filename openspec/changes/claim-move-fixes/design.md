# Design

## Dry run is the write, rolled back

The old `task mv --dry-run` resolved each reference with `GetTask` and stopped there, so it could not know
whether the state existed, whether the edge existed, whether the edge needed a comment or a scope, or whether
the task was leased. Copying those checks into the CLI would be a second copy of the rules that drifts.
Instead the service runs the real transition inside its write transaction and returns a sentinel error when
`dry_run` is set, which rolls the transaction back. The answer is the task as the move would leave it, and
every refusal is the one the real move gives, with the same kind and so the same exit code and HTTP status.

A route dry run walks every hop in one transaction and rolls back once, so a later hop is judged against the
state the earlier hops would leave, which separate per-hop dry runs could not do.

Other dry runs that take a status were checked. `claim release` has no `--dry-run`. `task copy --keep-status`
and `task move` take no state name: they keep a state of the same key or fall back to the initial one, so
there is nothing to refuse. Bulk move has no dry run; its refusal of an unknown state now lists the states.

## Claim and start

Opt-in, not the default. The documented worker shape is `claim next` followed by `task mv REF doing`, and
`claim exec --on-start doing` makes the same move itself. Starting by default would turn that second move into
a refusal (`doing` to `doing` is not an edge) and break every script written against the docs. A tenant or
workflow setting was considered and left out: a claim's behaviour depending on a setting the worker cannot see
is the kind of surprise the experiment was about.

The start state is `start_status` when given, else the first state, in the workflow's order, whose category
is in progress and which has an edge from the task's current state. A task already in an in-progress state,
or already in the named state, is left where it is. No such edge, or a named state the workflow lacks or
cannot reach from here, refuses the whole claim: the transaction rolls back and nothing is claimed, so a
worker is never left holding a task in a state it did not ask for. The move runs through the same code as
`TransitionTask`, so the edge's scope and comment rules apply and its audit entry and event are written
beside the claim's.

## The lease token stays required for the holder

The token proves the process holding the lease, not the account. Two workers often share one agent token,
and a worker whose lease lapsed and was re-claimed by its own account is still a zombie. Letting the same
actor move without the token would reopen exactly the hole `requireLeaseToken` closes. The refusal keeps its
kind and exit code, and when the caller is the holder it now says so and names `--lease-token` and the
`lease_token` field.

## Finishing releases the lease

A transition into a terminal state that carried the caller's live token releases the lease after the status
write, in the same transaction, recording `task.release` with `reason: terminal`. A move into a terminal state
while somebody else holds the lease is still refused for want of their token, as before.

A script that moved to `done` and then released would now get `lease_expired` on the release. To keep those
scripts working, a release whose token no longer matches is accepted without writing anything when the task
is unclaimed, in a terminal state, and the release carries no status, result or comment. Anything that would
write is still refused, so a zombie cannot attach a result to finished work.
