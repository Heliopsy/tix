# Retrying a write safely

## Why

tix-mobile queues writes while it is offline and replays them when it reconnects. A write whose response
was lost on a flaky network looks exactly like one that never arrived, so the app cannot tell whether
replaying it would create a second task or a second comment. The API needs a way to say "this is the same
request as before" and get the first answer back instead of a second change.

## What Changes

- Every mutating route under `/api/v1` (POST, PUT, PATCH and DELETE) accepts an `Idempotency-Key` header,
  1 to 200 printable ASCII characters, scoped to the tenant, the authenticated actor and the key.
- The first request with a key runs. Its status, body and content type are kept for a window,
  `retention.idempotency_keys`, 24 hours by default.
- A repeat with the same method, path, query and body answers the kept response without running again,
  marked `Idempotent-Replayed: true`.
- The same key with a different request answers `422` with code `invalid`. A repeat while the first is still
  running answers `409` with `Retry-After`.
- The write a request makes and the record that it ran commit in one transaction, so a crash between the
  write and the response never lets a retry apply the change twice.
- The retention pruner removes expired keys, and `tix prune` reports how many.
- Without the header nothing changes.

## Capabilities

### Modified Capabilities

- `http-api`: the `Idempotency-Key` header and its answers.
- `retention`: expired idempotency keys are pruned.
- `configuration`: `retention.idempotency_keys`.

## Impact

`internal/store` (table, `IdempotencyTx` on both engines, migration), `internal/service` (reserve, finish,
the mark inside `write`, pruning), `internal/httpapi` (middleware), `internal/core` (`PruneResult` count),
`internal/config`, `internal/openapi` (header parameter on every mutating operation), `internal/mcp` (header
parameters are not tool arguments), `api/openapi.yaml`, `docs/api.md`, `docs/configuration.md`, the skill.
