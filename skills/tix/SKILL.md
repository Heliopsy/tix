---
name: tix
description: Drive tix, a task tracker built as one shared queue for humans and AI agents, entirely from its `tix` CLI. Use this whenever a task says to use tix, whenever you need durable task state that survives your own process dying, whenever you need to claim and work a queue of tasks under a lease, or whenever you see `tix` referenced in a repo, CI job, or agent instructions. There is no MCP server; this CLI is the only interface. Covers writing your own plan down as tasks before starting, the claim/lease/release loop, `tix claim exec`, crash recovery, exit codes, machine-readable output, comments and artifacts, dependencies and subtasks, and `tix status`.
version: 12
verified-against: tix 425cd60 (2026-10-26)
---

# tix

One queue, shared by people and agents. This file is the whole hot path: the
loop you run constantly. Four reference files sit beside it for the things you
need once in a while:

- [reference/queries.md](reference/queries.md) — the filter language, deadlines,
  sorting and paging, the actor directory, `tix stats`, `tix watch`.
- [reference/setup.md](reference/setup.md) — targets and contexts, API tokens
  and scopes, `tix serve`, workflows and custom fields, the audit log,
  import/export, sync, webhooks, bundles, retention.
- [reference/ssh.md](reference/ssh.md) — serving the terminal interface over
  SSH, and enrolling keys.
- [reference/recovery.md](reference/recovery.md) — the worked crash transcript
  this skill's claims were verified against.

The commit this file was checked against is the `verified-against` field above,
and nowhere else. `cmd/skill_test.go` reads that field and fails if it names a
commit that is not an ancestor of HEAD, or if any file in this skill names a
command or flag the CLI does not have. Run `tix version` against the build you
are driving; if it is much newer than that commit, trust `tix <command> --help`
where the two disagree.

## Write your plan down before you start

This is the point of using tix as an agent, and it is the step you are most
likely to skip.

If you are handed an assignment, **decompose it into tix tasks first, then
work them one at a time.** A plan that lives only in your context is lost
completely when your process dies. A plan recorded as tasks costs you one
task's work, because every other task is still sitting in the queue where you
or another agent can pick it up.

```sh
# 1. the assignment, as one parent task
tix task add "ship the retry fix" -p default --priority high -q -o json

# 2. the steps, as subtasks of it, with the ordering written down
tix task add "reproduce the flake" --parent default-1 -q
tix task add "patch and test" --parent default-1 --depends-on default-2 -q
tix task add "update the changelog" --parent default-1 --depends-on default-3 -q

# 3. read back what you committed to
tix task tree default-1
```

**Subtasks and dependencies are deliberately different things, and you want
both.**

- `--parent` is **containment**: this work belongs to that work. It groups the
  plan so a reader (or your replacement) sees the shape of it, and it stops the
  parent being deleted out from under the children — `task rm` on a task with
  subtasks is exit `6` unless you pass `--cascade`.
- `--depends-on` is **ordering**: this cannot start until that is finished. It
  is what the queue enforces. A task with an unfinished dependency reports
  `"blocked": true` and `tix claim next` will not hand it out at all.

So `--parent` is for whoever reads the plan; `--depends-on` is for whoever
works it. A plan with only parents lets you claim step three before step one.

Write the steps at the granularity you are willing to lose. One task per unit
of work you could redo from its description alone.

Then take them one at a time, leaving a trail as you go:

```sh
tix comment add default-2 "reproduced on the third run; it is a race in TestRetry"
tix artifact put default-2 --kind log --file ./test-output.txt --content-type text/plain
```

(Once you have *claimed* default-2, `artifact put` also needs
`--lease-token "$token"` — see "Leases" below. `comment add` never does.)

When you discover work you did not plan for, add it as a task rather than
doing it silently. That is the same durability argument: an unrecorded detour
is lost with your context.

## The loop

