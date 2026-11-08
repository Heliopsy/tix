# Following events over HTTP

## Why

The event stream is a WebSocket and nothing else. Clients behind proxies, captive portals and networks
that break WebSockets (tix-mobile is the first) have no way to follow events at all, and a client that
falls back to listing loses the cursor and the gap-free guarantee that makes the stream worth having.
They need a plain request that answers the same events from the same cursor, so they can fall back after a
few failed WebSocket attempts and switch back later without missing anything.

## What Changes

- `GET /api/v1/events/poll` (`event.poll`, service method `PollEvents`) answers the events after
  `since_seq` that match `types` and `project_ids`, at most `limit`, with `next_seq` to pass back.
- `wait` (up to 25s) holds an empty answer until an event commits or the wait runs out, woken by the
  same commit signal that drives the WebSocket's readers.
- The credential rules, the filter vocabulary and the expired-cursor error are the WebSocket's, read from
  one place in `internal/service` rather than copied.
- `tix events` runs one poll from the command line.
- `tix mcp` offers the operation without `wait`.

## Capabilities

### Modified Capabilities

- `event-stream`: an HTTP long-poll alongside the WebSocket, with the same cursor, filters and scopes.

## Impact

`internal/core` (service method, input and page types), `internal/service` (the poll, and the event
entitlement and type vocabulary moved out of `internal/httpapi`), `internal/httpapi` (route, query type,
the WebSocket calling the shared rules), `internal/client`, `internal/capability`, `cmd/events.go`,
`internal/mcp/overlay.yaml`, `api/openapi.yaml`, `docs/api.md`, generated CLI docs, the tix skill.
