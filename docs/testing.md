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
| `just test-shard SHARD` | One CI shard with coverage into `out/coverage/`; `scripts/test-shards.sh names` lists them |
| `just test-shards` | Every shard in turn, merged into `coverage.out`; then `just cover-floor` applies the floor |
| `just shards-check` | Fails unless every package is in exactly one shard and CI runs every shard |
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

A missing capability never fails a run, so working without a database stays possible and no
dependency is forced. The report says what was proven.

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
number. Its current figure comes from `go tool cover`, like any other package.

## How CI runs the suite

CI splits the Go suite by package across parallel runners: `service`, `cmd`, `web`, `api`
(`httpapi`, `integration`, `demo`), `store` (the engines, `connect`, `bench`) and `rest`, which takes
every package no named shard claims. `scripts/test-shards.sh` owns the split. Every shard runs with
`-race -shuffle=on -count=1` and coverage, against PostgreSQL as well as SQLite. The Coverage job
merges the six profiles and applies the 85% floor to the total, and fails rather than skipping when a
shard failed. The Gate job runs tidy, gofmt, vet, the build, `just shards-check` and `just smoke` in a
couple of minutes, so a slip shows before the suite finishes.

macOS runs the whole suite on main, nightly, and on a pull request that `just os-scope` says touches
something an operating system can tell apart: build constraints, syscall or terminal imports, SSH,
paths, filesystems. Other pull requests run the `rest` and `store` shards there, which hold every
package that reaches the operating system.

Two things keep the slow packages fast, and both are for tests only:

- `sqlitetest.Path(t)` hands a test a copy of a database migrated once per test binary. Migrating a
  fresh file costs seconds under the race detector, and the migration path itself is still covered
  from an empty file by `internal/store/sqlite` and `internal/store/migrations`.
- `service.UseCheapHashingInTests()` in a `TestMain` drops argon2id to `auth.TestParams` for services
  built through wiring a test cannot pass a hasher to. `internal/service` never calls it and asserts
  `New` hashes at production cost, and `internal/architecture` fails if anything but a `_test.go`
  file calls it.

## Mutation testing

