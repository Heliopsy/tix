# Serve the API as MCP tools

## Why

Agents increasingly reach tools through the Model Context Protocol rather than a shell. tix had no MCP server, so
an MCP client could only drive it by running the CLI, and the skill had to say outright that there was none.
The OpenAPI document generated in `generate-the-openapi-spec` already describes every route, its body, its
parameters and its scope, and named this server as its follow-up: tools generated from it cannot drift from the
API any more than the document can drift from the router.

## What Changes

- `tix mcp` serves the HTTP API to an MCP client over stdio. Its tools are built at startup from the OpenAPI
  document embedded in the binary: one tool per JSON operation, the input schema merged from path parameters,
  query parameters and the request body, each call one request through the existing HTTP client with the
  configured server and token, and failures returned as tool errors carrying the API's error envelope.
- `internal/mcp/overlay.yaml`, embedded, holds what the document cannot decide: operations to hide (with a
  reason), tool renames, descriptions, parameters to leave out, and which operations are destructive. `--overlay
  PATH` lays a user file over it.
- Destructive tools are offered only with `--allow-destructive` or when an overlay enables them. Tools whose scope
  the token lacks are not listed.
- `tix mcp --list` prints every operation and what became of it, without contacting a server.
- `docs/mcp.md`, links from the README, `docs/README.md` and `docs/api.md`, a line in AGENTS.md, a section in the
  skill's setup reference, and the ROADMAP entry marked shipped.

## Impact

- New: `internal/mcp`, `cmd/mcp.go`, `docs/mcp.md`. Changed: `internal/client` gains `Raw`, a request built
  from the document rather than a typed method.
- New dependency: `github.com/modelcontextprotocol/go-sdk` v1.8.0, the official Go SDK (Apache-2.0, with MIT for
  older contributions), pure Go. Its own dependencies are pure Go as well.
- No change to the service, the registry or the HTTP API: `tix mcp` is a client.
