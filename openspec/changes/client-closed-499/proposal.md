# Log an abandoned request as 499, not 500

## Why

A long poll on `GET /api/v1/events/poll` whose client disconnected was logged with status 500. The service
returns the cancelled context's error, `WriteError` maps an unknown error to `internal`, and the request line
records a server error no client ever received. Production logs showed one after 14.4s when a phone dropped
off and another at 207ms. Every handler that reads the request context does the same.

## What Changes

- The request-logging middleware records a server error written after the request's own context was
  cancelled as `499`, nginx's "client closed request", writes no body, and adds `note="client closed request"`
  to the `info` line. A handler that wrote nothing before the client left is recorded the same way.
- A cancellation the client did not cause, and the server's own request timeout, are unchanged: `500` and `504`.
- The browser interface no longer logs a cancelled page at `error` level and renders nothing for a browser
  that left.

## Capabilities

### Modified Capabilities

- `server`: a request its client abandoned is logged as 499 rather than as a server error.

## Impact

`internal/httpapi/middleware.go`, `internal/web/render.go`, `docs/deployment.md`, `docs/api.md`.
