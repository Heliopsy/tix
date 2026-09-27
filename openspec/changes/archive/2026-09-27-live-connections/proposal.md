# See and end the connections a server is holding

## Why

A running tix server holds two kinds of live connection: browsers and API clients on the event stream, and
terminal sessions over SSH. Both are counted already. The WebSocket hub knows how many are open and which
tenant each belongs to; the SSH listener knows how many sessions each key holds. Neither number is reachable
from outside the process: `Hub.Len` and the SSH gate's counters have no callers but tests.

So an administrator cannot answer "who is connected", and cannot do anything about it. Today the only way to
end a session somebody else is holding is to restart the listener, which ends everybody else's too.

That gap has a sharp edge. Revoking an SSH key stops it authenticating but deliberately does not cut the
sessions it already holds, on the reasoning that the listener should not carry a second subscription on its
connection path. The accepted cost was that an operator wanting a hard cut restarts the process. Being able
to end one connection removes that cost without putting anything back on the connection path.

## What Changes

- **A connection registry** in the server, fed by the WebSocket hub and the SSH listener, holding what is
  live right now: an identifier, which surface it arrived on, the actor behind it, the tenant, where it came
  from and when it started.
- **Counts**, both per tenant and for the process as a whole.
- **Ending one connection**, by identifier, from the CLI, the API and the web interface.
- **The SSH listener gains the session handles it lacked**, which is also what a draining shutdown needs to
  tell a session why it is closing rather than cutting it silently.

## Naming

These are **connections**, not sessions. `core.Session` already means a login session, an authenticated
period backed by a row in `sessions`. A live WebSocket is not that, and neither is an SSH terminal. Reusing
the word would make two unrelated things share a name in the same service interface.

## Impact

- The registry is **per process**. A connection lives in exactly one process, so a server can only list and
  end the connections it is itself holding. A deployment running several servers against one database gets a
  partial view from each. This is stated in the response rather than hidden, and the extension that would fix
  it is described in the design.
- Nothing is stored. The registry is memory only, and a restart empties it, which is correct: the connections
  are gone too.
