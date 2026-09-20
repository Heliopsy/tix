# AGENTS.md

Coding standards for tix. Terse by design. Read before writing code.

## Architecture invariants

- `cmd/` is Cobra flags and wiring only. No business logic, no database access.
- No package under `internal/` may import Cobra.
- `internal/core` imports only the standard library. It is the frozen contract.
- Business rules live only in `internal/service`.
- The authorization policy lives in `internal/authz`, and `internal/service` is its only caller.
- `internal/client` performs no validation and no rules. It marshals and nothing else.
- Every mutation writes domain rows, its audit entry, and its outbox event in ONE transaction.
- Every query is built by the tenant-scoped builder in `internal/store/sql`. Never construct a query without a tenant scope.
- Dialect differences live only in `internal/store/sqlite` and `internal/store/postgres`.
- All list queries are keyset-paginated. Never use `OFFSET`.
- Every new `Service` method needs a `capability.Registry` entry with CLI, HTTP and Web bindings.
- Builds stay `CGO_ENABLED=0`. Never add a cgo dependency.

## Go style

- Follow Effective Go. `gofmt -s`. No exceptions.
- Error strings lowercase, unpunctuated. Wrap with `%w` and context: `fmt.Errorf("reading config %q: %w", path, err)`.
- Exported identifiers have doc comments. Every package has a package comment.
- Write to the command's configured writer, not `os.Stdout` directly.
- Prefer the standard library. New dependencies need a reason in the PR description.
- Keep functions short enough to test without a mock.

## Comments

- One line of doc comment on exported items. No multi-line essays.
- No commentary inside function bodies. If code needs explaining, rename or restructure it.
- Comment only what the code cannot say: a non-obvious invariant, a deliberate trade-off.

## Tests

- Tests live next to the code they cover, `*_test.go`.
- Table-driven by default.
- Use a real temporary SQLite database, not mocks.
- Anything time-dependent uses `clock.FakeClock`. Never `time.Sleep` to coordinate.
- Coverage threshold is 80%, excluding `internal/tui`.
- New behaviour ships with its test in the same change.

## Specs

- OpenSpec is normative. Behaviour changes update `openspec/` before the code.
- Requirements use SHALL. Every requirement has at least one scenario.
- Tick tasks in `tasks.md` as soon as the code ships.

## Commits

- Conventional Commits: `feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`.
- Never add `Co-Authored-By` trailers.
- Keep messages concise and human. No template filler.

## Before you push

    just check

Run `just ci` to reproduce the full CI suite locally in containers.
