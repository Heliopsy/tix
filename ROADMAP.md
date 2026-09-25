# Roadmap

v1 scope is the `tix-v1` OpenSpec change. This file records what comes after, and
the seam in v1 that makes each one additive rather than a rewrite.

## Shipped since v1

### Theming across every surface

A tenant names a theme and both the browser and the terminal interface render its accent.
Several palettes are built in, configuration defines more under `themes:`, and a tenant that
names none gets the product default.

That default changed after the fact. Themes shipped keeping an older behaviour, where an
unnamed tenant's accent was derived by hashing its key and id, so that an upgrade repainted
nobody. In practice it meant a fresh install opened in a colour nobody had chosen, two installs
of the same software looked unrelated, and neither matched the logo, so not repainting bought
less than it cost. A theme also grew an optional dark pair, because an accent picked to read on
white is a smear on near-black and the browser follows the system, which for most people is
dark.

**What differs from the plan below:** the palette is an accent pair rather than a scheme per
surface. The web interface already has light, dark and dim for everything structural, and a
terminal cannot honour a browser's palette, so the themeable thing is the accent. State and
priority colours are deliberately excluded: they carry meaning, and a tenant that themes itself
red should not lose the red that means blocked. See [docs/theming.md](docs/theming.md).

### Shell completion that installs itself

`tix completion install` detects the shell, writes the script to that shell's own completions
directory, reports the path, and takes `--shell`, `--dry-run` and `--uninstall`.

**What differs from the plan below:** it never edits a startup file. The original note called a
completions directory "the better target than the startup file, so the block is a fallback";
in practice all three supported shells have one, so the fallback was never needed and the
marker-block machinery would have been code nobody reaches. Writing into a file the user did
not name is also the part that is hard to undo, and this way `--uninstall` removes exactly one
file it wrote.

### Self-update from the CLI

`tix update` replaces this binary with a release built for its platform, verifying the
archive against the checksum published beside it and renaming the replacement over the
target so an interrupted download cannot leave a truncated binary on PATH. `--check`
reports without writing, and `--version` installs a named release, including an older one.

**What differs from the plan above:** the refusals are decided by the recorded VCS revision
rather than by the module version. A `go build` from a tagged tree records a module version
indistinguishable from an installed one, so version alone refused a locally built binary as
"installed with the Go toolchain"; only `go install pkg@version` has no checkout and
therefore no revision. That was found by running the command against a real release, not by
the unit test that had been fed plausible-looking values.

Restarting a running server is still not part of it, for the reason the plan gave.

See [docs/upgrading.md](docs/upgrading.md), which is explicit that the checksum proves the
download matches what was published and not that the release is authentic; the cosign
signatures are the stronger guarantee and verifying them needs a verifier this binary does
not carry.

### Statistics

`tix stats`, a statistics screen in the browser and a page in the terminal, over one window
and one optional project: completions per day, median and slowest lead time, where the work
sits, who moved it, and what has been waiting longest.

The leaderboard prints what it counts every time it is shown. A count of tasks moved to a
terminal state is not a measure of how much anybody did, and a number presented without that
sentence gets read as one. The wording lives in `core` so the command line and the browser
cannot drift apart on it.

**What differs from the plan above:** attributing a completion to an actor needed the audit
trail. The tasks table records when a task reached a terminal state and never by whom, so the
query matches the transition entry written in the same instant. A completion whose transition
has aged out of retention reports no actor rather than the wrong one.

See [docs/statistics.md](docs/statistics.md).

### One tick, and sync that keeps its own state

A release spent on defects that shared a shape: each was covered by a test asserting the code
returned the right value, and none asserted what a person experiences.

The completion animation was attached to the completed state rather than to the act of
completing, so every render replayed it on every already-ticked checkbox and opening a list
sent a wave of ticks down the page.

External sync lost and misreported its own state four ways. A full refresh blanked the run's
watermark so the source would be read from the beginning, but that watermark is what gets
persisted, so a refresh failing on its first request erased the position every previous run
had earned. The result's watermark was assigned after the audit entry embedding it had been
written, so every entry claimed the run reached nothing. An unreachable source and a bug in
tix were both reported as internal errors, so neither the exit code nor the message told a
scheduled job which had happened; there is a `KindUpstream` now, 502 and exit 7, applied at
the fetch boundary only. And a finished import printed its own Go struct, because
`SyncResult` had no case in the renderer.

