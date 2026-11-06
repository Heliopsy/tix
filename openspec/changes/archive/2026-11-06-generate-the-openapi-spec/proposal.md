# The API describes itself in an OpenAPI document

## Why

The HTTP API had no machine-readable description. A client author read `docs/api.md`, whose routes table was
transcribed by hand, and then the handlers, because the table named paths and nothing about bodies, query
parameters, statuses or scopes. Generating clients, an MCP server or request validation from that was not
possible, and a description written by hand would drift from the code the way the table already had.

## What Changes

- `api/openapi.yaml`, an OpenAPI 3.1 document generated from the capability registry by `just openapi`.
  Schemas are reflected from the Go types the handlers decode and write, descriptions come from their doc
  comments, enums from the constants of closed core types, and error responses from the same `core.Kind` to
  status table the server answers with.
- `GET /api/v1/openapi.json` serves that document, embedded at build time, with no credential.
- The registry gains a `Contract` on each operation for the routes whose body, response, query parameters or
  media type are not the service method's own signature, and an `Endpoints` list for routes that are not the
  one binding of a service method: the probes, the document itself, a task's tags and audit trail, and
  `PUT /workflows/{key}`.
- The routes table of `docs/api.md` is generated from the document.
- Guards: the committed document must equal a fresh generation; the document must hold exactly the routes the
  router registers; every request and response of the HTTP API test suite is validated against it; every route
  the http-api spec names must be in it.

## Impact

- `internal/capability` (contracts, endpoints), `internal/openapi` (new), `api` (new, the embedded document),
  `internal/httpapi` (the route, query parameter types, a mux that records its patterns), `internal/wire`.
- New dependencies: `github.com/swaggest/openapi-go` and `github.com/swaggest/jsonschema-go` (MIT) for the
  generator, `github.com/santhosh-tekuri/jsonschema/v6` (Apache-2.0) in tests only. All pure Go.
- `justfile` (`openapi`, docs-check), `docs/api.md`, `AGENTS.md`.
