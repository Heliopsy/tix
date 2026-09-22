# Roadmap

v1 scope is the `tix-v1` OpenSpec change. This file records what comes after, and
the seam in v1 that makes each one additive rather than a rewrite.

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

### Theming for the web interface and the TUI

User-supplied colour schemes on both surfaces, rather than the fixed palettes plus dark and
low-contrast modes that v1 ships.

**Seam in v1:** the web interface already defines every colour as a custom property on
`:root`, redefined per theme scope, so an additional theme is a block of property values and
no template change. The TUI resolves colour through pure functions tested against
`output.Painter`, so a palette becomes another input to those functions rather than a rewrite
of the views. Both surfaces already persist a display preference, so a theme name travels the
same path a theme choice would.

### Self-update from the CLI

`tix update` detects how this binary was installed, fetches the matching build, verifies it
and replaces itself, restarting a server that is running from the same path.

Detection has to be honest about what it cannot do. A binary under a package manager's prefix,
or one built from source, should refuse and say which command to run instead, rather than
overwriting something another tool owns. The cases worth handling are the release archive
(replace in place) and `go install` (re-run it).

Replacement is the part that bites. The running binary cannot be overwritten on Windows at
all, and on Unix it must be written beside the target and renamed, so the swap is atomic and a
failed download never leaves a truncated binary on `PATH`. If the target directory is not
writable, say so before downloading rather than after.

Restarting a server is a separate decision from updating, and it belongs behind a flag rather
than happening because an open file handle was found. A tracker that restarts itself while
someone is mid-transition is worse than one that says a restart is pending.

**Seam in v1:** `internal/version` already carries the version, commit and build date from
ldflags, so a build can identify itself. GoReleaser already publishes `checksums.txt` alongside
the archives and signs them, so verification needs no new release machinery, and
`install.sh` already does the platform detection and checksum verification that `tix update`
would repeat. `tix serve` already shuts down gracefully, so a restart has something to wait on.

### Shell completion that installs itself

`tix completion install` detects the running shell, then writes or refreshes a marked block at
the end of that shell's startup file, so completion works in the next terminal without anyone
reading an installation note.

The block is delimited by begin and end markers and rewritten wholesale on every run, which is
what makes it idempotent and what makes `tix completion uninstall` exact. Appending without
markers is how a startup file collects six copies of the same snippet over a year.

Detection should prefer the shell that is actually running over `$SHELL`, which records a login
default rather than the current process. Where a shell has a completions directory of its own,
that is the better target than the startup file, so the block is a fallback rather than the
first choice. Anything ambiguous asks rather than guessing, since this edits a file the user
did not name.

**Seam in v1:** `tix completion` already generates scripts for every shell Cobra supports. The
command grows an `install` and an `uninstall` subcommand; nothing about the generation changes.

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
