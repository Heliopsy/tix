# Roadmap

v1 scope is the `tix-v1` OpenSpec change. Below is what has shipped since, then what comes after,
with the seam in v1 that lets each planned item be added without a rewrite.

## Shipped since v1

Grouped by area. The longer entries were written when that work shipped and record where it came
out different from the plan. [CHANGELOG.md](CHANGELOG.md) lists every release.

### Agents and the API

#### An OpenAPI document generated from the registry

`GET /api/v1/openapi.json` serves an OpenAPI 3.1 description of the HTTP API, with no credential
needed. It is generated from the capability registry that already binds every service method to its
command, route and screen, and committed as `api/openapi.yaml`. CI fails while the committed file
differs from what the registry generates, so the document cannot fall behind the routes it describes.
Every operation carries a summary and description taken from the service method's own doc comment.
See [docs/api.md](docs/api.md).

#### MCP server mode, as `tix mcp`

`tix mcp` serves the HTTP API to an MCP client over stdio, one tool per JSON operation. It came out
thinner than the plan, which was a third adapter mapping tools onto Service methods by hand. The
tools are generated at startup from the OpenAPI document the binary carries, and each call is one
request through the HTTP client with the caller's token, so a tool cannot drift from the API. Left for
later: a streamable HTTP transport, which needs its own answer to whose token a remote session runs
with. See [docs/mcp.md](docs/mcp.md).

#### Every API field from the command line

The registry already proved every operation had a command. A test now also proves the command can
set every field the operation's input takes, and the flags that were missing were added, so nothing
the API accepts needs `curl` instead of `tix`.

#### Leases per task, per project and per tenant