```sh
claim=$(tix claim next -p default -s todo -o json -q) || exit $?
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
  matters — but read the trap under "Crash recovery" before you rely on
  `-s todo` to drain a queue.
- Empty queue (or nothing eligible) is exit `3` with code `no_task_available`,
  not an empty success. Treat it as "nothing to do right now", not an error.
- **Exit `3` is two different answers. Read the code, not just the number.** A
  queue filter naming something this tenant does not have is exit `3` with
  code `not_found`, and sleeping will not fix it: the queue is not empty, the
  question was wrong. `-p nosuchproject` and `-s nosuchstatus` both fail that
  way, before the queue is looked at. If you get `not_found`, fix the filter
  or stop; do not poll. A tag is the one exception: `-l nosuchtag` is an
  ordinary empty queue, not `not_found`.
  [reference/queries.md](reference/queries.md) has the rest, including which
  terms are matched without regard to case.
- `tix claim task REF` claims one named task instead of the next eligible
  one. Exit `4` if it's already held. Unlike `claim next` it does **not**
  screen for `blocked`: it will hand you a task with an unfinished
  dependency. Check `"blocked"` yourself if you claimed by ref.
- Claiming does **not** transition the task. Move it yourself (`task mv`) or
  use `--on-start` with `claim exec`.
- There is no `tix claim` subcommand that lists leases. To see what is
  currently held — which is how you tell whether another agent is already on
  the queue — ask the task listing: `tix task ls --filter 'claimed:yes'`.

## Leases

A claim mints a `lease_token` and an absolute `lease_expires_at` (UTC, not a
duration — no clock-drift math). That token is required on every write to the
task's **status or its results**:

- `tix claim renew REF --token T --ttl 15m` — resets the deadline to
  now+ttl (not "extend by").
- `tix claim release REF --token T [--status S]`
- `tix task mv REF STATUS --lease-token T` — required whenever the task is
  currently claimed.
- `tix artifact put REF --lease-token T` — required whenever the task is
  currently claimed. `claim release --result` and `claim exec` already carry
  results under the token, so reach for this one when you are recording
  something mid-run rather than at the end.

**Omit `--ttl` unless the work is genuinely long.** Without one the length
comes from the project's workflow, then the tenant, then the deployment. A ttl
you do pass is capped at whatever the operator set, 24h at most, and over that
is refused — exit `2`, nothing renewed, old deadline still running:

```console
$ tix claim renew infra-8 --token "$token" --ttl 48h
error: invalid: lease ttl 48h0m0s exceeds the maximum of 24h0m0s
```

Renew again with a shorter ttl, not the same one.

**Keep the token in a variable for the whole run.** Both `task mv` and
`artifact put` need it, and `claim` is the only thing that ever tells you what
it is.

On `claim renew` and `claim release`, `--token` is the *lease* token and
shadows the global API `--token`. Elsewhere `--token` is the API token.

**If your token goes stale — because you didn't renew in time, or someone
reclaimed the task, or someone else claimed it — every such write with that
token fails with exit `4` and error code `lease_expired`:**

```console
$ tix claim renew default-1 --token stale-token --ttl 15m
error: lease_expired: the lease on task "default-1" is no longer held under that token; claim it again

