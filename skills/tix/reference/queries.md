# Finding and watching work

Read this when the hot-path listing in SKILL.md is not enough: you need
negation, a deadline, a sort order, the actor behind a handle, a progress
report, or the event stream instead of a poll.

## Filter flags

`task ls` and `claim next` share filter flags; each is a repeatable flag
(pass it more than once, or comma-separate one instance — both work), OR'd
within a flag, AND'd across flags:

```sh
tix task ls -p infra -p ops -s todo -s doing -l ci --unclaimed
tix task ls --query "flaky test"        # substring match over title + body
tix task ls --blocked                   # only tasks blocked by a dependency
tix task ls --recurring                 # only tasks that come back when finished
tix task ls --sort priority --desc
tix task ls -p infra --all              # follow cursors, read every page
```

## One expression instead

One `--filter` expression does the same job in a single string, with
negation and weak matching the flags cannot express. Terms are space
separated and ANDed, a leading `-` excludes, and `~` in place of `:` matches
weakly, anywhere inside the field:

```sh
tix task ls --filter 'status:todo -tag:ops title~deploy'
tix task ls --filter 'claimed:yes'      # what another agent is holding
tix task ls --filter 'is:blocked'
tix task ls --filter 'is:recurring -status:done'
tix task ls --filter 'is:trashed'       # only soft-deleted tasks
tix task ls --filter 'checklist:open'   # a description with items still to tick
tix task ls --filter 'is:snoozed'       # parked until later, hidden by default
```

Its keys are `project`, `status`, `tag`, `assignee`, `creator`,
`claimed-by`, `priority`, `due`, `due-before`, `due-after`, `parent`, `is`,
`sort`, `limit`, `text`, `title`, `body`, `claimed`, `blocked`, `deleted`,
`starred`, `has` (`has:checklist`), `checklist` (`checklist:open`,
`checklist:done`) and `snoozed`. Snoozed tasks are left out unless you ask:
`is:snoozed` lists only them, `snoozed:any` lists them beside the rest. An
unknown key is exit `2` and the error names the whole set. The other flags add
to whatever `--filter` selected.

## What answers empty and what answers not_found

Exit `3` carries two codes and they mean opposite things for what you do
next. `no_task_available` is "sleep and try again". `not_found` is "the
question was wrong": a project, status, parent, assignee, creator or
claimed-by term naming something this tenant does not have. Both filtering and
claiming fail that way rather than answering empty, because an empty result
with a zero status cannot be told apart from a correct one. Verified:
`tix task ls -p nosuchproject` is `not_found`, exit `3`, where it used to be
`[]` and exit `0`.

Two deliberate exceptions, so don't "fix" them:

- A **tag** never fails a listing. A tag exists only by being applied, so "no
  such tag" and "a tag with no tasks" are the same state.
  `tix task ls -l nosuchtag` is `[]` and exit `0`.
- A **custom field key** still answers empty.
  `tix task ls --filter 'field.nosuchfield:high'` is `[]` and exit `0`, not an
  error. Don't read that as "no task has that value".

A **status** is checked against the union of every workflow in the tenant,
not the workflow of the project you scoped to, and the error names the whole
set:

```console
$ tix claim next -p default -s nosuchstatus -o json -q
error: not_found: no workflow of this tenant defines a status "nosuchstatus"; the defined statuses are blocked, cancelled, doing, done, todo
```

So a status one workflow defines and another does not is a legitimate filter
that may return an empty page; only a status no workflow defines is
`not_found`.

Project keys and statuses are matched without regard to case, on `task ls` and
`claim next` alike: `-p INFRA --status TODO` selects what `-p infra -s todo`
selects. Tags and text terms are matched as written.

## Deadlines

`--overdue` is the shorthand; `--due-before` and `--due-after` take a date.
`due:` says the same thing in an expression, either as a named window —
`overdue`, `today`, `week`, `month` — or bounded with `due:<DATE` and
`due:>DATE`:

```sh
tix task ls --overdue                            # the deadline has passed
tix task ls --filter 'due:overdue -status:done'  # the same, minus finished work
tix task ls --filter 'due:week'                  # falling due in the next seven days
tix task ls --due-after 2026-01-01 -o json       # one end of a window, as a flag
```

Both bounds are inclusive, and both drop a task with no deadline: `--overdue`
never returns an undated task. `--overdue` beside `--due-before` is exit `2`,
because the two name one bound. An unknown window is exit `2` and the error
names the four:

```console
$ tix task ls --filter 'due:fortnight'
error: invalid: unknown due: value "fortnight"; try one of overdue, today, week, month, or a bound such as due:<2026-02-01
```

Set a deadline with `tix task add --due` or `tix task edit --due`.

## Sorting and paging

Default `--sort` is `urgency`: priority first, then soonest `due_at`, undated
tasks last within a priority band. `--sort created_at|updated_at|priority|due_at|title`
still work; an unknown value is exit `2`. `--desc` reverses it.

