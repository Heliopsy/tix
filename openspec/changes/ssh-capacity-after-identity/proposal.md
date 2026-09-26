# Stop an unenrolled key holding a session slot

## Why

The SSH listener's concurrency gate took a session slot in `handle`, before the presented key had been
resolved to an actor. Public-key authentication succeeds for every key on every listener, gated only by the
per-source rate limiter, so on a listener serving enrolled keys a stranger completed the handshake, opened a
session channel, took a slot, and was only then refused at the application layer.

The slot was given back when that refusal closed the session, but the window is the whole identity lookup,
and a client controls how many session channels it opens on a connection it has already authenticated. A
handful of keys, from a handful of addresses, therefore held `MaxSessions` continuously and an enrolled key
was refused for capacity that no enrolled session was using. That is unauthenticated denial of service on
the one surface a stranger reaches over the network.

The ordering was deliberate: deciding capacity before any lookup is what makes a full listener answer every
key with the same message, so nobody can learn from a refusal whether a key is enrolled. That property is
worth keeping. It does not require that strangers be able to occupy the capacity.

## What Changes

- **The gate admits a session in two steps.** A reservation answers the capacity question at the point the
  old code decided it, before any lookup, and counts against the per-key cap only. The listener-wide slot is
  taken after the identity behind the key is known.
- **The identity is resolved in `handle`**, between the two steps, rather than inside the program handler.
  The resolved actor is carried to the program handler on the session context.
- **A refusal for capacity still precedes any lookup**, so a full listener refuses a stranger and a member
  at the same point, with the same message, after the same work.

## Impact

- No change to what any client is told. The message an unenrolled key gets, the exit status, and the absence
  of a registered connection are all as before.
- The demo listener is unaffected in substance: every key there resolves to a sandbox, so nothing is refused
  between the reservation and the slot. A sandbox may now be provisioned for a key that then loses a race
  for the last slot, where before the listener was full and no sandbox was created.
- Concurrent pre-lookup resolutions for one fingerprint are now bounded by the per-key cap, which they were
  not before.