$ tix task mv default-2 done --lease-token agent-a-stale-token
error: lease_expired: the lease on task "default-2" is not held by this token
```

`artifact put` fails the same way with the same message.

Dropping the token is not a way round it. On a task that is still claimed,
omitting it is refused too, so there is no tokenless back door to the result
list:

```console
$ tix artifact put default-2 --kind result --payload '{"stale":true}'
error: lease_expired: task "default-2" is claimed; supply the lease token
```

`task mv`'s `lease_expired` only fires when the target status is otherwise a
legal transition from the task's current state. If the transition itself
isn't legal you get the ordinary `precondition_failed`/exit `6` for an
illegal move instead — check the error text, not just that `mv` failed.

**Do not retry the write with a fresh guess at the token, and do not force
it.** The right response to exit `4` is: stop, re-`claim next` or `claim
task`, and re-evaluate — the new holder may already be doing the work, or
the task may need to be re-picked up from scratch.

### What the lease covers, and what it deliberately does not

Your **results are yours while you hold the task.** An agent whose lease
expired without noticing can neither move your task nor write over your
result, with its dead token or without one, so you do not have to defend
against it.

Two writes deliberately take **no** token, on a claimed task or any other. Do
not pass one and do not expect a refusal:

- **`tix comment add`.** Comments are discussion, and append-only — nothing
  replaces another. This is how a human talks to you about work you are
  holding, so it has to keep working; it is also the only channel someone in
  the browser has on a task you hold, since a browser session holds no lease.
- **`tix task edit`.** Field edits are shared triage: a human re-prioritising
  or re-assigning live work is the queue doing its job. If you care about a
  concurrent edit, that is what `--version` is for, not the lease.

So treat a **comment** as evidence, never a lock: two agents can write
contradictory ones. The **artifact** is what the lease protects, so keep your
token to hand.

Still prefer `claim release --comment` and `--result key=value` when you are
closing the task anyway: one lease-guarded write instead of three, and no token
to pass by hand.

## Crash recovery — why this is safe

Expiry is lazy. There is no sweeper in the path and nothing has to be running:
once `lease_expires_at` has passed, the queue reads the row as unclaimed, so a
crashed agent's task becomes claimable again on its own. Kill `-9` an agent
mid-work and another `claim next` picks the task up, with no cleanup step.

That is the whole durability argument, and it is why the unit of loss is one
task rather than a plan. [reference/recovery.md](reference/recovery.md) has
the transcript.

**The trap: a crashed task is claimable again but its status has not moved.**
The row stays in `doing`. So a worker draining the queue with
`claim next -s todo` will **never see it again** — it answers
`no_task_available`, exit `3`, while the task sits there claimable in `doing`.
Verified against a killed agent.

`tix claim sweep` is what puts the status back: it reverts any state whose
workflow marks `revert_on_lease_expiry`, and the built-in workflow reverts
`doing` → `todo`. It runs from cron, or on a ticker inside `tix serve`. So:

- If something runs the sweep, `-s todo` is a safe filter.
- If nothing does, do not scope by status at all, or sweep yourself first:
  `tix claim sweep --dry-run` reports what it would revert, `tix claim sweep`
  does it, and `--limit` caps one run.
- `tix status` tells you which world you are in: it reports how many servers
  are attached and how many **leases have expired and are unswept**. A nonzero
  count there with no server attached means nothing is sweeping.

An operator holding `task:reclaim` can end a *live* lease without its token,
for a worker that died without releasing:

```sh
tix claim reclaim REF --reason 'agent host died'
```

That frees the lease and leaves the status alone: it does not apply
`revert_on_lease_expiry` and it does not reopen anything, because reopening
work is `tix task mv`, made as its own decision. A task holding no live lease
is `conflict`, exit `4` — a finished task cannot be reclaimed, and neither can
one whose lease already lapsed, since there is nothing left to take.
`task:reclaim` is a scope of its own and is not implied by `task:claim`.

## `tix claim exec` — the loop in one command

Claims, runs a child process while renewing the lease on a ticker, then
releases with the child's exit status recorded as the result. Prefer it
whenever the work is a command, because the renewal is handled for you and a
kill takes the renewal with it:

```sh
tix claim exec --ref default-2 --on-start doing -- make test
tix claim exec -p default -s todo --on-start doing --on-failure blocked -- ./worker.sh
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

Both the start and the release status must be reachable from the status the
task is in at the time. The built-in workflow has no `todo`→`done` edge, so
skipping `--on-start doing` means the release fails **after the child already
ran** — the child's side effects land, then

```console
error: invalid: workflow "default" has no transition from "todo" to "done"
```

and the process exits `2`, not the child's status. The task is left claimed
under a lease nobody is renewing, so it comes back when the lease lapses.
Always set `--on-start` unless you know the workflow allows a direct edge.

Equally, do not point `--on-start` at the status the task is already in. That
is an illegal self-transition, and exec claims the task, fails the move and
stops **before** the child runs:

```console
$ tix claim exec --ref default-2 --on-start doing -- true
claimed default-2 until 2026-10-11T13:40:40Z
error: precondition_failed: cannot move task "default-2" from "doing" to "doing"; allowed targets are todo, blocked, done, cancelled
```

That is exit `6`, which is not in exec's table below, so a child exiting `6`
and a refused `--on-start` look alike — read the message.

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

## Statuses, categories and terminal states

Do not hard-code the five built-in statuses. A tenant can define any workflow,
so **read the states before you move a task**:

```sh
tix workflow get default -o json      # states, transitions, and default_lease
```

Every state carries a `category`, and there are six: `todo`, `in_progress`,
`blocked`, `waiting`, `done`, `cancelled`. The category is how you reason
about a status you have never seen — a state in `waiting` is parked on
something outside the queue, so it is not work you can push forward by
claiming it, and a state in `done` or `cancelled` is `terminal`, which is what
releases a dependency.

