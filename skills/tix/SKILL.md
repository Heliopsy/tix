---
name: tix
description: Drive tix, a task tracker built as one shared queue for humans and AI agents, entirely from its `tix` CLI. Use this whenever a task says to use tix, whenever you need to claim and work a queue of tasks under a lease, or whenever you see `tix` referenced in a repo, CI job, or agent instructions. There is no MCP server; this CLI is the only interface. Covers the claim/lease/release loop, `tix claim exec`, machine-readable output, exit codes, filtering, `tix watch`, comments/artifacts, dependencies, the actor directory, `tix stats`, `tix ssh`, and multi-target config.
version: 5
verified-against: tix 25d0995 (2026-09-25)
---

# tix

One queue, shared by people and agents. Read this once; you should not need
`tix --help` after.

The commit this file was checked against is the `verified-against` field above,
and nowhere else. `cmd/skill_test.go` reads that field and fails if it names a
commit that is not an ancestor of HEAD, or if this file names a command or flag
the CLI does not have. Run `tix version` against the build you are driving; if
it is much newer than that commit, trust `tix <command> --help` where the two
disagree.

## The agent loop

```sh
claim=$(tix claim next -p infra -s todo -o json -q) || exit $?
ref=$(jq -r .task.ref <<<"$claim")
token=$(jq -r .lease_token <<<"$claim")

tix task mv "$ref" doing --lease-token "$token" -q
# ... do the work ...
tix claim release "$ref" --token "$token" --status done \
  --result exit_code=0 --comment "what happened"
```

- `tix claim next` picks the highest-priority unblocked, unclaimed,
  **non-terminal** task matching the filter and claims it atomically. No
  filter still excludes `done`/`cancelled` tasks; it does not exclude other
  non-terminal statuses like `blocked`, so scope with `-s todo` if that
  matters.
- Empty queue (or nothing eligible) is exit `3` with code `no_task_available`,
  not an empty success. Treat it as "nothing to do right now", not an error.
- **Exit `3` is two different answers. Read the code, not just the number.** A
  queue filter naming something this tenant does not have is exit `3` with
  code `not_found`, and sleeping will not fix it: the queue is not empty, the
  question was wrong. `-p nosuchproject` and `-s nosuchstatus` both fail that
  way, before the queue is looked at.
- `-s <typo>` used to answer `no_task_available`, so following the rule above
  literally would have had you idle for ever against a queue full of work.
  It is `not_found` now, checked against the states of every workflow in the
  tenant. If you get `not_found`, fix the filter or stop; do not poll.
- Project keys and statuses are matched without regard to case, so
  `-p INFRA -s TODO` claims what `-p infra -s todo` claims.
- A tag is deliberately **not** checked. A tag exists by being applied, so an
  unknown tag and a tag with no tasks are the same state: `-l nosuchtag` is an
  ordinary empty queue, `no_task_available`, not `not_found`.
- `tix claim task REF` claims one named task instead of the next eligible
  one. Exit `4` if it's already held. Unlike `claim next` it does **not**
  screen for `blocked`: it will hand you a task with an unfinished
  dependency. Check `"blocked"` yourself if you claimed by ref.
- Claiming does **not** transition the task. Move it yourself (`task mv`) or
  use `--on-start` with `claim exec`.

## Leases — read this part

A claim mints a `lease_token` and an absolute `lease_expires_at` (UTC, not a
duration — no clock-drift math). That token is required on every write that
touches the task:

- `tix claim renew REF --token T --ttl 15m` — resets the deadline to
  now+ttl (not "extend by").
- `tix claim release REF --token T [--status S]`
- `tix task mv REF STATUS --lease-token T` — required whenever the task is
  currently claimed.

On `claim renew` and `claim release`, `--token` is the *lease* token and
shadows the global API `--token`. Elsewhere `--token` is the API token.

If you don't renew in time, the lease expires. `tix claim sweep` (run from
cron, or automatically on a ticker inside `tix serve`) reverts any state
whose workflow marks `revert_on_lease_expiry` — the built-in workflow
reverts `doing` → `todo`. A worker that renews on schedule never sees this;
one that stalls, crashes, or is killed just stops renewing and the task
becomes claimable again with no cleanup step.

**If your token goes stale — because you didn't renew in time and someone
else claimed the task — every write with that token fails with exit `4`
and error code `lease_expired`:**

```console
$ tix claim renew default-1 --token stale-token --ttl 15m
error: lease_expired: the lease on task "default-1" is no longer held under that token; claim it again

$ tix task mv default-1 doing --lease-token stale-token
error: lease_expired: the lease on task "default-1" is not held by this token
```

`task mv`'s `lease_expired` only fires when the target status is otherwise a
legal transition from the task's current state. If the transition itself
isn't legal you get the ordinary `precondition_failed`/exit `6` for an
illegal move instead — check the error text, not just that `mv` failed.

