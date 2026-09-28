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

## Mutation testing

[AGENTS.md](../AGENTS.md) asks you to break the behaviour a guard protects and watch the guard fail.
`just mutate` does that mechanically: [gremlins](https://github.com/go-gremlins/gremlins) changes one
operator at a time, reruns the package's tests, and reports every change the tests did not notice.
Fourteen guards in this repository have passed over broken behaviour, and none of them was found by
review or by coverage. This is the tool shaped like the thing that did find them.

| Command | What it does |
| --- | --- |
| `just mutate internal/sshd` | Every mutant in one package |
| `just mutate internal/web '<regexp>'` | The same, with the files matching the regexp left out |
| `just mutate-sweep` | Every package in `mutate_packages`, each reported on its own |

The first run installs the pinned gremlins into `bin/`, which takes a few seconds.

The second argument to `just mutate` is a Go regexp matched against each file's name *inside* the
package directory, and it names what to leave out. RE2 has no negative lookahead, so narrowing a
package down to one file means listing the rest:

```sh
# every mutant in internal/output/stream.go and nothing else
just mutate internal/output '^(?:color|event|json|output|table|time|timestyle|yaml)\.go$'
```

For `internal/web` and `internal/service` this is the only practical way in.

There is no recipe for "only the lines I changed", although gremlins has a `--diff` flag for exactly
that, because the flag does not work when it is pointed at a package: it keys the changed lines by
their path from the repository root and keys mutants by their path inside the directory it was given,
so the two never line up. 119 changed lines under `internal/tui` produced 1227 mutants out of scope
and none tested. From the module root the paths do agree, but then the coverage step is a run of the
entire suite and the run never finished inside half an hour on a busy machine, so it is not offered
here. Name the files instead.

### Reading the output

```text
internal/authz: 5 killed, 1 survived, 2 uncovered, 0 out of scope, 0 timed out
  survived  internal/authz/policy.go:57:19  CONDITIONALS_NEGATION
```

*Killed* means a test failed when the code changed, which is the outcome you want. *Uncovered* means
no test reaches that line at all, which coverage already tells you. *Survived* is the interesting
one: the code changed and every test still passed.

A survivor is a question, not a defect, and there are three answers. Work out which one you have
before writing anything.

1. **A real gap.** The behaviour differs and nothing asserts it. Write the guard, and watch it fail
   the way AGENTS.md describes. Most of these fit the shape the fourteen shared: the assertion reads
   something wider than the thing it names.
2. **An equivalent mutant.** The change cannot be observed from outside. `if p.Limit > MaxPageLimit
   { p.Limit = MaxPageLimit }` behaves identically as `>=`, and a capacity hint in `make(map[k]v,
   len(a)+len(b))` behaves identically as `-`. There is nothing to assert. Leave it.
3. **Killed somewhere else.** gremlins runs only the mutated package's own tests, so a guard living
   in another package does not count as a kill. Reversing `core.DefaultRetention`'s event window
   survives `internal/core`'s tests and is killed immediately by `internal/config` and
   `internal/retention`. Check the wider suite before believing a survivor:

   ```sh
   # edit the line by hand, then
   go test ./internal/core ./internal/config ./internal/retention -count=1
   ```

   Of 50 survivors checked against the wider suite this way, 7 were already killed elsewhere. Add
   the equivalent mutants and something over a third of a survivor list is not a finding, which is
   why this is a list to read rather than a number to defend.

### What it costs, and where it runs

A mutant is one run of the package's tests, so the cost is the mutant count times that run, divided
by the six workers. Measured on a twenty-core machine at `269bd9a`; the last column is that
arithmetic rather than a stopwatch for the two that were never run to the end.

**These figures understate it, and the reason is worth knowing.** They were taken while the coverage
run was still being served from Go's build cache, which gave every mutant a budget of a couple of
dozen seconds. Mutants were being cut short rather than finishing, so the sweep looked fast because
it was doing less. With the cache defeated the budgets are honest and the runs are longer:
`internal/sshd` took 45 minutes on a scheduled run, against the 7 below.

| Package | Runnable mutants | Its tests | Whole package |
| --- | --- | --- | --- |
| `internal/authz` | 6 | 0.01s | seconds |
| `internal/capability` | 14 | 1.6s | ~1 minute |
| `internal/output` | 153 | 0.04s | ~30 seconds |
| `internal/core` | 244 | 0.04s | ~1 minute |
| `internal/sshd` | 186 | 2.2s | ~7 minutes |
| `internal/tui` | 1075 | 0.9s | ~20 minutes |
| `internal/web` | 675 | 25s | ~45 minutes |
| `internal/service` | 1336 | 185s | ~11 hours |

So it is not in `just check` and not in `just ci`. A gate that doubles CI time gets turned off, and
this one would do considerably worse than double. It runs two ways instead: `just mutate` aimed at
the package or the files you just changed, and a weekly
[scheduled workflow](../.github/workflows/mutation.yaml) over `mutate_packages` that posts the
survivor list to its job summary. `internal/web`, `internal/service` and `internal/tui` are
deliberately out of the sweep; reach them a file at a time.

**`internal/tui` is excluded for runner time, not because it is clean.** It had 207 survivors in the
first sweep, more than any other package, so it is the first one to reach for by hand. It is out
because a hosted runner is far smaller than a developer machine and the difference is the whole
story: `internal/sshd` takes 6m19s locally and 45 minutes on a runner, for a byte-identical result
(180 killed, 10 survived, 39 uncovered, 1 timed out, both times). `internal/tui` has five times the
mutants, which does not fit one scheduled job, and a job that cannot pass should not be scheduled: a
permanently red weekly run teaches everyone that red means nothing.

Reach it with `just mutate internal/tui` locally, or dispatch the workflow with `internal/tui` as its
package input when you want a runner to do it.

The scheduled workflow runs one job per package rather than one job for the sweep. A single slow
package used to starve the rest: `internal/sshd` spent 45 minutes and the job was killed part way
through `internal/tui`, so four packages were never reached and the week's report was one package
and a signal. Split, each package has its own budget and its own summary.

### Two ways the tool lies, and the guards against them

Both are configured in [`.gremlins.yaml`](../.gremlins.yaml), which explains them at the point of
the setting. Neither is hypothetical; both happened while this was being set up.

- **Every mutant times out and the run reports perfect efficacy.** The per-mutant timeout is derived
  from how long the coverage run took, so on a package whose tests finish in milliseconds it is
  shorter than the compile. `internal/core` reported 240 timeouts, 4 kills, `Test efficacy: 100.00%`
  and exit status 0. `just mutate` refuses any run with a timeout in it.
- **The run dies half a second in.** gremlins copies the whole module once per worker and never
  closes the files it copies, so at its default of one worker per CPU it exceeds the file descriptor
  limit and panics with `error, this is temporary`. Workers are pinned rather than left to the
  machine.

One thing neither guard covers: those copies live in `/tmp/gremlins-*`, about a gigabyte each, and
gremlins only removes them when it exits normally. Interrupt a run and they stay. `rm -rf
/tmp/gremlins-*` after a Ctrl-C.

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
