# Every OpenAPI parameter says what it takes

## Why

The OpenAPI document described every operation and every body field, and none of its 200 query and path
parameters. The query structs carried field comments, but the generator only attached field comments to body
schemas, and path parameters had nowhere to take a description from. `tix mcp` builds its tool arguments from
these parameters, so a model choosing a value for `snoozed`, `text` or `{ref}` saw a bare name and a type.

## What Changes

- A query parameter's description is the doc comment of its field on the operation's Query struct, embedded
  page fields included. The comments now state format, defaults and allowed values.
- A path parameter's description comes from one table in `internal/openapi`, keyed by name, with an entry per
  collection where a name means different things (`{ref}` of a task, a project, a template or a tenant).
- A parameter family read by prefix, `field.KEY`, is described under `x-tix-query-prefix-descriptions` from the
  doc comment of the constant spelling the prefix.
- `TestEveryParameterIsDescribed` fails on any parameter or prefix without a description.

## Impact

- `internal/openapi`, `internal/httpapi/query.go`, `api/openapi.yaml`, and through it the MCP tool arguments.