**Do not retry the write with a fresh guess at the token, and do not force
it.** The right response to exit `4` is: stop, re-`claim next` or `claim
task`, and re-evaluate — the new holder may already be doing the work, or
the task may need to be re-picked up from scratch.

## `tix claim exec` — the single most useful primitive

Claims, runs a child process while renewing the lease on a ticker, then
releases with the child's exit status recorded as the result. One command
instead of the whole loop above:

```sh
tix claim exec --ref infra-42 --on-start doing -- make test
tix claim exec -p infra -s todo --on-start doing --on-failure blocked -- ./worker.sh
```

| Flag | Effect |
| --- | --- |
| `--ref` | claim this task instead of the next eligible one |
| `--on-start` | move here before running (often required — see below) |
| `--on-success` | release status on exit 0 (default `done`) |
| `--on-failure` | release status on nonzero exit |
| `-p / -s / -l / --ttl / --actor` | same as `claim next` |

It prints `claimed REF until <deadline>` to stderr before starting the
child; `-q` suppresses that.

The release status must be reachable from the *current* status. The
built-in workflow has no `todo`→`done` edge, so skipping `--on-start doing`
means the release fails **after the child already ran** — verified: the
child's side effects land, then

```console
error: invalid: workflow "default" has no transition from "todo" to "done"
```

and the process exits `2`, not the child's status. The task is left claimed
under a lease nobody is renewing, so it comes back through `claim sweep`.
Always set `--on-start` unless you know the workflow allows a direct edge.

Exec's exit codes are not the normal table, because the child's own exit
status is threaded through:

| Exit | Meaning |
| --- | --- |
| `3` | no eligible task — child never started |
| `4` | the lease was lost mid-run |
| `2` | the release was illegal (see above) — the child may already have run |
| anything else | the child's own exit status |

2, 3 and 4 can collide with a real child exit code. If that distinction
matters, claim explicitly with `claim next`/`claim task` instead, or check
whether the child produced its expected output.

If `claim exec` itself is killed, renewal stops with it and the lease just
expires on schedule — no cleanup step needed.

## Machine-readable output

`-o json`, `-o yaml`, `-o ndjson` (default is `table`). `tix task ls`
streams so large listings never buffer. Machine formats **never** carry
colour escapes, even with `--color` forced — verified: `-o json --color`
produces plain bytes, no ANSI. `-q` suppresses diagnostics (not errors —
those still go to stderr with a nonzero exit), and it does not suppress the
command's own result table.

```console
$ tix claim next -p default -s todo -o json -q
{
  "task": {"ref": "default-1", "status": "todo", "priority": 2, ...},
  "lease_token": "M3eGR7ubYpHzHaUJ3nl-weTlTY1DL2q577Nqk2aHp8M",
  "lease_expires_at": "2026-09-22T10:23:38.942529676Z"
}
```

`priority` is numeric: 1 highest … 5 lowest, 3 is `normal`. `--priority
high` on `task add` maps to `2`.

## Exit codes

Same table for every command except `claim exec` (see above). Each
subcommand's `--help` names the codes that command can return; the root
`tix --help` does not print the table.

| Exit | Meaning |
| --- | --- |
| 0 | success |
| 1 | error |
| 2 | usage — invalid flag, bad value, illegal input |
| 3 | not found: a filter term naming nothing, or an empty/no-match queue |
| 4 | conflict — held task, lost lease, version clash |
| 5 | permission denied |
| 6 | precondition failed — illegal transition, task has subtasks |

A dependency cycle is exit `2` (`invalid`), not `6` — verified with `dep add`
closing a loop. Exit `6` is for things a
legal *value* still can't do right now (no edge from the current status, subtasks
blocking a delete), exit `2` is for the value itself being wrong (unknown status
name, self-dependency, a cycle).

Never ignore exit `4`. It means your write did not happen and the state you
think you're in is not the state that's actually there.

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
not the workflow of the project you scoped to. So a status one workflow
defines and another does not is a legitimate filter that may return an empty
page; only a status no workflow defines is `not_found`.

Project keys and statuses are matched without regard to case: `-p INFRA` and
`--status TODO` work.

**A failed listing writes nothing to stdout.** It used to close the document
on the way out, so a failed `-o json` printed `[]` beside its error and a
pipeline reading only stdout read a valid empty answer. If it fails part way
through, the JSON array is left *unterminated* on purpose, so a consumer gets
a syntax error rather than a short listing it would believe. NDJSON and YAML
lines already written stand. Check the exit status; do not infer success from
parseable output.

## Filtering and querying

`task ls` and `claim next` share filter flags; each is a repeatable flag
(pass it more than once, or comma-separate one instance — both work), OR'd
within a flag, AND'd across flags:

