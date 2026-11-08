# Design

## One rule set for both transports

The WebSocket kept its entitlement check (`subjectScope`, project narrowing) and its event type vocabulary
in `internal/httpapi`. A second transport copying them would let the two drift, so both move to
`internal/service` (`MayReceiveEvent`, `ValidateEventTypes`, `EventSubjectScopes`) and the WebSocket calls
them. The expired-cursor error is built by one function used by the WebSocket, the direct subscription and
the poll, so a client can handle all three with one branch.

## Cursor

`since_seq` is exclusive, as on the WebSocket, and absent or zero starts at the newest event, also as on
the WebSocket: a client gets a starting cursor from its first poll without being handed the whole retained
log.

`next_seq` is the highest sequence number the request examined, not only the highest it delivered. When
the filter or the credential excludes a long run of events, a cursor that only moved on delivery would
make every later poll read that run again, and a quiet filter over a busy tenant would read the whole
retained log every 25 seconds. With a full page the two are the same number, because reading stops at the
last event delivered. A poll reads at most 5000 events; one that stops there answers at once rather than
waiting, since more is already committed.

## Waiting

The poll registers for the store's commit signal (`store.CommitWaker`, which both engines implement and
which the WebSocket's per-tenant readers already use) before its first read, so a commit between the read
and the wait still wakes it. On a wake it reads from where it stopped; on the deadline it answers empty.
The deadline is taken from the service clock, so tests drive it with `clock.FakeClock`. There is no
ticker: a lost wake costs at most the remainder of one wait and never an event, because the client resumes
from `next_seq`. The 25 second cap keeps a wait under the default 30 second request timeout.

## Surfaces

- HTTP: the route. `wait` takes a duration such as `25s`, or a number of seconds.
- CLI: `tix events`, one poll. JSON and YAML print the page; table and ndjson print events and report
  `next_seq` on standard error.
- Web: exempt. The browser already follows events over the WebSocket.
- TUI: exempt. The terminal views subscribe directly and never need a fallback transport.
- MCP: offered, with `wait` hidden. A model catching up on what happened since a cursor is a real use, but
  holding a tool call open for 25 seconds is not.
