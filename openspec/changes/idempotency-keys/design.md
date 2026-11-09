# Design

## Where the key lives

The key is an HTTP concern, so the HTTP layer reads, validates and hashes it, and it is not a `Service`
method with a registry entry: no CLI command, web screen or TUI binding has a reason to send one. The rules
(validation, what a stored row means, the window) live in `internal/service` as two methods on `Local`,
`BeginIdempotent` and `FinishIdempotent`, which the router finds by type assertion on its service, the way
it finds the tenant resolver. A router over a service without them ignores the header.

A key is scoped to the tenant, the actor the credential resolves to, and the key itself. Two tokens of
the same actor share keys; two actors never do.

## What is compared

The fingerprint is a SHA-256 over the method, the request URI (path and query) and the body bytes. Headers
are not part of it: a client that changes `Accept` between attempts still gets the first answer.

## Atomicity

The handler calls a service method that opens its own transaction, and the response body only exists after
that transaction has committed, so the final response cannot be written in the mutation's transaction.
Two writes do the job instead:

1. `BeginIdempotent` commits a row in state `reserved`, with a lease of twice the request timeout.
2. The request context carries the row's identifier. `Local.write`, the one door to a writable tenant
   transaction, marks the row `executed` inside the mutation's own transaction. The change and the record
   that it happened commit together or not at all.
3. After the handler answers, `FinishIdempotent` stores the status, content type and body and moves the
   row to `done`.

A crash between 2 and 3 leaves an `executed` row with no response. A retry then gets `409` without
`Retry-After` ("applied but its response was not kept") instead of running again; the client reads the
resource to learn the outcome. A crash before 2 leaves a `reserved` row; once its lease lapses a retry
takes the key over and runs, which is safe because nothing committed.

The alternative, the service writing the whole response inside its transaction, would need every service
method to know its HTTP rendering. Reserving in the router and finishing after the handler, with only the
mark inside the transaction, keeps the HTTP shape in `internal/httpapi` and costs one small update per
write.

A request that makes several transactions (bulk, import) marks the row in the first one; the rest find it
already marked. Writes outside a tenant transaction (tenant and account administration through the
unscoped door) do not mark the row, so for those a lost response after a failure can run again.

## Which responses are kept

- A request that committed a write keeps its response whatever the status, a `5xx` included, because
  running it again would apply the change twice.
- A request that committed nothing keeps a `2xx` and releases the key on anything else, so a validation
  error, a conflict or a server error can be retried with the same key once the cause is fixed.
- A response over 1 MiB is not kept; the key then behaves as applied without a kept response.

## Status codes

`invalid` maps to `400` everywhere else, but the router writes the reuse error at `422` with code
`invalid`, through the same envelope helper it uses for `413` and `415`. A client can then tell "your key
is malformed" (`400`) from "your key belongs to another request" (`422`) by status alone.

## Expiry and pruning

Each row carries `expires_at`, set to the window after it is finished (or reserved). A lookup treats an
expired row as absent, so the window holds without the pruner. The pruner deletes expired rows in id
batches selected by `expires_at` order through an index on `(tenant_id, expires_at)`, no `OFFSET`. The
table is tenant scoped, so PostgreSQL gives it row-level security from the migration that creates it.

## Clients

The CLI and `internal/client` do not retry, so they send no key. `tix mcp` does not offer the header as a
tool argument: a model's tool call is not replayed by a queue, and a key it invented would only add a way
to fail.

## Known follow-ups

Two gaps are left open on purpose and should be closed in later changes:

- **Writes through the unscoped door do not mark the row.** Tenant and account administration commit
  outside `Local.write`, so the claim is never marked `executed`. If such a request commits and then
  answers a `5xx`, the key is released as if nothing ran; if its response is lost, the row stays
  `reserved` until the lease lapses. Either way a retry runs the write again.
  Closing it means marking the claim from the unscoped transaction helper as well, with the tenant the
  claim was reserved in.
- **Bulk and import mark only their first transaction.** A request that commits in several transactions
  marks the row in the first one. A crash after a later commit and before the response is kept still
  answers `409` without `Retry-After`, which is safe, but the kept outcome cannot say how far the request
  got. Closing it means either one transaction per request or recording progress on the claim.
