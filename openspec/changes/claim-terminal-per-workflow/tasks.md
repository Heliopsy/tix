# Tasks

## 1. Contract

- [x] 1.1 `internal/store/store.go`: `WorkflowTerminal{WorkflowID, States}`, and `ClaimNextRow.TerminalStates`
      replaced by `Terminal []WorkflowTerminal` so every call site has to say which workflow it means
- [x] 1.2 `internal/store/store.go`: `ClearClaim` returns `(bool, error)`, false meaning the row no longer
      carries the lapsed lease the caller read

## 2. Dialects

- [x] 2.1 `internal/store/postgres/claim.go`: `terminalPredicate` resolves the row's project to its
      workflow through a correlated subquery, `blockedByDependency` applies it to `dep`, and the candidate's
      own status is judged the same way
- [x] 2.2 `internal/store/sqlite/claim.go`: the same predicate, rendered identically
- [x] 2.3 both engines: `ClearClaim` re-asserts `lease_expires_at IS NOT NULL AND lease_expires_at <= ?`,
      and separates a missing task from a re-leased one on the miss path

## 3. Service

- [x] 3.1 `internal/service/claim.go`: `claimTerminal` lists every workflow in the tenant, because a
      dependency's workflow is not constrained by the filter
- [x] 3.2 `internal/service/claim.go`: `sweepOne` skips a row it no longer owns, reports whether it cleared,
      and `SweepLeases` counts only what it cleared
- [x] 3.3 `internal/service/claim.go`: the revert is built from a read taken after the clear, under the row
      lock the clear holds
- [x] 3.4 `internal/service/claim.go`: `claimScope` states the not-found for a project-scoped actor whose
      project is not there, rather than leaving it to a workflow lookup that no longer happens
- [x] 3.5 `internal/bench/ops.go`: the benchmark names its fixture's workflow

## 4. Guards

- [x] 4.1 `internal/service/claim_test.go`: a second project under a workflow where `done` is a waypoint and
      `shipped` is terminal; every new guard uses it
- [x] 4.2 `internal/service/claim_test.go`: a dependency judged by its own workflow, both directions
- [x] 4.3 `internal/service/claim_test.go`: the unscoped queue offers a task terminal only elsewhere, and
      still skips one terminal in its own workflow
- [x] 4.4 `internal/service/claim_test.go`: the sweep skips and does not count a row reported as re-leased
- [x] 4.5 `internal/service/claim_test.go`: the sweep does not write back the snapshot its listing took
- [x] 4.6 `internal/store/postgres/claim_test.go`: two real transactions in two goroutines, ordered by
      channels, over the sweep-versus-claim interleave
- [x] 4.7 `internal/store/{postgres,sqlite}/claim_test.go`: `ClearClaim` refuses a live lease and still
      reports not-found for a task that is not there
- [x] 4.8 `internal/store/{postgres,sqlite}/claim_test.go`: the per-workflow predicate, with one query
      carrying two workflows that disagree
