# Tasks

## 1. Service

- [x] 1.1 `core.Project.Starred`, `ListStarredProjects`, `StarProject`, `UnstarProject`
- [x] 1.2 `internal/service/projectstar.go`, marking `Starred` on `GetProject` and `ListProjects`
- [x] 1.3 Service tests: per person, idempotent, back-compat with a lane star

## 2. Transports

- [x] 2.1 `wire` routes, `httpapi` handlers, `client` methods
- [x] 2.2 Registry entries, `just openapi`
- [x] 2.3 HTTP tests

## 3. CLI and TUI

- [x] 3.1 `tix project star`, `unstar`, `ls --starred`, the star marker in `ls`
- [x] 3.2 TUI projects view: `*` toggles, `/` filters to starred
- [x] 3.3 CLI and TUI tests

## 4. Web

- [x] 4.1 Star toggle on the projects rows and the project page header, htmx and no-JS
- [x] 4.2 Starred section, `?starred=1`, sidebar Show all
- [x] 4.3 Pin renamed to Star, Bulgarian strings
- [x] 4.4 Web tests

## 5. Docs

- [x] 5.1 `docs/web-ui.md`, `docs/tui.md`, CLI docs, the skill (version bump)
