# Tasks

## 1. Authority

- [x] 1.1 `internal/core/identity.go`: `ScopeTaskReclaim` (`task:reclaim`) and its entry in `AllScopes`
- [x] 1.2 `internal/authz/action.go`: `ActionTaskReclaim`, its scope mapping, its place in `allActions`
      and its entry in `projectConfinable`

## 2. Core contract

- [x] 2.1 `internal/core/input.go`: `ForceReclaimInput` with an optional reason and its `Validate`
- [x] 2.2 `internal/core/types.go`: `EventTaskReclaimed` (`task.reclaimed`) in the event vocabulary
- [x] 2.3 `internal/core/service.go`: `ForceReclaim` on `ClaimService`

## 3. Store

- [x] 3.1 `internal/store/store.go`: `ForceReclaimRow` and `ForceReclaim` on `ClaimTx`
- [x] 3.2 both engines: the conditional update, keyed on the task, the holder the caller read and a live
      lease, writing no expiry evidence
- [x] 3.3 `internal/store/postgres/reclaim_test.go`: two real transactions in two goroutines produce one
      winner, and the reclaimed token cannot renew
- [x] 3.4 `internal/store/sqlite/reclaim_test.go`: the predicate on its own, a lapsed lease refused, and
      only the lease columns moved

## 4. Service

- [x] 4.1 `internal/service/reclaim.go`: `ForceReclaim`, authorized on `ActionTaskReclaim`, refusing a
      task with no live lease, recording `task.reclaim` / `task.reclaimed` with the forced marker, the
      previous holder and handle, the kept status and the reason
- [x] 4.2 `internal/service/reclaim.go`: no status write and no `revert_on_lease_expiry` consultation
- [x] 4.3 `internal/service/reclaim_test.go`: a member is refused by name, the previous holder's token is
      refused on renew, release and transition, a terminal task and a reverting state both stay put, the
      trail is a reclaim and not an expiry, a stale holder is refused, and a renewal does not defeat it

## 5. Surfaces

- [x] 5.1 `internal/wire/wire.go` and `internal/httpapi/handlers_claim.go`:
      `POST /api/v1/tasks/{ref}/claim/reclaim`
- [x] 5.2 `internal/client/claims.go`: the remote implementation
- [x] 5.3 `cmd/claim.go`: `tix claim reclaim REF --reason --dry-run`
- [x] 5.4 `internal/web`: the route, the handler, `CanReclaim` on the task view and the control on the
      task screen, offered only beside a live lease
- [x] 5.5 `internal/web/history.go`: the reclaim's sentence in the task history
- [x] 5.6 `internal/tui`: the `Reclaim` binding on `F`, its action, command and dispatch, and its place in
      the board and detail key sets, offered over a task another worker holds
- [x] 5.7 `internal/capability/registry.go`: `claim.reclaim` with all four bindings
- [x] 5.8 `internal/integration/service_matrix_test.go`: a transport-equivalence scenario

## 6. Documentation

- [x] 6.1 `docs/agents.md`: how to take a lease back, what it does not do, and `task:reclaim` in the
      scope table
- [x] 6.2 `docs/api.md`: `task.reclaimed` in the event vocabulary
- [x] 6.3 `docs/tui.md`: the `F` binding and the operation count the registry now holds
- [x] 6.4 `skills/tix/SKILL.md`: the reclaim beside the lease rules an agent has to follow