**What this changed about the tests:** two mapping tables in `core` are now exhaustive over
`Kinds`, and the taxonomy the tests iterate is `Kinds` itself rather than a hand-kept copy
that called itself complete while nothing checked it. `just docs-check` now covers
`screenshots/` as well as `docs/`, after the screenshot index drifted from its own directory.

### A backlog worth looking at, and durations a person can read

`tix demo seed` fills a database with a window of history: projects, people, agents, custom
field definitions, and tasks carrying descriptions, tags, priorities, assignees, due dates and
comments, with completions spread across the window by several actors. Every screenshot until
then showed titles and nothing else, which is the opposite of what the screens are for.

The history is replayed through the ordinary service calls against a clock the command
advances rather than written behind them. That is forced rather than tasteful: statistics
attribute a completion by matching the transition entry written in the same instant, so a
snapshot import would have left every leaderboard and lead-time figure empty.
`connect.Overrides` grew a `Clock` for this and nothing else uses it. The command refuses a
database that already holds tasks or users unless `--reset` is passed, because nobody should
meet this command by finding invented tasks in their real tracker.

Seeding it immediately showed the statistics screens printing `384h50m23.606839092s`
everywhere except the browser, which had a humaniser of its own the other two surfaces could
not reach. The rendering belongs to `core.Duration` now and all three call it; `String` stays
Go's own, because the snapshot formats are built from it and have to keep the precision the
human form throws away. A guard asserts the statistics table prints no machine-rendered
duration.

### The list, and an interface that stops lying about state

An afternoon in front of the seeded demo produced the same shape of defect again, on the
screen people spend the most time on.

The status pill is a control now, offering the moves the row's own workflow allows, including
states reachable only through another state. The whole route is spelled out before it is
applied and each hop is an ordinary transition, so the audit trail carries one entry per step:
a task that passed through a state really did pass through it. That control was unusable at
first, and the reason is worth recording. It was a panel positioned inside the task list,
which hides its overflow, so on a lower row the choices were clipped out of the scrollable
area entirely, took no pointer events, and could not be reached at any scroll position.
Clicking a choice did nothing, forever, which reads as a frozen page. It is a popover in the
top layer now, and `popovertarget` brings light dismissal, Escape and one-at-a-time with no
script.

Static assets carried no cache validator and their urls never change between releases, so a
browser could serve the previous release's stylesheet against this release's markup with
nothing to say so. That happened during review: a reader saw new markup styled by an old sheet
and reported the interface as broken. Each asset now carries an `ETag` derived from its own
bytes and answers conditional requests.

Dates are the reader's business, not the deployment's. Format and timezone are per-browser
preferences with the configured values as the default. The obstacle was that the time style
was bound into the templates at parse time and shared by every request; template sets are
cached per resolved style now, and a test drives concurrent readers to prove none is served
another's zone.

`ListActors` runs the full length of the contract, service to CLI to HTTP to a `/actors`
screen, and feeds the assignee field, which suggests handles rather than asking for an
identifier from memory. It suggests without constraining, because an actor from another tenant
is deliberately assignable and a control that only accepted what it listed would remove that
silently.

The rest was the list telling the truth about itself: lease badges for held and claim-expired,
so an abandoned claim stops looking like untouched work; a date that says it is the last
update; Columns and Projects merged into one View panel, because they are one decision; a
Clear control on the filter, since a project chip writes one in a click and getting out should
cost a click too; and a way back to the listing a task was opened from.

See [docs/web-ui.md](docs/web-ui.md).

### A filter that says when it does not understand, and a list you can walk back

Using the seeded demo for a second afternoon turned up the same shape once more, this time in the
answers rather than the screens. `tix task ls -p nosuchproject` returned `[]` and exited 0, which is
the one answer that cannot be told apart from a correct one: it reads as "that project has no work"
rather than "there is no such project", and a person or an agent acts on it. Every
reference-shaped term of a filter is resolved before the listing is queried now, and one that names
nothing fails as `not_found`. Project keys and statuses are matched without regard to case, because
`-p INFRA` selecting nothing was the same silence by another route.

Two of those decisions are worth recording because a later tidying would get them wrong. A tag is
never checked: a tag comes into being by being applied, so "no such tag" and "a tag with no tasks"
are the same state, and refusing the first would refuse the second. A status is checked against the
union of every workflow in the tenant rather than the workflow of the project in scope, because
workflows need not agree and a cross-project listing may reasonably name a status only one of them
defines.

