# Tasks

## Store

- [x] Migration creating `idempotency_keys` with the key and expiry indexes
- [x] `idempotency_keys` in the scoped tables, so the builder and PostgreSQL row-level security cover it
- [x] `store.IdempotencyTx` on SQLite and PostgreSQL, with tests on both

## Service

- [x] `BeginIdempotent` and `FinishIdempotent` on `Local`, with the window option
- [x] `Local.write` marks the reserved row inside the mutation's transaction
- [x] Pruning removes expired keys and reports the count; dry run counts them
- [x] Service tests on SQLite with `clock.FakeClock`

## Surfaces

- [x] HTTP middleware: validation, replay, mismatch, in flight, release, `Idempotent-Replayed`
- [x] HTTP tests: replay, mismatch, in flight, invalid key, no header, isolation, 5xx not kept
- [x] `retention.idempotency_keys` configuration key, validated and documented
- [x] Generator adds the header parameter to every mutating operation; `just openapi`
- [x] MCP leaves header parameters out of tool arguments
- [x] `docs/api.md` "Retrying safely", `docs/configuration.md`, the tix skill