[AGENTS.md](../AGENTS.md) asks you to break the behaviour a guard protects and watch the guard fail.
`just mutate` does that mechanically: [gremlins](https://github.com/go-gremlins/gremlins) changes one
operator at a time, reruns the package's tests, and reports every change the tests did not notice.
Fourteen guards in this repository have passed over broken behaviour, and none of them was found by
review or by coverage. They were found by breaking the behaviour by hand, which is what this tool
automates.

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

**These figures understate the cost.** They were taken while the coverage
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
because a hosted runner is far smaller than a developer machine: `internal/sshd` takes 6m19s
locally and 45 minutes on a runner, for a byte-identical result
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
the setting. Both happened while the tool was being set up.

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

## Web screenshots

`just screenshots` captures every screen of the browser interface, and every popup on it, at four sizes:

| Name | Viewport | Scale |
| --- | --- | --- |
| `phone` | 390 x 844, touch, mobile user agent | 3x |
| `tablet` | 820 x 1180, touch | 2x |
| `fullhd` | 1920 x 1080 | 1x |
| `4k` | 3840 x 2160 | 1x |

```sh
just screenshots                                   # everything, light theme
just screenshots --only /tasks --viewports phone   # a quick look while working on one screen
just screenshots --theme both --viewports fullhd   # light and dark side by side
```

Each run writes `out/screenshots/<time>/<viewport>/<page>[--<state>][--<popup>].png` and an `index.html`
contact sheet beside them, one table per page with the viewports side by side, and prints its path at the
end. `report.json` there lists the routes not captured and why. `out/` is ignored by git.

By default the recipe builds `bin/tix`, seeds a fresh database in a temporary directory with
`tix demo seed`, serves it on a free loopback port with `--db` and isolated XDG directories, signs in as
the seeded administrator through the login form, and adds what the seed does not write: a template, two
saved filters, a notification channel and a calendar feed. The server is stopped and the directory deleted
on every exit, a failure or Ctrl-C included.

`--url URL` captures a running instance instead, signed in as `TIX_SHOT_EMAIL` with `TIX_SHOT_PASSWORD`.
When the tenant asks for a second factor, `TIX_SHOT_TOTP` is the account's TOTP secret and the code is
derived from it; an account that still has to enrol cannot be used. Against a running instance the tool
only reads screens and opens menus, and adds no fixtures, so a screen whose parameter has nothing to point
at (no template, no channel) is reported as not captured. The session it signs in with is left to expire,
and the credentials are never printed. Use an administrator, or the admin screens render as refusals.

Which pages exist comes from `scripts/screenshots/routes.json`, which lists every GET route in
`internal/web/routes.go`, with how to fill its parameters, extra states to capture (the board and lanes
views, a selection, the task dialog) or why it is skipped (an htmx fragment, a file). A test in
`internal/web` fails when a route is missing from it or it names one that no longer exists, so a new
screen is added there in the same change.

Popups are found rather than listed: every `[popover]`, every `<details>` (the (i) tips, More options,
the confirmations inside them) and the command palette and shortcuts overlay. Each is opened through its
visible trigger, after opening whatever encloses it, so a menu inside the phone's sidebar is captured the
way a person reaches it. A popup with the same text as one already captured in that viewport, the
sidebar or a tip repeated on every page, is captured once, and a menu repeated on every row once per page.
`--max-popups N` (default 40) caps one page state. Toasts are not captured: they follow a change, and the
tool makes none.

The phone and tablet shots keep their touch emulation while they are taken: Playwright's full-page
capture drops it, so for those two the tool grows the viewport to the page's height and takes a plain
shot instead. A phone shot shows what a phone shows, without the rows hidden under
`(hover: none) and (pointer: coarse)`.

Animations and transitions are off, reduced motion is requested, every page sees the same `Date.now()` for
the whole run, and fonts and htmx settle before each shot. The viewports run one after another in a single
browser, so a full run stays light on the machine; `--jobs N` runs N viewport and theme combinations at
once instead. Playwright is pinned in
`scripts/screenshots/package-lock.json`; the recipe installs it and Chromium only when they are missing.

**The layout check.** After every shot the page is measured: each visible button, button-styled link, menu
item, segment, tab, badge, pill, chip and kbd whose label takes two lines or runs past the window's edge,
and each control, heading, label, `th` or `dt` with a word split across two lines, is printed under
`layout:` at the end of the run and listed in `report.json`. Lines are told apart by the rectangles
`Range.getClientRects` returns for each text node, which is the only place a browser says where it broke
a line. A "Save" drawn as "Sav" over "e" is what it exists to catch. `--strict` makes any finding fail the
run with a non-zero exit:

```sh
just screenshots --viewports phone,tablet,fullhd --strict
```

**Task rows and cards.** The same check measures every task row and card on the page. It reports a row
wider than its list or a card wider than its column, a paragraph, list, heading, table or code block
inside one (a rendered description leaking into a summary), a row over twice the median row height of
its list, and, from 640px up, an attribute cell (status, priority, tags and so on) whose left edge is
out of line with the same cell in the rows above. `--hostile` adds a project of tasks with long titles,
long tags and Markdown descriptions holding headings, tables, fenced code and unbroken tokens, sorted to
the top of `/tasks`, and the opt-in `laptop` viewport (1440x900) covers the width most of these broke
at. `just ui-rows` runs the listings with both at phone, tablet, desktop and laptop, light and dark:

```sh
just ui-rows
```

**Checks on every screen.** Besides controls, words and task rows, `--strict` fails on four things that
hold on any page:

- the page scrolling sideways, `document.scrollingElement.scrollWidth` wider than the window, even by 1px;
- content spilling out of its box: text or an image whose right edge passes the edge of a box that neither
  clips nor scrolls (`overflow-x: visible`), by more than 3px. A box that clips or scrolls has chosen what
  happens to the extra and is left alone, and only the innermost spilled box is reported;
- a `<time>` without a `datetime` attribute, or with Go's zero time in it;
- visible text or a tooltip holding what Go prints for a zero or default-formatted time: `0001-01-01`,
  `+0000 UTC` or `m=+`.

A word split is not reported when the run of characters it belongs to is wider than its box: that token
has to break or spill, and breaking it is the right call.

**List rows and table cells.** Their text (a `td`, a project's name and description, a record or person
row) is judged without that exemption, because a collapsed cell makes every word in it "wider than its
box": that is how a project name broken down to one letter passed. A split of a run of up to 24
characters between spaces is reported, as `one letter left on a line` when a piece is a single
character; a longer run is a token, and breaking it is reported only when it would have fitted. A cell
whose content overflows it is the spill check's, below. Rendered Markdown and controls are left to the
other checks.

`--hostile` adds projects named `QA`, `Infrastructureplatform` (key as long) and
`Инфраструктура и мрежи`. The `desktop` viewport (1280x800) is the width between the tablet and the
laptop where the project grid had two columns and no room, and `--locale bg-BG` sends that
`Accept-Language`, so the screens are drawn with their longer Bulgarian labels. `just ui-rows` runs the
listings, the projects screen and the table screens at phone, tablet, desktop and laptop, then the
projects and table screens again in Bulgarian.

`--hostile` also adds, through the API after signing in, sixty-four tasks in the `web` project with long
titles, long tags, unbroken tokens and Markdown bodies with tables, code and quotes, a project with a long
name, a task with a long history of edits and comments, users with long names and two dozen tokens, half
of them expiring.

## UI regression

The **UI regression** job in `ci.yaml` runs on every pull request and every push to `main`, and has to be
green before a merge. `just ui-regression` runs the same steps locally:

| Step | Recipe | What it runs |
| --- | --- | --- |
| Page check on every route | `just ui-check` | every route at `phone` and `fullhd`, light and dark, `--hostile --strict` |
| Task rows and cards | `just ui-rows` | the listings, projects and table screens at phone, tablet, desktop and laptop, and in Bulgarian |
| Browser tests | `just ui-browser` | the Go tests named `*InABrowser`, which drive a real Chrome, and the launcher's tests |
| JavaScript tests | `just jstest` | `internal/web/jstest` |

A test that drives a browser is named `...InABrowser` so this job picks it up, and starts Chrome through
`launchChrome` in `internal/web`. Chrome on a loaded runner sometimes never writes `DevToolsActivePort`;
the launcher gives it 45 seconds, notices at once when it exits instead, and tries again up to three
times, each with a fresh profile, with `--disable-dev-shm-usage` and `--no-sandbox`. Its own tests drive a
stand-in browser that fails twice before starting.

The screenshots and contact sheet are uploaded as the `ui-regression-screenshots` artifact when the job
fails, and kept for five days when a pull request touches `internal/web` or `scripts/screenshots`.

`scripts/screenshots/known.json` lists findings already reported and owned by somebody, each with why, so
the gate can run while they are fixed. A listed finding is printed under `known:` and does not fail the
run. An entry that matches nothing on a page the run captured is stale and fails `--strict`, so the change
that fixes a finding deletes its entry in the same commit. An entry marked `"popup": true` is only judged
by a run that opens popups, and one marked `"varies": true` depends on the fonts installed and is never
stale. CI's runner is the reference. Never add an entry to get a change of your own
through.

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
