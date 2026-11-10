# Design

- **Refuse at the session, not at authentication.** SSH has no way to carry a reason with a failed public key
  attempt, and a banner would be shown to every connection. Accepting the key and refusing in `handle` reuses the
  path every other refusal (unknown key, capacity) already takes: `fatalf` writes to stderr and exits 1.
- **Nothing is reachable through the accepted handshake.** The listener has no channel or request handlers besides
  the session handler, and the throttle check is the first thing it does after reading the key, before the
  capacity gate and the key lookup. A client over the limit gets the message and nothing else, and the limiter
  still spares the database the lookup.
- **One decision per connection.** The decision is stored in the connection context the first time the callback
  runs, so a callback that runs again on the same connection does not spend a second token.
- **The wait is computed, not guessed.** `limiter.take` returns the time until the bucket holds a whole token,
  rounded up to whole seconds in the message and never under one second.
