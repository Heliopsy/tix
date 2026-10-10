# Tasks

## 1. Service

- [x] 1.1 `internal/service/claim.go`: `sweepOne` records which state the revert left, and emits
      `EventTaskTransitioned` with `from`, `to`, `ref` and `reason: "lease_expiry"` after the expiry event,
      only when a status was actually written
- [x] 1.2 `internal/service/claim.go`: the transition event goes out without an audit entry of its own, with
      the reason stated beside it

## 2. Guards

- [x] 2.1 `internal/service/claim_test.go`: a consumer replaying `task.transitioned` alone arrives at the
      reverted status, and an expiry that reverted nothing announces no transition
- [x] 2.2 `internal/service/claim_test.go`: the transition payload carries both ends of the move, the ref and
      the reason
- [x] 2.3 `internal/service/claim_test.go`: the sweep leaves one audit entry per task, under the expiry
      action and not the transition action