`--cursor`/`--limit` (default 25) page manually; `--all` does it for you. All
listings are keyset-paginated, so a cursor is stable while rows change under
it.

## Who is who

`--assignee` takes an actor **handle or id**, on `task add`, `task edit` and
`task ls` alike, and so do the `creator:` and `claimed-by:` filter terms. A
handle that names nobody is `not_found`, exit `3`, not an empty list, so a
typo says so instead of reading as "this actor has no work":

```sh
tix actor ls -o json                    # this tenant's directory, agents included
tix actor show raj -o json              # handle or id in, the record out
tix task ls --assignee raj
tix task ls --assignee 01M0Z9JH9Y80C13N5TJ8GQK206
```

The reference is classified by shape, not by asking the directory: a value of
exactly the length and alphabet of a generated id is treated as an id and
passed through with no lookup, so an actor from **another tenant** stays
assignable. Anything else is a handle and must resolve in this tenant.

`actor ls` returns `id`, `kind`, `handle` and `display_name` per row and
`--limit` (default 25) caps it. Agents are listed beside people, so this is
how you find the id behind a handle a human gave you, and who else is
working the queue. An unknown handle or id on `actor show` is exit `3`.

`--actor` on `claim next` and `claim exec` claims **on behalf of** another
actor, which is how a supervisor process takes work for a named agent.

## Reporting progress

`tix stats` reports a window ending now: what was completed and created, how
long work took, where the backlog is sitting, and who closed what. Use it to
report progress instead of counting `task ls` rows yourself.

```sh
tix stats -o json                       # whole tenant, last 14 days
tix stats --window 720h -p infra        # one project, last 30 days
tix stats --since 2026-09-01 --top 10 --oldest 20 -o json
```

- The default window is 14 days. `--window` takes a duration (`30d`, `720h`,
  `16d 1h`), `--since` an RFC3339 timestamp or `YYYY-MM-DD`. Passing both is
  exit `2`; they contradict each other. A window over `87600h` is exit `2`
  too. That duration vocabulary is Go's syntax plus a day of exactly 24
  hours, and it is the same everywhere the CLI, the API or the config file
  takes a duration, so anything the product prints for a reader can be typed
  back. There is no week unit: `4w` is exit `2`.
- `--top` (default 5) sizes the leaderboard and `--oldest` (default 5) the
  list of tasks not yet in a terminal state. Both cap at 100, above which is
  exit `2`.
- `-p` scopes to one project key; an unknown key is exit `3`.
- The leaderboard counts tasks moved to a **terminal state**, nothing else,
  and the JSON carries `leaderboard_measure` saying so. It is not a measure
  of effort, and quoting it as one is a lie about your colleagues.

JSON keys: `since`, `until`, `completed`, `created`, `per_day`,
`median_lead_time`, `slowest_lead_time`, `by_category`, `top_actors`,
`leaderboard_measure`, `oldest`. `by_category` is keyed by the six state
categories, so it is where a backlog in `waiting` shows up as distinct from
one in `blocked`. Durations in the machine formats stay Go's own syntax
(`"94h0m0s"`), so snapshots round-trip; the table renders the same value as
`3d 22h`, which `--window` will read back.

```console
$ tix --db /tmp/scratch.db stats --window 720h
WINDOW             2026-08-26 .. 2026-09-25  (whole tenant)
COMPLETED          36
CREATED            66
MEDIAN LEAD TIME   2d 15h
SLOWEST LEAD TIME  12d 3h
```

## Watching instead of polling

**Do not poll `task ls` in a loop.** Subscribe instead:

```sh
tix watch --type task.created --type task.released -o ndjson --since "$last_seq"
```

Prints one JSON event per line as it happens (`--since` resumes without a
gap, `--limit N` stops after N events, `0` follows forever; with no
`--since` the stream starts at the *next* event, it does not replay
history). On connect it prints one banner line to **stderr** — `watching
local <db> (from flag); ctrl-c to stop` — never stdout, so a piped `-o
ndjson` stays clean; `-q` suppresses it like every other diagnostic.

Default output (no `-o`) is one human-readable line per event, e.g.
`13:53:02  local  created  default-7  watch me` or, for an edit,
`12:54:45  local  updated  default-1  the title and priority` — the payload
names the changed field(s). Filter with `--actor`, `--project`, `--type`
(trailing `*` is a prefix match).

Event types are past tense and are not the audit action names:
`task.created`, `task.claimed`, `task.transitioned`, `task.released`,
`task.lease_expired`, `task.reclaimed`, `project.created`,
`workflow.updated`. For an agent cooperating with others, `task.claimed` and
`task.released` are the two that say whether the queue is being worked.

History does not last for ever: retention prunes old events, so a `--since`
cursor older than the policy resumes from what survives rather than from
where you stopped. See [setup.md](setup.md).

A server started with `tix serve` exposes the same event stream over
WebSocket; see the project's `docs/api.md` if you are integrating outside the
CLI.
