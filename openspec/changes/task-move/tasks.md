# Tasks

## 1. Service

- [x] 1.1 `core.MoveTaskInput`, `Service.MoveTask`
- [x] 1.2 `taskMover` shared by `DeleteProject` and `MoveTask`, in `internal/service/move.go`
- [x] 1.3 Service tests: status mapping, tags, subtree, parent detach, permission, version, refusals

## 2. Transports

- [x] 2.1 `wire` route, `httpapi` handler, `client` method
- [x] 2.2 Registry entry, `just openapi`
- [x] 2.3 HTTP and integration tests on both engines

## 3. CLI and web

- [x] 3.1 `tix task move` with `--to`, `--version`, `--dry-run`
- [x] 3.2 Move to… form on the task screen, Bulgarian strings
- [x] 3.3 CLI and web tests

## 4. Docs

- [x] 4.1 `docs/api.md`, `docs/features.md`, `docs/web-ui.md`, the skill