```sh
tix task ls -p infra -p ops -s todo -s doing -l ci --unclaimed
tix task ls --query "flaky test"        # substring match over title + body
tix task ls --blocked                   # only tasks blocked by a dependency
tix task ls --sort priority --desc
tix task ls -p infra --all              # follow cursors, read every page
tix project ls -o json                  # the project keys, if you do not know them
```

One `--filter` expression does the same job in a single string, with
negation and weak matching the flags cannot express:

```sh
tix task ls --filter 'status:todo -tag:ops title~deploy'
```

Its keys are `project`, `status`, `tag`, `assignee`, `creator`,
`claimed-by`, `priority`, `due-before`, `due-after`, `parent`, `is`, `sort`,
`limit`, `text`, `title`, `body`, `claimed`, `blocked`, `deleted`. An
unknown key is exit `2` and the error names the whole set.

Default `--sort` is `urgency`: priority first, then soonest `due_at`, undated tasks
last within a priority band. `--sort created_at|updated_at|priority|due_at|title`
still work; an unknown value is exit `2`.

`--cursor`/`--limit` (default 50) page manually; `--all` does it for you.
`task show REF -o json` for one task; `task tree REF` for a task and its
descendants. A fresh database seeds four projects — `default`, `work`,
`homelab`, `house` — so `task ls` with no `-p` spans all of them, while
`task add` with no `-p` lands in `default`.

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
`--limit` (default 50) caps it. Agents are listed beside people, so this is
how you find the id behind a handle a human gave you, and who else is
working the queue. An unknown handle or id on `actor show` is exit `3`.

## Statistics

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
`leaderboard_measure`, `oldest`. Durations in the machine formats stay Go's
own syntax (`"94h0m0s"`), so snapshots round-trip; the table renders the same
value as `3d 22h`, which `--window` will read back.

```console
$ tix --db /tmp/scratch.db stats --window 720h
WINDOW             2026-08-26 .. 2026-09-25  (whole tenant)
COMPLETED          36
CREATED            66
MEDIAN LEAD TIME   2d 15h
SLOWEST LEAD TIME  12d 3h
...
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
(trailing `*` is a prefix match). Event types are past tense and are not
the audit action names: `task.created`, `task.claimed`,
`task.transitioned`, `task.released`, `project.created`,
`workflow.updated`. A server started with `tix serve` also exposes the same
event stream over WebSocket — see the project's `api.md` if you're
integrating outside the CLI.

## Comments and artifacts

Leave a trail a human can read without re-running your work:

```sh
tix comment add default-1 "reproduced: race in TestRetry, fixed with a mutex"
tix artifact put default-1 --kind result --payload '{"tests_run":42,"fixed":true}'
tix artifact put default-1 --kind log --file build.log --content-type text/plain
```

`artifact put --kind` is `result|log|file|metric` (default `result`), and
`--name` labels it on the task. `--payload -` / `--file -` read stdin.
`claim release --comment "..."` and `--result key=value` (repeatable) are
the fast path when you're closing the task anyway — no separate
`comment add` needed.

## Dependencies and subtasks

```sh
tix dep add default-2 default-1     # default-2 waits for default-1 to reach a terminal state
tix task add "child" --parent default-1
```

A task with an unfinished dependency shows `"blocked": true` in its JSON
and `claim next` will not hand it out (`claim task` still will). `dep add`
fails exit `2` on a self-edge or a cycle (`invalid`, not
`precondition_failed`). `task rm` on a task with subtasks fails exit `6`
unless you pass `--cascade`.

`dep add` needs `task:write` (not just `task:claim`/`task:transition`) and
checks authority over *both* ends of the edge: a project-scoped token can
only link two tasks it can both reach, and a cross-project link with such a
token fails exit `5`, not a database error.

## Targets and contexts

Everything resolves the same way whether it's a flag, an env var, or a
named context — so the same script works locally, in CI, and in a
container with no code change:

| Flag | Env var | |
| --- | --- | --- |
| `--db` | `TIX_DATABASE_DSN` | local sqlite path |
| `--server` | `TIX_SERVER_URL` | remote API |
| `--token` | `TIX_TOKEN` (or `TIX_SERVER_TOKEN`) | API token |
| `--tenant` | `TIX_TENANT` | tenant key |
| — | `TIX_PROJECT` | default project |
| `--ctx` | `TIX_CURRENT_CONTEXT` | named context (see `tix ctx --help`) |
| `-o` | `TIX_OUTPUT_FORMAT` | output format |

Precedence, highest first: flag > env var > `.env`/discovered context file >
config file > default. `tix config show -v` prints what actually resolved
and why. `tix doctor` sanity-checks the target before you trust it, and
names the resolved database and where it came from.

Give an agent a scoped token, not a user session:

```sh
tix token create ci-agent --scope task:read --scope task:claim \
  --scope task:transition --scope comment:write