The built-in workflow's terminal states are `done` and `cancelled`. **Use
`cancelled` when you abandon work**, rather than leaving the task in `doing`
or marking it `done`: it is terminal, so dependants unblock, and it records
that the work did not happen.

`tix task mv REF STATUS --hops` reaches a status the workflow only gets to
through another state: it prints the route, then applies one ordinary
transition per hop, so the audit trail records every state the task passed
through. With `--dry-run` it prints the route and writes nothing. When two
routes are equally short it refuses rather than guessing, and asks you to name
one, written `doing>done`.

## When the work needs a decision you are not entitled to make

Some tasks cannot be finished by trying harder. The requirement is genuinely
ambiguous, or the choice belongs to somebody with authority you do not have:
what the retention period should be, which of two behaviours is wanted, whether
an external service may be called at all.

**Do not guess, and do not stall.** Guessing produces work that has to be
redone and a `done` that is not true. Stalling holds a lease on a task nobody
else can take, until it expires and another agent repeats your confusion.

Park it, ask, and release it:

```sh
tix comment add REF "Blocked: should an expired link return 404 or 410? \
I can implement either, and the choice changes the client contract."
tix claim release REF --token "$LEASE" --status blocked
```

The comment is the question and belongs on the task, where whoever answers is
already looking. The status change is what takes the task out of the queue, so
`claim next` stops offering it and no other agent spends a second on it.

Three things make this the right move rather than a failure:

- The question is durable. It outlives your process, so it is still there when
  somebody reads the board tomorrow.
- The task is visible as blocked, which is a count on the board rather than
  silence.
- Answering is one comment and one `tix task mv REF todo`, and the work is back
  in the queue for whoever is free, not for you specifically.

Use a state in the `blocked` category for a decision you are waiting on, and
one in `waiting` for something outside the queue that will arrive on its own,
such as a build or a third party. Both keep the task out of `claim next`; they
say different things to the person reading the board.

If the whole task turns out to be wrong rather than merely undecided, use
`cancelled` with a comment saying why. It is terminal, so dependants unblock.

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
| 4 | conflict — held task, lost lease, version clash, duplicate name |
| 5 | permission denied |
| 6 | precondition failed — illegal transition, task has subtasks |
| 7 | upstream failed — a system tix called, such as a sync source, answered badly |

Exit `6` is for things a legal *value* still can't do right now (no edge from
the current status, subtasks blocking a delete); exit `2` is for the value
itself being wrong (unknown status name, self-dependency, a cycle). A
dependency cycle is exit `2` (`invalid`), not `6`.

Never ignore exit `4`. It means your write did not happen and the state you
think you're in is not the state that's actually there.

## Machine-readable output

`-o json`, `-o yaml`, `-o ndjson` (default is `table`). `tix task ls`
streams so large listings never buffer. Machine formats **never** carry
colour escapes, even with `--color` forced. `-q` suppresses diagnostics (not
errors — those still go to stderr with a nonzero exit), and it does not
suppress the command's own result table.

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

**A failed listing writes nothing usable to stdout:** a listing that fails
part way through leaves its JSON array *unterminated* on purpose, so a
consumer gets a syntax error rather than a short listing it would believe.
Check the exit status; never infer success from parseable output.

Reading one task or a few:

```sh
tix task show default-1 -o json      # one task
tix task tree default-1              # a task and its descendants
tix task ls -s todo --unclaimed -o json
tix task ls --blocked                # only tasks blocked by a dependency
tix task ls --starred                # only tasks you starred
tix project ls -o json               # the project keys, if you do not know them
```

A fresh database seeds four projects — `default`, `work`, `homelab`, `house` —
so `task ls` with no `-p` spans all of them, while `task add` with no `-p`
lands in `default`. `--limit` (default 25) sizes a page, `--cursor` continues
one, `--all` follows them for you. Tasks the caller starred (`tix task star
REF`) list first under every sort; a star is per actor, so another worker's
stars never reorder your queue, and a token that stars nothing sees the plain
order. The whole filter language, deadlines and sorting are in
[reference/queries.md](reference/queries.md).