`tix claim next -s <typo>` was the sharpest edge. It answered `no_task_available`, and the published
skill tells an agent to read that as "nothing to do right now", so a misspelled status would have had
a worker idling for ever against a queue that was never empty. It is `not_found` now. A failed
listing also stopped writing a document to standard output, where a closing `[]` beside an error on
standard error let a pipeline reading only stdout believe an empty answer.

The listings gained one shared position control. Six of them had one each before, none of which said
where the reader was and none of which offered a way back. Previous is the interesting half: keyset
pagination only goes forward, so the page carries the cursors it walked through in its own URL and
pops the last one off, which keeps the whole position shareable and asks nothing of the store. The
trail arrives from the address bar, so it is validated and bounded and discarded whole rather than
repaired.

The rest was the same lesson in smaller places. The column cookie recorded the columns shown, which
cannot tell a column the reader turned off from one that did not exist when they chose, so every
column added later read as refused by exactly the readers who had customised most; it records what is
hidden now, behind a version marker, and converts what it finds in the old form. A refused filter
reports on the filter bar with the expression still in the box rather than on the full-page error
screen. A tenant administrator gets the configuration navigation by default, since the tenant screen
was otherwise reachable only by typing its URL. Durations became one vocabulary, Go's syntax plus a
day, so anything the product prints can be typed back, and the binary embeds `time/tzdata` so a
reader's zone does not depend on the base image.

## v2

### Bidirectional sync with Jira and OpenProject

v1 imports one way and refreshes idempotently. v2 pushes changes back.

**Seam in v1:** `external_refs` already records `system`, `external_id`, `external_url`,
`external_version`, and `last_synced_at` for every imported entity, and `sync_sources`
holds a per-source cursor. `external_version` is populated in v1 but read only for change
detection; it becomes the conflict-detection field. The `Importer` interface is shaped so
a `Syncer` adding `Push` extends it.

**Still to design:** conflict resolution policy when both sides changed, change-origin
tracking so a pushed change does not echo back as an inbound change, inbound webhook
receivers for Jira and OpenProject, and per-field sync direction.

### SSO / OIDC

**Seam in v1:** `users.sso_subject` and `users.sso_provider` exist and are never read.
`internal/auth` defines an `Authenticator` interface with a local implementation, and the
middleware holds a chain. Adding a provider is a new implementation plus a config block,
with no schema migration and no signature change.

### MCP server mode

**Seam in v1:** `core.Service` is transport-neutral and complete. An MCP server is a third
adapter alongside `internal/httpapi` and `cmd/`, mapping tools onto Service methods.
Nothing in v1 needs to change to add it.

### ACME / automatic certificates

**Seam in v1:** `tenant_domains.cert_mode` already distinguishes how a domain gets its
certificate. v1 supports supplied certificate files; ACME becomes another mode.

### Full-text search on SQLite

**Seam in v1:** search is behind a store method with a PostgreSQL `tsvector` implementation
and a SQLite `LIKE` implementation. FTS5 becomes a third implementation of the same method.
The asymmetry is documented in `docs/scaling.md`.

### Automatic enrichment

A background worker that fills in what a hurried capture left out: a body for a one line task,
a priority, tags, a first pass at subtasks. Enabled per project, or asked for per task with
`tix task enrich`. The model is reached over an OpenAI-compatible API, so the endpoint, key and
model name are configuration rather than a hard dependency on one vendor, and a local runtime
serving that API works without tix knowing the difference.

This is the first feature that sends task content off the machine, and that changes the
product's promise, so it has to be built defensively:

- **Off unless switched on**, per project, never globally by default. A tracker that quietly
  posts a private task list to a third party because a key was present in the environment is a
  breach, whatever the release notes said.
- **Say what leaves.** A dry run shows the exact payload before anything is sent, and the
  configured endpoint is visible in `tix doctor` rather than buried in a config file.
- **Attribute the writes.** Enrichment mutates tasks, so it runs as a system actor and its
  changes land in the audit log like any other mutation. "Who wrote this description" must have
  an answer.
- **Never overwrite a human.** Enrich empty fields, and offer a suggestion rather than an edit
  where a person has already written something. A field a person typed is the one thing a model
  should not silently replace.
