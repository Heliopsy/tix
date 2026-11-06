# A task's due date can be cleared over HTTP

## Why

`UpdateTaskInput.DueAt` is a pointer to a pointer so that Go callers can tell "leave it" (nil) from "clear
it" (a pointer to nil). The JSON decoder cannot: it turns `"due_at": null` into a nil outer pointer, so a
`PATCH /api/v1/tasks/{ref}` had no way to remove a due date. The doc comment said as much. Every surface
that reaches the server over HTTP inherited the gap: `tix task edit --due ""` against `--server`, the
terminal in remote mode, and an MCP client sending `null`. The local command line and the browser call the
service directly and were not affected.

## What Changes

- `UpdateTaskInput` decodes `"due_at": null` as a clear. Absent leaves the due date, a timestamp sets it.
- The field's doc comment, and so the OpenAPI description and MCP tool schema, say null clears it. The
  schema already declared the field nullable, so the wire format does not change.
- docs/api.md states the absent, null and value rule for `PATCH /tasks/{ref}`.

The other optional fields on task, project, tenant and template updates were checked. Each clears with a
value JSON can carry (`""`, `0`, `[]`, or a null inside `custom_fields`), so none needed the same change.

## Impact

- `internal/core` (input decoding), `api/openapi.yaml` (description only), docs/api.md.
- No storage, service or client change: the client already sent `null` for a clear.