**A recurring task makes its own successor.** A task whose JSON carries
`recurrence` (set with `tix task add --repeat 'every week'`, changed with
`tix task edit --repeat` or removed with `--no-repeat`) creates the next
occurrence itself when you move it to a done state, in the same write, and
records it as `recurrence_next_id`. Do not create the next one yourself, or
there will be two. Moving it to `cancelled` ends the series instead.

**Do not poll `tix task ls` in a loop.** `tix watch` subscribes to the event
stream instead; see [reference/queries.md](reference/queries.md).

## Comments, artifacts and results

Leave a trail a human can read without re-running your work:

```sh
tix comment add default-1 "reproduced: race in TestRetry, fixed with a mutex"
tix artifact put default-1 --kind result --payload '{"tests_run":42,"fixed":true}' --lease-token "$token"
tix artifact put default-1 --kind log --file build.log --content-type text/plain --lease-token "$token"
```

`artifact put --kind` is `result|log|file|metric` (default `result`), and
`--name` labels it on the task. `--payload -` / `--file -` read stdin.

**`artifact put` needs `--lease-token` while you hold the task; `comment add`
never does** — see "What the lease covers" above. Drop `--lease-token` only
when the task is genuinely unclaimed; on a claimed one it is exit `4`.
`claim release --comment "..."` and `--result key=value` (repeatable) are the
fast path when you're closing the task anyway, and need no token of their own.

Every mutating command takes `--dry-run`, which reports what it would do and
writes nothing. Use it when you are unsure a transition or a delete is legal.

## Dependencies

```sh
tix dep add default-3 default-2     # default-3 waits for default-2 to finish
tix dep ls default-3
tix dep rm default-3 default-2
```

A dependency is released when the task it points at reaches a **terminal**
state — `done` or `cancelled` in the built-in workflow, not merely "moved on".
`dep add` fails exit `2` on a self-edge or a cycle (`invalid`, not
`precondition_failed`).

`dep add` needs `task:write` (not just `task:claim`/`task:transition`) and
checks authority over *both* ends of the edge: a project-scoped token can only
link two tasks it can both reach, and a cross-project link with such a token
fails exit `5`, not a database error. `task add --depends-on` is the same
edge, recorded as the task is created, which is what the planning section
uses.

## Know which store you are writing to

```sh
tix status                 # what this installation holds, and what is serving it
tix doctor                 # whether the target is sound, before you trust it
tix config show -v         # what resolved, and why
```

A command with no `--db` and no configured server resolves to the
zero-configuration store at `$XDG_DATA_HOME/tix/tix.db`, **which is somebody's
real work.** The absence of the flag is the bug. For anything experimental,
pass `--db` at a scratch path, and prefer a fresh tenant over reusing
`default`.

Targets, contexts, tokens and scopes are in
[reference/setup.md](reference/setup.md).

## Do not

- Do not start work before recording the plan. A crash then costs the plan,
  not one task.
- Do not trust a comment or an artifact as proof that you still hold a task.
  They are not lease-guarded. Renew, or close through `claim release`.
- Do not drain a queue with `claim next -s todo` unless something is running
  `claim sweep`. A crashed task stays in `doing` and that filter will never
  show it again.
- Do not poll `task ls` in a loop. Use `tix watch`.
- Do not ignore exit `4`. Re-claim; don't retry the write.
- Do not sleep on exit `3` without reading the code. `no_task_available` means
  wait; `not_found` means your filter is wrong and waiting is forever.
- Do not write to somebody's default store while testing or demoing — always
  pass `--db` (or `TIX_DATABASE_DSN`) at a scratch path.
- Do not assume a ref (`default-1`) is stable across databases or tenants —
  it's a per-project sequence, not a global ID. If you need a durable
  identifier, use the task's `id`.
- Do not skip `--on-start` on `claim exec` when the workflow has no direct
  edge from the current status to your `--on-success`/`--on-failure` target —
  the release fails after the child already ran.
- Do not assume `claim task REF` screened the task for you. It hands out
  blocked tasks; `claim next` does not.
- Do not leave abandoned work in `doing`. Move it to `cancelled`, which is
  terminal, so whatever depends on it stops waiting.

## A full transcript

[reference/recovery.md](reference/recovery.md) walks one assignment from plan
to finish, through a crash and a second agent, with real output throughout.
Every command in this skill was run against a scratch SQLite database while
writing it.
