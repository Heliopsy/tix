# Testing

What a green run proves, what it does not, and how to tell the two apart.

## Run the suite

| Command | What it does |
| --- | --- |
| `just test` | The whole suite with the race detector, then a summary of what the environment did not allow |
| `just test-short` | The same, skipping container-backed and long-running tests |
| `just test-postgres` | Starts PostgreSQL and runs the suite against both engines |
| `just cover` | Coverage profile plus `coverage.html` |
| `just cover-check` | The coverage floor, the headroom left above it, and the packages nearest the bottom |
| `just jstest` | The web assets' JavaScript suite, on node's own runner |
| `just check` | The pre-push gate set |
| `just ci` | Every CI gate, locally, in containers |

Tests never touch a real store. Pass `--db` or `TIX_DATABASE_DSN` at a temporary path; see
[AGENTS.md](../AGENTS.md).

## Partial runs are announced

Some tests need something the machine may not have: a PostgreSQL database, a filesystem that permits
symlinks, a character device. Each is optional, so its absence skips tests rather than failing them.

A silent skip reads exactly like a pass, and the difference is not small. Without
`TIX_TEST_POSTGRES_DSN` the `internal/store/postgres` package covers **6.9%** of its statements;
with it, **85.7%**. Same command, same exit code, and one of the project's two storage engines went
entirely untested.

So the skips report themselves. Every gate is recorded through `internal/testenv`, and a run that
could not use something prints one block naming it and the command that would fix it:

```text
======================================================================
  PARTIAL RUN: this green does not cover everything
======================================================================
  postgres: TIX_TEST_POSTGRES_DSN is not set, so only SQLite was exercised
      enable with: just pg-up && just test-postgres
======================================================================
```

A run with nothing missing says so instead:

```text
test environment: complete. Every optional capability was available.
```

One block for the whole run, not one notice per package: seven scattered skip lines are seven lines
nobody reads.

**Bare `go test ./...` cannot show this.** Since Go 1.24 the tool prints nothing at all from a
package whose tests pass, so the report is invisible there unless you pass `-v`. That is why the
`just` recipes collect it instead: each test binary appends a marked line to the file named by
`TIX_TEST_ENV_LOG` (absolute path; a test binary runs in its own package directory) and the recipe
renders one summary at the end. Use `just test` rather than `go test ./...` when you want to know
what you proved.

A missing capability never fails a run. Working without a database stays possible; the goal is
honesty about what was proven, not a forced dependency.

### Adding a gate

Skip through `testenv`, never through `t.Skip` directly, so the new gate lands in the same summary:

```go
testenv.Skip(t, testenv.Capability{
    Name: "docker",
    Why:  "no container engine on PATH",
    How:  "install podman or docker",
})
```

`testenv.PostgresDSN(t)` is the single gate for database-backed tests. A package that can skip needs
`func TestMain(m *testing.M) { testenv.Main(m) }`, or a call to `testenv.AppendLog()` and
`testenv.Report(os.Stderr)` if it already has a `TestMain` of its own.

## Coverage

Two floors, because a run without PostgreSQL proves less:

| Run | Floor | Measured today |
| --- | --- | --- |
| Both engines, which is what CI runs | 85% | 87.8% |
| SQLite alone | 78% | 80.6% |

Each leaves roughly three points of headroom. A floor set flush against the current number turns red
on an unrelated change, and a gate that cries wolf is a gate people learn to ignore.

`just cover-check` prints the floor that applied, the headroom above it, and the five packages
nearest the bottom, so a slow decline is visible while it is still small. Under 1.5 points of
headroom it warns rather than failing, which is the moment to raise coverage rather than the floor.

No package is excluded from the total. `internal/tui` used to be, as untestable `View()` rendering;
it is now covered like every other package, and excluding it only hid a covered package from the
number. Its current figure comes from `go tool cover`, like any other package, so no count is
repeated here to go stale.

## PostgreSQL

```sh
just pg-up            # start it on 127.0.0.1:55432
just test-postgres    # both engines
just pg-down          # stop it
```

Row-level security has a gate of its own: the policies only prove anything when the database granted
the unprivileged `tix_app` role, so a database whose user cannot `CREATE ROLE` skips those tests and
says so. `just pg-up` provides one that can.

[`internal/store/postgres/README.md`](../internal/store/postgres/README.md) covers the engine's own
test layout, and `just leak` runs the cross-tenant isolation suite on both engines with `-v`.
