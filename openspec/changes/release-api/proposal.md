# Release check over the API

## Why

tix-mobile is drawing the screen the web serves at `/release`: whether a newer release exists, its
notes, the commands that upgrade each install method, and the bundled changelog. The server knows all
of it, but only the browser interface can read it. `GET /status` carries no update information and
needs `tenant:admin`, so a client has nothing to draw from.

## What Changes

- `GET /api/v1/release` answers any signed-in caller with `running`, `latest`, `status`, `newer`,
  `checked`, `failed`, `notes`, `methods` and `changes`, the same figures the release screen draws.
  Notes and changelog entries are Markdown with their links taken out, for the client to render.
- The release check moves out of `internal/web` into `internal/release`. One check per process serves
  the browser and the API, so the two never disagree and GitHub is asked at most once per interval.
- The method identifiers are `binary`, `container` and `go`; the web keeps its `upgrade-` element ids.

## Capabilities

### Modified Capabilities

- `http-api`: a read-only route reports the release check and the bundled changelog.

## Impact

`internal/release` (new), `internal/web/release.go`, `internal/httpapi`, `internal/capability/endpoints.go`,
`internal/wire`, `internal/server`, `internal/serving`, `api/openapi.yaml`, docs/api.md, docs/web-ui.md.
No new dependency, no migration, no user-visible string.
