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
- Coverage floor is 85% with both engines, 78% when the PostgreSQL suite skips. No package is excluded.
- A test that skips on a missing capability skips through `internal/testenv`, so the run reports it.
- New behaviour ships with its test in the same change.

### Watch the guard fail

A test you have not watched fail is not evidence. Before a guard counts as
written: copy the file, break the behaviour it protects, run it, read the
failure, restore, and confirm the restore is byte-exact with `sha256sum`.

This is not ceremony. Fourteen guards in this repo have passed while the code
they protected was broken, and every one was found this way rather than by
review, by coverage, or by the guard itself.

They nearly all failed the same way: **the assertion read something wider than
the thing it names.** A frame instead of the footer, a page instead of the nav,
a stylesheet instead of one rule, a document instead of one table. The token was
somewhere else on the page, so the test passed while the feature was gone.
Helpers exist for this and are worth copying rather than reinventing:
`formAt`, `formBlock`, `formRow`, `confirmLine`, `nav`, `shapeRow`,
`inputValue`, `declarations`, `setupSection` and `internal/docsmd`.

Four traps, all real:

- **A mutation that does not mutate looks like a working guard.** Check the
  edit landed before trusting the run. One agent's first three mutations passed
  because its pattern missed a hard-wrapped line.
- **A second check can mask the one you are testing.** Removing a keystroke
  gate changed nothing observable, because a later refusal also stopped the
  view opening, while the read it should have prevented still went out. Assert
  both halves: the affordance is absent *and* the call was never made.
- **Restore against git, not against your backup.** `sha256sum` proves you
  restored the bytes you copied, not the bytes that belong there. A backup
  taken before a commit landed restored cleanly over the commit and silently
  reverted the feature, and the check passed. Finish with `git diff -- <file>`
  empty.
- **A shared scratch directory is a shared file.** Parallel agents pointed at
  one temp path overwrote each other's harness mid-run, which recorded a kill
  with no mutation applied. Scratch goes somewhere named for the one agent
  using it.

Each of these produces something indistinguishable from a passing guard, which
is why they are written down rather than remembered.

`just mutate internal/tui` does this mechanically to a whole package, one
operator at a time, and names every change the tests sat through. It is not a
substitute for doing it by hand on the behaviour you actually care about: it
breaks operators, not intent, and a good share of what it reports is either
unobservable or already caught by another package's tests. Read
[docs/testing.md](docs/testing.md#mutation-testing) before acting on a survivor.

### Documentation a test can decide

Prose explaining why a thing is the way it is does not rot. Tables that
transcribe an enumerable list out of the code do, and did, twice in two days.
A list the code already holds is asserted rather than maintained: see
`internal/tui/docs_test.go`, `internal/capability/docs_test.go` and
`cmd/docs_readme_test.go`. Judgement stays prose and stays reviewed.

## Never touch a real store

- Tests, hand verification and demos use an isolated database, always. Pass
  `--db` or `TIX_DATABASE_DSN` pointing at a temporary path, and prefer a fresh
  tenant over reusing `default`.
- Never run against the zero-config store at `$XDG_DATA_HOME/tix/tix.db`. That
  is somebody's real work. A command with no `--db` resolves there, so the
  absence of a flag is the bug.
- `tix serve` for a demo takes the same `--db`. A server started without one
  puts a browser on the real store.
- This is not hypothetical: hand verification once created projects in a real
  store, which then broke the zero-config `task add` path for its owner because
  the tenant no longer had exactly one project.

## Specs

- OpenSpec is normative. Behaviour changes update `openspec/` before the code.
- Requirements use SHALL. Every requirement has at least one scenario.
- Tick tasks in `tasks.md` as soon as the code ships.

## Commits

- Conventional Commits: `feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`.
- Never add `Co-Authored-By` trailers.
- Keep messages concise and human. No template filler.
- Merge an agent branch with `--ff-only`, so its commits land on `main` where
  release-please can read them. release-please walks merge commits on `main`
  and considers only the merge commit's own subject; the conventional commits
  inside a `--no-ff` merge are invisible to it. `fix(cmd): install the
  interrupt handler before the listener binds` shipped that way and reached no
  changelog.
- When a merge commit is unavoidable, give it a conventional subject naming
  what it delivers, never `merge: ...`.

## Release pull requests

- Approve only the runs whose head is the current one. Approving a run that is
  `action_required` replays it with the workflow file it was created against,
  so a stale run resurrects a workflow bug already fixed on `main` and reports
  it as a failure on the release.
- release-please does not rebase its branch when the release content has not
  changed, so a pull request cut before a workflow fix keeps failing on it.
  Close it and let the next push to `main` regenerate one on the fixed base.

## Parallel agents

Rules for a batch of agents working at once, each learned the slow way.

- **One worktree and one scratch directory per agent.** Worktrees go in
  `.worktrees/<task>`, branched from `origin/main`. Scratch, databases and
  screenshots go in a directory named for that agent. A Postgres container
  gets a port checked free first: one run once reached an unrelated database
  that happened to hold the port it picked.
- **Split by file set, not by feature count.** `layout.html`, `tasks.html`,
  `app.css`, `internal/web/i18n.go`, the README feature line,
  `capability/registry.go` and the operation count in `docs/tui.md` are shared
  by nearly every web feature. Ten agents all touching them meant every merge
  forced nine rebases and nine fresh CI runs. Group features that share these
  files under one agent, or run them one after another.
- **Migrations merge in version order.** The runner applies only versions above
  the highest one applied, so a lower number merged later is silently skipped
  on every database already past it. `internal/store/migrations/gaps_test.go`
  fails on an unlisted gap. Reserve numbers up front, merge strictly ascending,
  and keep a PR whose lower numbers are not on `main` yet as a draft instead of
  letting it sit red.
- **Verify lightly per change, fully once per batch.** Locally: `just lint`,
  `go test` for the packages you touched, the node suite if you changed
  JavaScript, `just spec`, `just docs-check`, and screenshots of the pages you
  changed. Watch each new guard fail by running only that test
  (`go test -run TestName ./pkg/`), not the package. The full suite, PostgreSQL
  and the container gates run in CI, and `just check` once per batch. Ten
  agents each running the full suite on both engines pushed the load average
  past 60, got runs killed, and doubled every agent's wall time.
- **Green means every check passes.** `gh pr checks N --json bucket --jq
  '[.[].bucket]|all(.=="pass")'` prints `true`. Reading the table by eye merged
  a PR with a failed check once. Rerun a failed job only for a known flake
  (#102, the SSH timing tests), and name it in the PR.
- **Rebase once, when it is your turn.** A PR waiting behind another does
  nothing until that one merges, then rebases once and pushes. Rebasing on
  every merge while waiting only spends CI.
- **Every feature is whole on every surface.** Service method and registry
  entry, HTTP route in the `docs/api.md` routes table, CLI flags and generated
  docs, TUI binding or a written exemption, web, OpenSpec change, docs, the
  skill under `skills/tix`, Bulgarian strings. The parity guards in
  `internal/capability` fail on most of the gaps; the rest is review.
- **Leave a Website note in the PR body.** List the new commands, flags, exit
  codes and screens, so the tix.red docs can be updated at release. Write route
  parameters as `REF`, `KEY` or `ID`, never a bare lowercase word in braces:
  the site's smoke test reads that as an unfilled token and blocks the deploy.
- **Report early when it runs long.** An agent past an hour reports what is
  done and what is left rather than staying silent.

## Before you push

    just check

Run `just ci` to reproduce the full CI suite locally in containers.
