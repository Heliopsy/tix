# Every API field reaches the command line

## Why

The registry proves every operation has a command, but not that the command can say everything the
API can. An audit of the inputs the API decodes against the flags of the commands bound to the same
operations found four holes:

- a custom field's default could be set through the API and a bundle, and not with `tix field put`
  or the browser's field form. Both of those replace the whole definition, so saving a field there
  silently cleared a default set elsewhere. The browser form also read `indexed` and `position` from
  controls it never drew.
- a transition's expected `version` was honoured by the service and sent by nothing but the API.
- `tix project ls`, `tix user ls`, `tix actor ls` and `tix webhook deliveries` printed "more results
  available; next cursor ..." and took no `--cursor` to follow it.
- `tix project ls` could not narrow by key, which `GET /projects?key=` can.

## What Changes

- `tix field put --default VALUE`. A field holding text keeps the value as text; any other type
  reads it as JSON first, as `--field` does.
- The browser's field form gains Default, Position and Indexed controls, the default converted to the
  field's type the way the task screen converts a value.
- `tix task mv --version N`, with and without `--hops`, for a single task. Naming a version with
  several tasks is a usage error rather than a version applied to all of them.
- `--cursor` on `tix project ls`, `tix user ls`, `tix actor ls` and `tix webhook deliveries`, and a
  repeatable `--key` on `tix project ls`.
- A guard, `internal/capability/inputparity_test.go`, reflects over the inputs the API decodes and
  fails when a field has no flag, argument or written reason on the command bound to the same
  operation.

## Impact

- `cmd/project.go`, `cmd/parse.go`, `cmd/task.go`, `cmd/auth.go`, `cmd/actor.go`, `cmd/admin.go`
- `internal/web/projects.go`, `internal/web/templates/fields.html`, `internal/i18n/bg_browse.go`
- No service, store or wire change.
