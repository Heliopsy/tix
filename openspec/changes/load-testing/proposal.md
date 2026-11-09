# Load testing with a memory budget

## Why

Production was OOM-killed at a 256 MiB limit with about four people using it. Nothing in the repository
could reproduce that: `internal/bench` measures store queries in-process, and no test drives a real
server with browsers, phones and agents at once or looks at the server's memory. A fix for the spike needs
a way to show it worked, and a guard so it does not come back.

## What Changes

- `just load` and `just load-smoke` run `internal/loadtest/tixload`, a Go program that is not part of the
  `tix` binary. A local run bootstraps tenants and accounts with the CLI, seeds a thousand tasks with 5 to
  50 KiB descriptions, comments, tags and assignees through a first server, then measures a fresh one.
- Virtual users follow three behaviours, mixed by `--mix`: a browser (session cookie, pages, assets with
  conditional requests, a WebSocket on the event stream), the phone app (long poll with `wait=25s`) and an
  agent (list, read, create, update, transition, comment).
- The report gives request rate, p50/p95/p99 per route and status counts, and the server's start, steady
  and peak RSS from `/proc/PID/status`, with the live heap from gctrace.
- `--max-rss` fails the run when peak RSS passes it; the default is 128 MiB for up to four users.
- `--target URL` loads a running server, read-only unless `--allow-writes`, which confines writes to a named
  or newly created scratch project and to tasks the run created.
- A nightly and on-demand **Load** workflow runs four users for two minutes against the budget and uploads
  the report. It is non-blocking until the memory spike fix lands.

## Capabilities

### New Capabilities

- `load-testing`: the load tool, its memory budget and its remote-mode write guard.

## Impact

`internal/loadtest`, `justfile`, `.github/workflows/load.yaml`, `docs/testing.md`, `docs/deployment.md`.
No product behaviour changes.