- **Ask rather than guess.** Where the capture is too thin to enrich confidently, the worker
  records a question on the task instead of inventing an answer. "Which NAS, the old QNAP or the
  Synology?" is useful; a confident wrong description is worse than an empty one, because a
  person reads it and believes it. A question is also a comment, so it already has somewhere to
  live and someone to notify.
- **Bounded.** Rate limits, a per-project budget, and a cap on retries. An endpoint that is
  slow or down leaves the task exactly as it was.
- **Idempotent.** An enriched task carries a marker so a restarted worker does not re-enrich
  the same rows, and so a queue draining after an outage does not multiply cost.

**Project instructions.** A model given only a task title guesses at a house style it cannot
know. So a project carries a free-text instruction that travels with every enrichment request
for that project: what the project is, what the words mean locally, how long a body should be,
which of two plausible NAS boxes is the one anybody means. It is the difference between a
generic draft and one that sounds like it came from the team.

Two constraints follow from that, and both are the same shape as the rules above:

- **It is sent off the machine**, so it is covered by the same opt-in and the same dry run. The
  instruction has to be visible in the preview of what would be transmitted, not an invisible
  prefix somebody forgot they wrote a year ago.
- **It is not a place for secrets.** Refusing to send a project instruction that looks like a
  credential is cheap and worth doing, because someone will eventually paste one in, and the
  field is text a person types rather than a field a schema constrains.

An instruction is per project, which is the unit people already think in, and it inherits from
the tenant when a project has none, so a house style is written once.

**Seam in v1:** `actors` already distinguishes `user`, `agent` and `system`, so an enrichment
worker is an actor that claims and writes like any other. A project instruction is a column on
`projects` and a field on the project input, reachable from every surface the way a project's
colour and icon already are. The webhook dispatcher already
establishes the pattern for a bounded background worker with retry and backoff that a server
owns and a direct-database CLI drains opportunistically. Every mutation already writes rows,
audit and event in one transaction, so enrichment inherits the audit trail rather than needing
its own. Custom fields already hold whatever a model produces that has no typed column.

### The terminal interface over SSH

`ssh tix.example.com` lands in the terminal interface, with no client to install and nothing to
configure. The same board, driven by the same keys, reached by a protocol every machine already
has.

`charmbracelet/wish` serves a bubbletea program over SSH, forwards window resizes as the
resize message the program already handles, and resolves the colour profile from the client's
`TERM`. The rendering side is close to free.

The work is authentication. Today a credential is a password or an opaque token; this needs a
public key bound to a user, and a way to enrol one. That is a small, well understood mechanism,
but it is a new one, and it has to reach the same authorization decisions everything else goes
through rather than growing a second path.

The rest is the usual cost of a network listener: a persisted host key, connection rate
limiting, and the same refusal to bind a non-loopback address without an explicit choice that
`tix serve` already makes. Session isolation needs proving rather than assuming, because a
shared map behind an SSH handler would be a cross-tenant leak, and that is the one failure this
project treats as unrecoverable.

A public read-only demo tenant is the obvious second use: `ssh tix.red` showing seeded tasks in
the real interface, rather than a video of it.

**Seam in v1:** `tui.Options` already takes `In io.Reader` and `Out io.Writer` and passes them
to bubbletea's `WithInput` and `WithOutput`, so the interface is not bound to `/dev/tty` and a
session's streams can be handed to it directly. `internal/connect` already resolves a target per
invocation, so a per-session target needs no new concept. `golang.org/x/crypto` is already a
direct dependency.

### Scheduled backups to S3 or a directory

A backup is a snapshot plus a schedule plus somewhere to put it, and tix already has the first
one: `tix export` writes a per-tenant snapshot the importer can read back. What is missing is
the other two.

A backup target names where a dump goes and what goes in it: a filesystem path, or an
S3-compatible bucket, which covers S3 itself, MinIO, Backblaze B2, Garage and the rest without
a second implementation. Several targets can be configured at once, because the useful
arrangement is usually more than one: an hourly dump of one busy project to a local disk, a
nightly dump of everything to object storage offsite, and a weekly copy somewhere a different
person controls.

Each target carries its own scope, schedule and retention:

- **Scope.** Everything, one tenant, or named projects. A project that matters more than the
  rest should be backupable more often than the rest, without dragging the whole database along
  each time.
