# Tasks

## Tool

- [x] `internal/loadtest`: mix, sizes, recorder and per-route percentiles
- [x] Memory: `/proc/PID/status` sampler, gctrace heap, start, steady and peak, budget verdict
- [x] Local mode: CLI bootstrap, seeding server, fresh measured server, stop by PID
- [x] Users: browser with cookie, pages, conditional assets and WebSocket; phone long poll; agent reads and writes
- [x] Remote mode: banner, read-only client, `--allow-writes` with a scratch project
- [x] `internal/loadtest/tixload` main, `just load` and `just load-smoke`

## Tests

- [x] Report, percentile, memory summary and budget tests
- [x] Write guard: refused writes are never sent, read-only agents only read, remote agents change only their own tasks
- [x] Scratch project: named, created, and a colliding generated key refused
- [x] Smoke: one user for three seconds against an in-process server produces a report

## CI and docs

- [x] `.github/workflows/load.yaml`, nightly and on dispatch, `continue-on-error` until the memory fix lands
- [x] docs/testing.md "Load testing", docs/deployment.md "Sizing memory"
- [ ] Remove `continue-on-error` once the memory-spike fix is on main and a nightly run passes
