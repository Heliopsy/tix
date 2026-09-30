# An administrator can take a lease, and taking it moves nothing else

## Why

A held claim strands work. An agent dies without releasing, or a person claims a task and goes away,
and the task is unreachable until the lease runs out. Today the only route back is waiting: `tix claim
sweep` clears a lease that has already lapsed, and nothing at all reaches a lease that has not. A
workflow with a long `default_lease` is exactly the one where this hurts, so the setting that protects
long-running agents is the setting that makes a dead agent expensive.

The authority to end somebody else's lease is not the authority to take a lease. Every worker holds
`task:claim`; a worker that can end another worker's lease can end work it knows nothing about. So this
is a separate scope, refused by name when it is missing.

## What Changes

- **A new operation, `ForceReclaim`**, on `core.Service`, bound on all four surfaces: `tix claim
  reclaim REF`, `POST /api/v1/tasks/{ref}/claim/reclaim`, a control on the browser task screen, and `F`
  on the terminal board and detail views.
- **A new scope, `task:reclaim`**, and the action `task.reclaim` mapped to it. The admin role holds it
  through `*`; the member role does not. It is project-confinable, because a reclaim reaches exactly one
  task.
- **Reclaim only, never reopen.** The operation moves the lease columns and nothing else. A task in a
  terminal state stays terminal. The workflow's `revert_on_lease_expiry` rule, which the sweeper
  consults for a lapse, is deliberately not consulted here. An administrator who wants the task moved
  makes that transition themselves, as a second action that says so in the trail.
- **A distinguishable audit entry.** The action is `task.reclaim`, the event is `task.reclaimed`, and
  the payload carries `forced: true`, the previous holder and their handle, the status the task kept,
  and an optional operator reason. An operator reading the trail can tell "an admin took this" from
  "this expired", which are different facts about different failures.
- **The previous holder's token stops working at once**, exactly as it does after expiry, through the
  existing single guard: `requireLeaseToken` sees an unclaimed task and a token offered for it, and
  reports lease expiry.
- **The compare-and-swap re-asserts the holder the caller read.** `ForceReclaim` in the store is keyed
  on the task identifier *and* `claimed_by_actor_id = <the holder the caller saw>` *and* a live lease.
  Two administrators forcing the same task at once produce one winner; the loser is told the lease is no
  longer the one it read rather than being handed a success over a claim it never saw.

## Impact

- Affected specs: `claim-lease`
- Affected code: `internal/core/identity.go`, `internal/core/input.go`, `internal/core/types.go`,
  `internal/core/service.go`, `internal/authz/action.go`, `internal/service/reclaim.go`,
  `internal/store/store.go`, both dialects' `claim.go`, `internal/wire/wire.go`,
  `internal/httpapi/handlers_claim.go`, `internal/client/claims.go`, `cmd/claim.go`,
  `internal/web/{routes,taskdetail,history}.go` and `templates/task.html`,
  `internal/tui/{keys,scheme,model,commands,messages}.go`, `internal/capability/registry.go`
- No schema change. The reclaim writes no `lease_expired_at` / `lease_expired_by`: those columns mean
  "the holder stopped answering", which is how a repeatedly dying agent is found, and an operator's
  decision recorded in that shape would read as the failure it is not.
- Behaviour change on the wire: `core.Service` gains a method, the event vocabulary gains
  `task.reclaimed`, and the scope vocabulary gains `task:reclaim`.