A claim that names no `--ttl` takes the first lease length set on the task, its project, an authored
workflow, the tenant or the deployment, in that order. `tix task add --lease` and `tix task edit
--lease` set a task's own, `tix project edit --lease-default` a project's, and the tenant screen the
tenant's. A ceiling caps all of them: the lowest of the deployment's `lease.max`, the tenant's and the
project's, and a project can only lower its tenant's. `tix task show` and the API report which level a
lease came from. See [docs/agents.md](docs/agents.md#how-long-a-lease-lasts).

#### An administrator can end a held lease

`tix claim reclaim` ends a lease without the holder's token, for the agent that died without releasing.
It needs `task:reclaim`, which `task:claim` does not include, so an ordinary worker cannot end another
worker's lease. The browser and the terminal offer the same action to an administrator.

#### Tokens that expire by default

`tix token create` mints a token that stops working after ninety days unless `--expires never` is
given. This was a breaking change for scripts that relied on an omitted `--expires` meaning never. An
administrator can revoke any member's token from the browser, and a webhook's generated signing
secret is shown once, right after it is saved.

### Capturing and organising work

#### Quick-add syntax

One line can carry a task's attributes: `Fix login !high #auth @raj due:fri +web ^infra-3` sets the
priority, a tag, the assignee, the due date, the project and the parent. The browser's add box and the
terminal's new-task prompt always parse it; `tix task add --parse` and `"parse": true` on the API parse
only when asked, so a script passing a title it did not write gets that title back.
[docs/features.md](docs/features.md) has the full token table.

#### Descriptions and comments in Markdown, with checklists

Descriptions and comments render as CommonMark with GitHub's tables, task lists and fenced code. A
`- [ ]` line is a checklist item that can be ticked in place in the browser, with `tix task check`, or
with `[` in the terminal, and rows show a `3/5` count. Raw HTML is shown as typed and images are never
loaded, only linked.

#### Task templates

A task can be saved as a template for one project or the whole tenant, with its description,
priority, tags, custom fields, repeat rule and, when asked, its subtasks. A new task is created from it
in one step, and `{DATE}`, `{N}` and `{PROJECT}` in a template's title are filled in at each use. It
works from the browser, the terminal, `tix template` and `/api/v1/templates`.

#### Repeating tasks

A task can carry a rule such as `every 2 weeks on mon,thu` or `3 days after done`, and finishing it
creates the next occurrence in the same write. Nothing is generated ahead of time, so a series has one
open task, and a chore finished three weeks late comes back once rather than three times overdue.
Cancelling ends the series.

#### Stars

A star puts a task first for you under every sort, on every surface: the browser, `*` in the terminal,
`tix task star` and the API. It is stored with your actor, so it follows you between surfaces and
never reorders anyone else's list.

#### Snooze

A snoozed task drops out of every default listing and every claim queue until its wake time, then
comes back without anything having to run. Snooze from the task screen, `b` in the terminal or
`tix task snooze REF UNTIL`. `is:snoozed` lists what is asleep.

#### Due dates on every surface

`tix task add` and `tix task edit` take `--due`, `tix task ls` takes `--overdue`, `--due-before` and
`--due-after`, and the browser and the terminal show and mark what is due or late. Before this a due
date could be stored but not set from the CLI, seen in a listing or filtered on.

#### Blocked, waiting and cancelled as their own categories

A workflow state used to be open or terminal, so cancelled counted as done and blocked as in progress.
States now carry a wider category, and colour, counts and statistics follow what the state means.

#### Moves through more than one state

`tix task mv REF done --hops` finds a legal route through the workflow, prints it and applies it one
ordinary transition at a time, so every guard, audit entry and event fires as it would by hand. The
terminal's transition picker and the browser's status control offer the same routes.

#### Copying a task to any project

"Copy to…" on the task screen, `Y` in the terminal, `tix task copy REF --to PROJECT` and the API copy a
task into any project the reader can create tasks in. Comments, subtasks and dependencies come along
only when asked, and a linked copy names its original.

#### Bulk actions and CSV export

Tick rows on the browser's task list and one action applies to all of them: move, priority, tags,
assignee, star, copy or delete, with a single Undo. `tix task bulk` and `POST /api/v1/tasks/bulk` do
the same, and each task still gets its own history entry. A listing exports as CSV from the View menu,
`tix task ls -o csv` or `?format=csv`.

#### Project keys, names, icons and colours

A project's key can be changed, and the old key stays an alias, so old references and links keep
working. Pickers name a project the way a person thinks of it and keys are drawn in upper case. A
project's icon can be a built-in symbol or an uploaded image as well as an emoji or monogram, and any
colour field takes a custom colour beside the palette.

#### Deleting a project asks about its tasks

Deleting a project that holds tasks asks whether to move them to another project or delete them with
it, and does either completely, with an audit entry and an event per task. Before this the database
cascade removed everything silently.

### Finding work

#### A filter that says when it does not understand, and a list you can walk back

Using the seeded demo for a second afternoon turned up the same shape once more, this time in the
answers instead of the screens. `tix task ls -p nosuchproject` returned `[]` and exited 0, which is
the one answer that cannot be told apart from a correct one: it reads as "that project has no work"
rather than "there is no such project", and a person or an agent acts on it. Every
reference-shaped term of a filter is resolved before the listing is queried now, and one that names
nothing fails as `not_found`. Project keys and statuses are matched without regard to case, because
`-p INFRA` selecting nothing was the same silence by another route.

A later tidying could easily get two of those decisions wrong. A tag is
never checked: a tag comes into being by being applied, so "no such tag" and "a tag with no tasks"
are the same state, and refusing the first would refuse the second. A status is checked against the
union of every workflow in the tenant, not just the workflow of the project in scope, because
workflows need not agree and a cross-project listing may reasonably name a status only one of them
defines.

`tix claim next -s <typo>` was the sharpest edge. It answered `no_task_available`, and the published
skill tells an agent to read that as "nothing to do right now", so a misspelled status would have had
a worker idling for ever against a queue that was never empty. It is `not_found` now. A failed
listing also stopped writing a document to standard output, where a closing `[]` beside an error on
standard error let a pipeline reading only stdout believe an empty answer.

The listings gained one shared position control. Six of them had one each before, none of which said
where the reader was and none of which offered a way back. Previous is the harder half: keyset
pagination only goes forward, so the page carries the cursors it walked through in its own URL and
pops the last one off, which keeps the whole position shareable and asks nothing of the store. The
trail arrives from the address bar, so it is validated and bounded, and a bad one is discarded
whole instead of repaired.

The rest was the same lesson in smaller places. The column cookie recorded the columns shown, which
cannot tell a column the reader turned off from one that did not exist when they chose, so every
column added later read as refused by exactly the readers who had customised most; it records what is
hidden now, behind a version marker, and converts what it finds in the old form. A refused filter
reports on the filter bar with the expression still in the box, not on the full-page error
screen. A tenant administrator gets the configuration navigation by default, since the tenant screen
was otherwise reachable only by typing its URL. Durations became one vocabulary, Go's syntax plus a
day, so anything the product prints can be typed back, and the binary embeds `time/tzdata` so a
reader's zone does not depend on the base image.

#### Negation, weak matching and a resumable watch

The filter language gained negation (`-tag:ops`) and weak matching (`title~api`), `tix watch` can
resume from where a previous run stopped, `tix tenant use` makes a tenant choice stick, and the
terminal switches tenant without leaving. See [docs/filtering.md](docs/filtering.md).

#### Saved filters, My work and recently viewed

A filter keeps its expression, sort and view under a name, private or shared with the tenant, and the
browser lists saved filters in its sidebar. `assignee:me` and `tix task ls --mine` mean the reader, My
work in the sidebar groups the reader's tasks by deadline, and the sidebar remembers the last ten tasks
opened.

#### A command palette in the browser

ctrl+k (command+k on a Mac) opens one search box over tasks, projects, screens, settings and actions
such as New task or switching the view. Task search runs the filter language on the server.

#### Statistics

`tix stats`, a statistics screen in the browser and a page in the terminal, over one window
and one optional project: completions per day, median and slowest lead time, where the work
sits, who moved it, and what has been waiting longest.

The leaderboard prints what it counts every time it is shown. A count of tasks moved to a
terminal state is not a measure of how much anybody did, and a number presented without that
sentence gets read as one. The wording lives in `core` so the command line and the browser
cannot drift apart on it.

**What differs from the plan:** attributing a completion to an actor needed the audit
trail. The tasks table records when a task reached a terminal state and never by whom, so the
query matches the transition entry written in the same instant. A completion whose transition
has aged out of retention reports no actor instead of the wrong one.

See [docs/statistics.md](docs/statistics.md).

### Trash and housekeeping

#### Trash and restore

A deleted task goes to the trash. `is:trashed` lists it, the browser has a Trash screen, and
`tix task restore` brings a task back. A tenant can turn its trash off, so that deletes are permanent.

#### Housekeeping per tenant and per project

Finished and cancelled tasks can go to the trash on their own after a number of days, and the trash
can empty itself after another. The tenant sets the rules with `tix cleanup set` or the Tenant screen,
a project overrides any of them from its board, and `tix cleanup preview` shows what would go without
changing anything. A project's board also offers a clean up now that applies its rules at once. See
[docs/configuration.md](docs/configuration.md#housekeeping).

### Notifications and calendars

#### An inbox and notification channels

Each person gets an inbox item when somebody mentions them, assigns them a task, comments on a task
they are assigned to or starred, or moves a task assigned to them; never for their own changes. The
sidebar keeps an unread count live, and `tix inbox`, `I` in the terminal and `/api/v1/inbox` cover the
rest. A person can also send their items to Slack, Discord, ntfy or their own endpoint, delivered and
retried by the webhook dispatcher.

#### Calendar feeds

`tix calendar create`, or Settings in the browser, issues a private iCalendar address for Google
Calendar, Apple Calendar or Outlook to subscribe to. Every unfinished task assigned to you with a due
date becomes an all-day event, or whatever a filter expression selects. The address reads as you and
stops working when it is rotated or revoked. See [docs/calendar.md](docs/calendar.md).

#### Screens that keep themselves current

The task list, the board and a task's own screen update when something commits, from another tab, the
terminal or an agent, and say what changed. The browser's live connection reconnects with backoff and
resumes from where it stopped, and events reach subscribers on the commit itself, from PostgreSQL's
notifications or an in-process wake, instead of on a polling tick.

### The browser

#### The list, and an interface that stops lying about state

An afternoon in front of the seeded demo produced the same shape of defect again, on the
screen people spend the most time on.

The status pill is a control now, offering the moves the row's own workflow allows, including
states reachable only through another state. The whole route is spelled out before it is
applied and each hop is an ordinary transition, so the audit trail carries one entry per step:
a task that passed through a state really did pass through it. That control was unusable at
first. It was a panel positioned inside the task list,
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

Format and timezone are per-browser preferences, with the configured values as the
default. The obstacle was that the time style
was bound into the templates at parse time and shared by every request; template sets are
cached per resolved style now, and a test drives concurrent readers to prove none is served
another's zone.

`ListActors` runs the full length of the contract, service to CLI to HTTP to a `/actors`
screen, and feeds the assignee field, which suggests handles so nobody has to type an
identifier from memory. It suggests without constraining, because an actor from another tenant
is deliberately assignable and a control that only accepted what it listed would remove that
silently.

The rest was the list telling the truth about itself: lease badges for held and claim-expired,
so an abandoned claim stops looking like untouched work; a date that says it is the last
update; Columns and Projects merged into one View panel, because they are one decision; a
Clear control on the filter, since a project chip writes one in a click and getting out should
cost a click too; and a way back to the listing a task was opened from.

See [docs/web-ui.md](docs/web-ui.md).

#### The browser interface, rebuilt

Every screen had a design pass: navigation, the task list and board, statistics, status, connections
and webhooks, with one page rhythm throughout. The sign-in screen became a finished card that follows
the tenant's theme, and the colour scheme moved onto a switch in the sidebar that works with
JavaScript off.

#### List, board and lanes

The task screen switches between a list, a board that merges the workflows in view, and lanes, one per
project, holding that project's open work as cards. Every board column and lane has a three dots menu
to hide, star, collapse or move it, kept per person on the server so it follows them to another
browser, the terminal and `tix view`. A lane card's own three dots menu carries the whole task.

#### A task screen that reads like an issue

The task screen is a header, a main column with the description and the conversation, and a properties
panel. A task opened from a listing can open in a dialog over it, a listed title can be renamed in
place, and status, priority and delete answer in place with an Undo.

#### Forms that read as forms

New task and new project have a real layout, keep what was typed when the server refuses it, and
adding a task can leave you on the list you were adding to.

#### A sidebar that fits how it is used

The sidebar resizes, collapses to a rail and folds its groups, and it is ordered by use: Search, Inbox,
My work, Tasks and Projects first, with pinned projects under Projects and the configuration screens in
one folded group.

#### A settings screen

Every session preference lives on one Settings screen, in sections: Appearance, Listings, Keyboard,
Dates and times, Inbox, Notifications, Second factor, Calendar feed and, for an administrator,
Database. Each row names the setting, says in one line what it does and has its control beside it, and
on/off settings are switches that still post with scripting off. Hover effects can be turned off
there.

#### The interface in Bulgarian

The browser draws every screen in English or Bulgarian, following `Accept-Language` unless Settings
picks one, and explains the platform's own terms with info tips.

#### The browser on a phone

On a phone the sidebar becomes a brand bar with one menu button. The board scrolls sideways as one
strip instead of stacking its columns, lanes snap one at a time, pickers and menus rise from the bottom
of the screen, and the task screen puts the properties straight under the title.

#### Release notes in the app

The running version in the sidebar footer opens a page the install serves itself: the running
version, the newest one, and what changed between them, read from the `CHANGELOG.md` embedded in the
binary. When a newer release exists, the update notice links there.

#### Theming across every surface

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

**What differs from the plan:** the palette is an accent pair, not a scheme per
surface. The web interface already has light, dark and dim for everything structural, and a
terminal cannot honour a browser's palette, so the themeable thing is the accent. State and
priority colours are deliberately excluded: they carry meaning, and a tenant that themes itself
red should not lose the red that means blocked. See [docs/theming.md](docs/theming.md).

### The terminal and SSH

#### The terminal interface over SSH

`ssh tix.example.com` lands in the terminal interface, with no client to install and nothing to
configure: the same board and the same keys, over a protocol every machine already has.

`charmbracelet/wish` serves the bubbletea program over SSH, forwards window resizes, and resolves the
colour profile from the client's `TERM`. The work was authentication. `tix ssh` serves keys enrolled
with `tix user key add`, each bound to an actor, and refuses a key nobody enrolled, so a session goes
through the same authorization decisions as every other surface. The sandbox that gave any key an
ephemeral tenant is still there, behind `--demo`, because handing a tenant to a stranger should be a
deliberate choice.

The listener carries the usual cost of one: a host key persisted beside the database, per-address
connection rate limits, per-key and total session caps, keepalives, and the same refusal to bind a
non-loopback address without an explicit choice that `tix serve` makes. `tix connection ls` shows who
is connected and `tix connection kill` ends a session. Session isolation is proved by a test, because a
shared map behind an SSH handler would be a cross-tenant leak.

**What differs from the plan:** the plan's seams held. `tui.Options` already took `In` and `Out`, so a
session's streams are handed to the interface directly, and `internal/connect` already resolved a
target per invocation. The public demo it suggested is the `--demo` sandbox, which gives each
visiting key a seeded tenant of its own rather than one shared read-only tenant. See
[docs/deployment.md](docs/deployment.md).

#### The terminal caught up with the browser

The terminal interface confirms before anything destructive and asks with real forms. It has a project
screen with its workflow and field definitions, history, artifacts, restoring a task, an actor
directory, and administration of projects, workflows, fields, members, tokens and webhooks. ctrl+k opens
a command palette built from the same list as the help overlay, and the board was redrawn around
title-led cards.

#### A board you can open over SSH by name

`ssh -t tix@host infra` opens that project's board directly. A session with no command lands on a start
menu of what the listener can host.

#### `tix watch` over SSH

`ssh tix@host watch --project house` streams the same lines `tix watch` prints, as the key's actor and
scoped to its tenant.

#### Logging in from the CLI with an SSH key

`tix login --ssh-key` signs a server-issued challenge with an enrolled key, from `ssh-agent` or a key
file, and gets the same session a password login does. A second factor still applies.

#### SSH behind a load balancer

`tix ssh` reads the real client address from a PROXY protocol header, only from proxies on its trusted
list, so the audit trail and per-address limits count the client instead of the proxy. A key nobody
enrolled no longer holds a session slot while it is looked up.

### Accounts and security

#### A second factor

A tenant can require a time-based code in the browser, for `tix login` and over SSH, each on its own.
Recovery codes cover a lost phone, an administrator can clear an enrolment, and API tokens are exempt.
See [docs/second-factor.md](docs/second-factor.md).

### Running it

#### Shell completion that installs itself

`tix completion install` detects the shell, writes the script to that shell's own completions
directory, reports the path, and takes `--shell`, `--dry-run` and `--uninstall`.

**What differs from the plan:** it never edits a startup file. The original note called a
completions directory "the better target than the startup file, so the block is a fallback";
in practice all three supported shells have one, so the fallback was never needed and the
marker-block machinery would have been code nobody reaches. Writing into a file the user did
not name is also the part that is hard to undo, and this way `--uninstall` removes exactly one
file it wrote.

#### Self-update from the CLI

`tix update` replaces this binary with a release built for its platform, verifying the
archive against the checksum published beside it and renaming the replacement over the
target so an interrupted download cannot leave a truncated binary on PATH. `--check`
reports without writing, and `--version` installs a named release, including an older one.

**What differs from the plan:** the refusals are decided by the recorded VCS revision
instead of the module version. A `go build` from a tagged tree records a module version
indistinguishable from an installed one, so version alone refused a locally built binary as
"installed with the Go toolchain"; only `go install pkg@version` has no checkout and
therefore no revision. Running the command against a real release found this, not the unit test,
which had been fed plausible-looking values.

Restarting a running server is still not part of it, for the reason the plan gave.

See [docs/upgrading.md](docs/upgrading.md), which is explicit that the checksum proves the
download matches what was published and not that the release is authentic; the cosign
signatures are the stronger guarantee and verifying them needs a verifier this binary does
not carry.

#### A container image

Each release publishes a signed multi-architecture container image beside the binaries.
See [docs/deployment.md](docs/deployment.md).

#### `tix status`

`tix status`, `/api/v1/status` and a Status screen report what an installation holds: the engine, the
database, the tenants, the open work, and every server process with its uptime. A running `tix serve`
registers itself and heartbeats so the report can say who is serving.

#### Logs, page sizes and other configuration

Logs can go to a file that rotates by size and age. The CLI and the browser each have their own page
size. The configuration keys `server.listen` and `server.token` are read as documented.

#### A backlog worth looking at, and durations a person can read

`tix demo seed` fills a database with a window of history: projects, people, agents, custom
field definitions, and tasks carrying descriptions, tags, priorities, assignees, due dates and
comments, with completions spread across the window by several actors. Every screenshot until
then showed titles and nothing else, which is the opposite of what the screens are for.

The history is replayed through the ordinary service calls against a clock the command
advances, not written behind them. This was forced: statistics
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

### Import, export and sync

#### Import and export on one screen

The browser's import and export are one screen that says what each does, and the sync screen shows
what each source has done.

#### One tick, and sync that keeps its own state

A release spent on defects that shared a shape: each was covered by a test asserting the code
returned the right value, and none asserted what a person experiences.

The completion animation was attached to the completed state instead of the act of
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
`Kinds`, and the tests iterate `Kinds` itself. Before, they iterated a hand-kept copy that
called itself complete while nothing checked it. `just docs-check` now covers
`screenshots/` as well as `docs/`, after the screenshot index drifted from its own directory.

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
model name are configuration, with no hard dependency on one vendor, and a local runtime
serving that API works without tix knowing the difference.

This is the first feature that sends task content off the machine, and that changes the
product's promise, so it has to be built defensively:

- **Off unless switched on**, per project, never globally by default. A tracker that quietly
  posts a private task list to a third party because a key was present in the environment is a
  breach, whatever the release notes said.
- **Say what leaves.** A dry run shows the exact payload before anything is sent, and the
  configured endpoint is visible in `tix doctor` instead of buried in a config file.
- **Attribute the writes.** Enrichment mutates tasks, so it runs as a system actor and its
  changes land in the audit log like any other mutation. "Who wrote this description" must have
  an answer.
- **Never overwrite a human.** Enrich empty fields, and where a person has already written
  something, offer a suggestion instead of an edit. A model should never silently replace a field
  a person typed.
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
  field is free text a person types, with no schema to constrain it.

An instruction is per project, which is the unit people already think in, and it inherits from
the tenant when a project has none, so a house style is written once.

**Seam in v1:** `actors` already distinguishes `user`, `agent` and `system`, so an enrichment
worker is an actor that claims and writes like any other. A project instruction is a column on
`projects` and a field on the project input, reachable from every surface the way a project's
colour and icon already are. The webhook dispatcher already
establishes the pattern for a bounded background worker with retry and backoff that a server
owns and a direct-database CLI drains opportunistically. Every mutation already writes rows,
audit and event in one transaction, so enrichment inherits the audit trail and needs none of
its own. Custom fields already hold whatever a model produces that has no typed column.

### Read-only view links

A link that carries a whole view: a board or a list, its filter, its sort, and the columns
worth showing. Open it on a wall display, a kiosk tab or a Home Assistant dashboard and it
renders without signing in, refreshes itself, and offers no way to change anything.

**Seam in v1:** the view is already addressable. Filters, sort keys and column selection are
already query parameters on the web routes, so the URL for a board filtered the way somebody
wants already exists; what is missing is a way to open it without a session. `api_tokens`
already carries scopes, so a read-only view is a token whose scopes permit reads and nothing
else, revoked the same way as any other. The event stream already exists, so a display can
update when something commits instead of polling on a timer. The web interface already
renders server-side with no build step, which is what makes a cheap browser on a fridge door
a reasonable target.

Calendar feeds have since shipped the same shape for one narrow view: a private address that reads
as one person, scoped to their due dates or a filter, and rotated or revoked on its own. A view link
is the same idea for a board or a list.

Two things have to be decided before any of that matters.

The first is how the link authenticates. A long unguessable URL is the shape people expect,
and it is also the shape that leaks: it lands in browser history, in proxy logs and in a
`Referer` header on every outbound click. A token in a header is safe and cannot be typed
into a television. Whatever is chosen should be scoped to one view, not one tenant,
and revocable on its own, so a display in a corridor does not become a key to the backlog.

The second is the preferences. Scheme, date format and timezone live in a cookie today
because they belong to the person reading, not the tenant. A kiosk has nobody to set
them, so for these links they have to travel in the link itself.

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

It must not become a second, weaker export path. The dump is the existing snapshot
format, written by the same code, so a backup is restorable by the importer that already exists
and is already tested. If the two ever diverge, the backup is the thing that will be wrong on
the day it matters.

Two things it has to get right, because getting them wrong is embarrassing later:

- **Credentials stay out of the snapshot and out of the logs.** A dump carries task content, and
  the target carries a secret key. Neither belongs in the other, and the redaction that already
  covers DSNs covers this too.
- **A failed backup is loud.** A target that has not succeeded since Tuesday is the whole
  problem with backups, so a stale target is a `doctor` failure and an event on the stream, not
  a line in a log nobody reads.

### Rate limiting and security headers

Two things a server facing the internet needs. Part of the groundwork has shipped; the two
headline pieces have not.

**Rate limiting.** What is limited today: the SSH listener, per source address and per key, and
second-factor codes, where repeated wrong codes lock verification for a while. Password checks are
bounded in how many run at once, which protects the CPU but is not a limit on guessing. Nothing limits
the HTTP login form, token authentication or the event stream. The login form is the sharpest edge,
because a password endpoint with no limit is a password endpoint with an offline-speed online attack
against it. Limits belong per credential and per source address, not only per tenant, since one
tenant's runaway agent should not exhaust everyone else's budget, and an unauthenticated attacker has
no tenant at all.

The prerequisite is in place. `server.trusted_proxies` names the reverse proxies whose
`X-Forwarded-For` and `X-Forwarded-Proto` are believed, and with none configured neither header is
read, so a client cannot spoof its address. `tix ssh` reads the PROXY protocol from its own trusted
list. A limiter by source address therefore has a real address to count.

**Security headers.** `X-Content-Type-Options: nosniff` is set across the HTTP surface, an uploaded
project icon is served under its own restrictive Content Security Policy, and calendar feeds send
`Referrer-Policy: no-referrer`. Missing: an application-wide Content Security Policy,
`Strict-Transport-Security`, a general `Referrer-Policy`, `Permissions-Policy`, and frame-ancestors.

CSP is the one with real work behind it. `layout.html` carries inline scripts, deliberately, so
the theme and the dismissed announcement apply before first paint instead of flashing. A
policy strict enough to be worth having cannot allow `unsafe-inline`, so those scripts need
nonces threaded through the render path, or hashes computed at build time. Rendered Markdown already
carries no inline style or script, so it holds under such a policy. Everything else on the list is a
header and a test.

**Seam in v1:** the HTTP middleware chain already exists in `internal/httpapi`, authentication,
tenant resolution and forwarded-header handling already run there, and a limiter is another link in
it. The SSH listener's token bucket in `internal/sshd` is the shape to reuse. `tix serve` already
refuses a non-loopback bind without TLS or an explicit override, so the deployment shape these
protect is already the documented one.

### Plugins as commands

Specified in the `plugin-system` OpenSpec change and not built. A plugin is an executable that tix
installs into a directory it owns and runs when `tix <name>` matches no built-in command, the way `git`,
`kubectl` and `gh` extend themselves. A manifest records each plugin's source, version and digest;
`$PATH` is never scanned; and a plugin gets no credential unless an operator grants it a scoped token at
install time.

**Seam in v1:** the CLI and the HTTP API are already the stable contracts a plugin would talk to, every
command answers `-o json`, exit codes are documented, and `internal/connect` already resolves the target
a plugin would be handed.

### iOS app

[tix-mobile](https://github.com/heliopsy/tix-mobile) is the Flutter client, and its iOS build exists
but is not published: there is no signing yet, so it is built from source. Today it is a client of a tix
server over the HTTP API. The app also has screens to create or open a local `.db` file, behind a build
flag, waiting for the engine.

**What exists:** `just lib-ios` builds `Tix.xcframework`, a static archive the app links into its own
binary and calls through the C ABI from `dart:ffi`, so tix runs in the app's process on a loopback port
with no second process, which iOS would not allow. See [docs/embedding.md](docs/embedding.md). The
libraries are built and tested in CI but not attached to releases.

**Still needed:**

- Release wiring for the xcframework, so the app pins a tix version and verifies a checksum instead of
  building tix itself.
- Signing in CI, TestFlight, App Store Connect, a privacy manifest and the review.
- iOS suspends the app in the background, and the embedded instance with it, so a local database does
  no lease sweeps or webhook deliveries while the app is closed. That is fine for a personal file and
  wrong for one agents also work on.
- Notifications need a sender on the server side through APNs, which is a tix change before it is an
  app change, and has to work for a self-hosted server without a hosted relay.

### Android app

The Android build of [tix-mobile](https://github.com/heliopsy/tix-mobile) is released on GitHub as an APK
per ABI and an AAB, as a pre-release client of a tix server. Local databases are behind a build flag.

**What exists:** `just lib-android` builds `libtix.so` for all four ABIs with the NDK, aligned for 16 KB
pages as Google Play requires, and CI checks the alignment on every change. The app loads it from
`jniLibs` with `DynamicLibrary.open`. The library carries the whole server, about 32 MB per ABI before
compression.

**Still needed:**

- Release wiring for the libraries, and the embedded engine in the app's release build.
- A Google Play listing and review. F-Droid needs a reproducible build from source with no proprietary
  dependencies.
- Android limits background work too, so the embedded instance stops with the app, as on iOS.
- Notifications through FCM, needing the same server-side sender as iOS.

### Windows app

The Windows build of [tix-mobile](https://github.com/heliopsy/tix-mobile) is released as a `.zip` for
Windows 10 x64, a client of a tix server, with the token kept by DPAPI. This replaces the earlier plan
of a webview wrapped around the browser interface: the native client shares one codebase with the phones.

**What exists:** `just lib-windows` builds `libtix.dll` with mingw-w64, loaded from beside the executable,
so a local database needs no `tix` binary installed or supervised. Loopback is not private on a shared
machine, so the app keeps the token it mints in the platform's secure storage.

**Still needed:**

- The embedded engine in the release build, and release wiring for the DLL.
- Code signing, so SmartScreen opens the app without a warning, and an installer or a store package in
  place of a `.zip`.

### Linux app

The Linux build of [tix-mobile](https://github.com/heliopsy/tix-mobile) is released as a `.tar.gz` for x64
and arm64 on GTK 3, a client of a tix server, with the token in libsecret.

**What exists:** `just lib` builds `libtix.so` for the host, installed under `lib/` beside the executable.
CI builds it and drives it through the C ABI from a C program on every change.

**Still needed:**

- The embedded engine in the release build, and release wiring for the library.
- Packaging beyond a tarball: a Flatpak manifest building from source for Flathub, and possibly an
  AppImage or a `.deb`. Which ones is undecided.

### macOS app

The macOS build of [tix-mobile](https://github.com/heliopsy/tix-mobile) is released as a `.zip`, ad-hoc
signed only, a client of a tix server with the token in the Keychain.

**What exists:** `just lib-macos` builds a universal `libtix.dylib`, arm64 and x86_64, with the install
name `@rpath/libtix.dylib`, embedded in `Contents/Frameworks`. CI builds it and drives it through the C
ABI on macOS.

**Still needed:**

- The embedded engine in the release build, and release wiring for the dylib.
- Developer ID signing and notarization, so Gatekeeper opens the app without a right-click, which the
  embedded dylib has to pass as well.

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
