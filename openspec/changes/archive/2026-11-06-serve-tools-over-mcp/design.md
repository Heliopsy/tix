# Design

## A thin layer driven by the document

The ROADMAP planned an MCP server as a third adapter mapping tools onto `core.Service` methods. That would be a
fourth description of every operation, after the registry, the handlers and the CLI, written by hand and kept in
step by review. The OpenAPI document already holds the operation's name, purpose, parameters, body schema, scope
and media types, and its own guards hold it to the router. Generating tools from it, and sending each call through
the HTTP client, means a route added to the API becomes a tool with no change here, and a tool can never accept
something the server would not.

The document read is the one embedded in the binary, so the tools describe the version of tix that ships them.
A server of another version may differ; the server validates every request, so a mismatch is an error message,
never a wrong write.

Each operation maps to one tool. The name is the operationId with dots replaced by underscores, because some
clients accept only `[A-Za-z0-9_-]`. The input is one object: path and query parameters beside the body's
properties, so a model fills one flat set of fields. Where a body property shares a name with a parameter
(`PUT /workflows/{key}` takes a `key` in both), or the body is not an object, the body travels whole under `body`.
Component references are inlined, a recursive one cut to a bare object, so a client needs no resolver; every
schema stays far below the 16 KiB budget the tests enforce.

Operations that do not speak JSON (the WebSocket stream, the calendar feed, icons, snapshot and bundle streams)
are left out by rule rather than by overlay entry, since no overlay setting could make them work.

Composite tools, several calls behind one name, were considered and left out: the one candidate, claiming and then
showing a task, is what `claim.next` already answers.

## The overlay

Some decisions are judgement and the document cannot hold them: that a sign-in step is no use to a process that
already has a token, that unstarring is not destructive although it is a DELETE, that a bulk change is
destructive although it is a POST. These live in `internal/mcp/overlay.yaml`, keyed by operationId, each
departure from the default with its reason. New JSON operations are exposed by default, so adding a route stays
one change; the guards demand only that every overlay key names a real operation and every judgement carries a
reason, so the overlay cannot rot silently when an operation is renamed or removed.

A user overlay is laid over the shipped one field by field, so an operator can enable one destructive tool or
rename one without restating the rest.

## Destructive is opt-in

A model acting on a misread instruction should cost one task, not a project. Every DELETE is destructive unless
the overlay says it only clears a reversible mark, and the overlay marks the non-DELETE operations that lose data
(bulk changes, reclaiming a live lease, pruning history, running housekeeping). These are left off the tool list
unless `--allow-destructive` is given or the overlay enables them, and every tool carries the MCP read-only,
idempotent and destructive hints from its method so a client can ask before calling.

## Scope filtering

The token's scopes are read once at startup from `whoami`, and a tool whose scope it lacks is not listed. The
server would refuse the call anyway; listing it would spend the model's context and attention on tools it cannot
use, and invite it to try. Least-privilege tokens therefore also give a smaller, more focused tool list.

## Stdio only

Stdio is how local clients such as Claude Code launch servers, and it keeps the token in the operator's process.
The SDK makes a streamable HTTP transport cheap to wire, but serving one would hand the configured token's
authority to whoever reaches the port, and answering that properly (per-session tokens, OAuth) is a feature of
its own. `tix serve` remains the network surface.

`tix mcp` requires a server and never opens a database. Serving a local store would need an actor without a
token and a second path for every call, and a command started with no flag must not open the zero-config store.

## Guards

- Every operation of the document plans to exactly one tool state, and every hidden or destructive one carries a
  reason.
- The overlay cannot name an operation absent from the document, hide without a reason, override the destructive
  default without a reason, hide a parameter that does not exist or is required, or carry an unknown key.
- Tool names are unique and valid; every input schema compiles as JSON Schema, holds no `$ref`, and is within
  budget.
- End to end, a real HTTP server on a scratch SQLite store and the MCP server over an in-memory transport: create,
  list and claim through tools, a scope-less token not offered a tool, a destructive tool absent without the flag.
  The command itself is driven over stdio against `tix serve`.
- The lists `docs/mcp.md` transcribes (non-JSON operations, non-DELETE destructive tools, overlay keys) are
  asserted against the code.
