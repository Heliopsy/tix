# Tasks

## Store

- [ ] Migration creating `idempotency_keys` with the key and expiry indexes
- [ ] `idempotency_keys` in the scoped tables, so the builder and PostgreSQL row-level security cover it
- [ ] `store.IdempotencyTx` on SQLite and PostgreSQL, with tests on both

## Service

- [ ] `BeginIdempotent` and `FinishIdempotent` on `Local`, with the window option
- [ ] `Local.write` marks the reserved row inside the mutation's transaction
- [ ] Pruning removes expired keys and reports the count; dry run counts them
- [ ] Service tests on SQLite with `clock.FakeClock`

## Surfaces

- [ ] HTTP middleware: validation, replay, mismatch, in flight, release, `Idempotent-Replayed`
- [ ] HTTP tests: replay, mismatch, in flight, invalid key, no header, isolation, 5xx not kept
- [ ] `retention.idempotency_keys` configuration key, validated and documented
- [ ] Generator adds the header parameter to every mutating operation; `just openapi`
- [ ] MCP leaves header parameters out of tool arguments
- [ ] `docs/api.md` "Retrying safely", `docs/configuration.md`, the tix skill