- **Schedule.** A cron expression, evaluated by the server that already runs the lease sweeper,
  the webhook dispatcher and the retention pruner. In direct mode there is no daemon, so the
  same job runs from `tix backup run`, and `tix doctor` says when a target last succeeded.
- **Retention.** Keep the last N, or everything within a window. A backup scheme that fills the
  disk it writes to has replaced one failure with another.
- **Naming.** A template with the tenant, the scope and the timestamp, so a bucket listing is
  legible without opening anything.

What it must not do is become a second, weaker export path. The dump is the existing snapshot
format, written by the same code, so a backup is restorable by the importer that already exists
and is already tested. If the two ever diverge, the backup is the thing that will be wrong on
the day it matters.

Two things it has to get right, because they are the ones that are embarrassing later:

- **Credentials stay out of the snapshot and out of the logs.** A dump carries task content, and
  the target carries a secret key. Neither belongs in the other, and the redaction that already
  covers DSNs covers this too.
- **A failed backup is loud.** A target that has not succeeded since Tuesday is the whole
  problem with backups, so a stale target is a `doctor` failure and an event on the stream, not
  a line in a log nobody reads.

### Rate limiting and security headers

Two things a server facing the internet needs, neither of which v1 has.

**Rate limiting.** Nothing is limited today. The login form is the sharpest edge, because a
password endpoint with no limit is a password endpoint with an offline-speed online attack
against it, but token authentication, the event stream and, once it exists, the SSH listener all
want the same treatment. Limits belong per credential and per source address, not only per
tenant, since one tenant's runaway agent should not exhaust everyone else's budget, and an
unauthenticated attacker has no tenant at all.

There is a prerequisite: **tix does not read `X-Forwarded-For`, `X-Real-IP` or
`X-Forwarded-Proto` anywhere.** Behind a reverse proxy, which is the documented deployment,
every request already appears to come from the proxy. Limiting by source address before that is
fixed would either limit everyone as one client or be trivially evaded, so the forwarded-header
handling has to land first, with an explicit trusted-proxy setting rather than blind faith in a
header any client can send.

**Security headers.** `X-Content-Type-Options: nosniff` is set in three places and nothing else
is. Missing: a Content Security Policy, `Strict-Transport-Security`, `Referrer-Policy`,
`Permissions-Policy`, and frame-ancestors.

CSP is the one with real work behind it. `layout.html` carries inline scripts, deliberately, so
the theme and the dismissed announcement apply before first paint rather than flashing. A
policy strict enough to be worth having cannot allow `unsafe-inline`, so those scripts need
nonces threaded through the render path, or hashes computed at build time. Everything else on
the list is a header and a test.

**Seam in v1:** the HTTP middleware chain already exists in `internal/httpapi`, authentication
and tenant resolution already run there, and a limiter is another link in it. `tix serve`
already refuses a non-loopback bind without TLS or an explicit override, so the deployment shape
these protect is already the documented one.

### Desktop application

A thin wrapper around the browser interface rather than a second client: the same
server-rendered screens in a native window, so there is one interface to build and one to
test. A wrapper keeps the multiplatform cost near zero compared with a native rewrite, and
the CLI already covers the scripted path.

**Seam in v1:** the browser interface is server rendered with no JavaScript build step, and
`tix serve` binds a local address by default. A desktop build starts the server on a
loopback port and points a webview at it, so it needs no new product surface. The open
question is bundling: a webview per platform is small but ties the build to each platform's
toolchain, while shipping a browser engine is large but uniform.

### Tests on arm64 as well as amd64

The release matrix already cross-compiles linux and darwin for amd64 and arm64, and windows
for amd64, but every test runs on amd64 only, so an architecture-specific fault would ship. The
parts most exposed are the pure Go SQLite driver, anything touching unaligned access or
atomics, and the time handling that lease expiry depends on.

**Seam in v1:** `CGO_ENABLED=0` everywhere and a `just ci` that runs the whole gate set in a
container, so the same recipes run on an arm64 runner without change. What is missing is the
runner and a matrix axis in the workflow.

## Considered and deliberately not planned

- Time tracking, billing, sprint and velocity reporting. Different product.
- Real-time collaborative text editing on task bodies. Large cost, narrow benefit.
- A hosted multi-customer SaaS control plane. Multi-tenancy supports it; operating it is out of scope.
