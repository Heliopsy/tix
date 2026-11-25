# Star projects

## Why

The owner wants to star projects as personal favourites. The projects screen already had Pin to
sidebar, but only as a link that showed on hover, and only in the browser. Pinning was already the
same mark as a project lane's star, so the screen called one thing by two names.

## What Changes

- Pin becomes Star everywhere in the interface, English and Bulgarian. The mark is unchanged: the
  star on the reader's own `lanes` group preference for the project. Existing pins are stars.
- Each row of the projects screen, and the project page's header, carries an always-visible star
  toggle (star outline or filled star from the sprite) with `aria-pressed` and a tooltip. It is a
  plain form, swapped in place with htmx.
- The projects screen draws a Starred section first and the rest below it. `?starred=1` shows only
  the starred ones.
- The sidebar keeps listing starred projects under Projects, the first twelve and then a Show all
  link to the starred filter.
- New service operations `ListStarredProjects`, `StarProject` and `UnstarProject` act on the
  caller's own stars only. A project read for a known reader carries `starred`.
- CLI `tix project star KEY...`, `tix project unstar KEY...`, `tix project ls --starred`; `ls`
  marks a starred project's key. The terminal's projects view stars with `*` and filters with `/`.
- HTTP `GET /api/v1/me/starred-projects`, `PUT` and `DELETE /api/v1/projects/{ref}/star`; MCP
  derives its tools from those.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: the star toggle, the Starred section and filter, the sidebar's Show all, the renaming.
- `cli`: `tix project star`, `unstar` and `ls --starred`.
- `http-api`: the starred projects routes and the `starred` field on a project.
- `tui`: starring and the starred filter in the projects view.

## Impact

- `internal/core` (`Project.Starred`, three `ProjectService` methods), `internal/service`
  (`projectstar.go`), `internal/wire`, `internal/httpapi`, `internal/client`, `internal/capability`,
  `api/openapi.yaml`, `cmd/project.go`, `internal/output`, `internal/tui`, `internal/web`,
  `internal/i18n`, docs and the skill.
- No migration: stars stay in the group preferences table, per actor, inside the tenant.
