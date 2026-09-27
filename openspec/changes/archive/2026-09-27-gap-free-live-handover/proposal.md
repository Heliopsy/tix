# Close the gap between a subscriber's replay and its live stream

## Why

A server reads a tenant's outbox through one pump per tenant, started by the hub the moment that tenant
gains its first connection. The pump took its starting cursor inside the goroutine it spawns, so the cursor
was read at an unbounded moment after the connection had already been registered and acknowledged.

Any event committed in that window sits above the range the connection's replay covered and at or below the
cursor the live stream opened with. Neither path delivers it. The tailer only ever reads above a cursor that
moves forward, so the event is lost, not delayed, and nothing logs a gap.

This contradicts the standing promise that `since_seq` resume is gap-free. It was near-deterministic on a
saturated CI runner and unreproducible on an idle laptop, because the pump goroutine is spawned into the
`runnext` slot and immediately displaced by the connection's own write loop: a few hundred microseconds of
starvation at the wrong instant is enough.

## What Changes

- **The pump's live cursor is taken before its tenant is reported as started**, and handed to the tailer as
  the subscription's `since_seq`. The hub therefore never acknowledges a connection whose live coverage
  floor does not already exist.
- **The window is guarded by a test that asserts delivery**, not call placement: it commits an event once
  the connection is registered and requires the subscriber to receive it.

## Impact

- Taking the cursor is a `SELECT MAX(seq)` on the WebSocket upgrade path, run while the hub holds the
  process-wide lock that orders its tenant hooks. Every connect and disconnect, in every tenant, serialises
  behind that query for its duration. This is accepted for now; the design records why and what would remove
  it.
- No client-visible protocol change. A subscriber that was losing an event in the handover now receives it.