```

The value prints once. `task:read`, `task:claim`, `task:transition`,
`comment:write` is normally all an agent needs; add `task:write` if it
will call `dep add`, `artifact:write` if it records artifacts. `--project`
on `token create` takes either the project's key or its id, same as
`-p`/`--project` everywhere else (`--project infra` works); an unknown key
or id is a clean `not_found`, exit `3`, and an unknown scope is `invalid`,
exit `2`.

## `tix ssh` — the demo listener

`tix ssh` serves the terminal interface (the same thing as `tix tui`) over
SSH. tix is the SSH server itself: no sshd, no system account, no password.

**Today this is a sandbox, not a product.** Any public key is accepted. The
key's fingerprint is the identity, so there is no signup: a new fingerprint
gets its own freshly seeded tenant (`sandbox-<hash>`, "Demo sandbox
SHA256:…"), the same key coming back gets that tenant again, and a tenant
nobody visits for `--tenant-ttl` (default 6h) is deleted with everything in
it. There is no way to enrol a key against a real account from the CLI.

```sh
tix ssh --db /var/lib/tix/demo.db
ssh -p 2222 visitor@localhost
```

The listener faces strangers, so it insists on its own target and refuses
the zero-configuration store:

```console
$ tix ssh
error: invalid: ssh refuses the zero-configuration store, which is somebody's real work: name a database of its own with --db
```

Exit `2` for invalid configuration, `1` for a fatal error. It binds
`127.0.0.1:2222` by default and refuses a non-loopback `--listen` unless you
also pass `--allow-public`. A host key is generated beside the database on first run
unless `--host-key` says otherwise. The rest is capacity and hygiene:
`--max-sessions` (100), `--max-sessions-per-key` (3), `--max-tenants` (200),
`--max-tasks` per sandbox (200), `--lease-ttl` (2m), `--idle-timeout` (30m),
`--keepalive-interval`/`--keepalive-max-missed`, `--rate-per-hour`/
`--rate-burst` per source address, `--reap-interval` (10m). Every one is
also a `TIX_SSH_*` environment variable.

`tix serve` does not host this; it is a separate listener and a separate
command.

## Do not

- Do not poll `task ls` in a loop. Use `tix watch`.
- Do not ignore exit `4`. Re-claim; don't retry the write.
- Do not write to somebody's default store while testing or demoing —
  always pass `--db` (or `TIX_DATABASE_DSN`) at a scratch path. A command
  with no `--db` resolves to the real zero-config store.
- Do not assume a ref (`default-1`) is stable across databases or tenants —
  it's a per-project sequence, not a global ID. If you need a durable
  identifier, use the task's `id`.
- Do not skip `--on-start` on `claim exec` when the workflow has no direct
  edge from the current status to your `--on-success`/`--on-failure`
  target — the release fails after the child already ran.
- Do not assume `claim task REF` screened the task for you. It hands out
  blocked tasks; `claim next` does not.

## Worked example

```console
$ tix --db /tmp/scratch.db task add "flaky test in ci job 412" -p default --priority high --tag ci -q
┌───────────┬──────────────────────────┬────────┬──────────┬──────────┬──────┬─────┬──────────────────┬─────────┐
│ REF       │ TITLE                    │ STATUS │ PRIORITY │ ASSIGNEE │ TAGS │ DUE │ UPDATED          │ BLOCKED │
├───────────┼──────────────────────────┼────────┼──────────┼──────────┼──────┼─────┼──────────────────┼─────────┤
│ default-1 │ flaky test in ci job 412 │ todo   │ high     │          │ ci   │     │ 2026-09-22 12:53 │ no      │
└───────────┴──────────────────────────┴────────┴──────────┴──────────┴──────┴─────┴──────────────────┴─────────┘

$ claim=$(tix --db /tmp/scratch.db claim next -p default -s todo -o json -q)
$ ref=$(jq -r .task.ref <<<"$claim")        # default-1
$ token=$(jq -r .lease_token <<<"$claim")

$ tix --db /tmp/scratch.db task mv "$ref" doing --lease-token "$token" -q

$ tix --db /tmp/scratch.db comment add "$ref" "reproduced: race in TestRetry, fixed with a mutex" -q
$ tix --db /tmp/scratch.db artifact put "$ref" --kind result --payload '{"tests_run":42,"fixed":true}' -q

$ tix --db /tmp/scratch.db claim release "$ref" --token "$token" --status done \
    --result exit_code=0 --comment "patched and merged" -o json
{
  "ref": "default-1",
  "status": "ok"
}
```

Every command above was run against a scratch SQLite database while writing
this skill; output shown is real, trimmed only where noted.
