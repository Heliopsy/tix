# Design

## A Go program, not k6

k6 would add a pinned tool to the toolbox and a JavaScript script, and it still could not start and seed a
scratch server or read `/proc` for the server's memory, so a wrapper would be needed anyway. A Go program
does all of it, reuses `github.com/coder/websocket`, and its budget and guard logic is unit-tested with the
rest of the suite. It lives under `internal/loadtest` with its `main` in `internal/loadtest/tixload`, so
goreleaser, which builds the module root, never ships it, and it uses `flag` rather than Cobra because no
package under `internal/` may import Cobra.

## Black box over HTTP

The users speak only HTTP and WebSocket, the way real clients do, so the same code runs against the scratch
server and against a deployed one. Seeding goes through the API too, which exercises the write path the
agents use. Tenants, accounts and tokens are made with the CLI before any server starts, because creating a
tenant needs a global admin and a shell holding the database is one.

## Measuring a fresh process

Seeding a thousand large descriptions allocates far more than serving them, and that memory would stay in
VmHWM for the life of the process. The run therefore stops the seeding server and measures a second one on
the same database. Peak is the larger of every sampled VmRSS and the final VmHWM, because the kernel's
high-water mark catches a spike that fell between two one-second samples, and the OOM killer acts on that
number. Steady is the median after a warm-up share of the run. The live heap comes from `GODEBUG=gctrace=1`
on the server's stderr, which needs no hook in the server.

## The budget

128 MiB for four users is half the limit production failed at, and what the owner considers reasonable for
the smallest team tix serves. Larger profiles carry no default, since no measurement yet says what they
should cost. Remote runs cannot read memory, so the budget is reported as not checked rather than passed.

## The write guard

Every request goes through one client, which refuses a mutating method unless the run allows writes; the
refusal happens before the request is built, so nothing reaches the server. Signing in is the one exception,
because a browser cannot be simulated without a session. With writes allowed against a remote server,
agents create tasks only in the scratch project and change only tasks they created. A scratch project is
either named, and then used or created, or generated with a random key that must not exist yet, so an
existing project is never written to unless someone typed its key.

## CI

A nightly and `workflow_dispatch` workflow, not a pull request check: the run takes minutes and only a
change to the server's memory moves it. It is `continue-on-error` while the known spike is unfixed, so it
records the number nightly without turning the run red, and the line is removed when the fix lands.
