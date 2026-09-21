---
name: tix
description: Drive tix, a task tracker built as one shared queue for humans and AI agents, entirely from its `tix` CLI. Use this whenever a task says to use tix, whenever you need to claim and work a queue of tasks under a lease, or whenever you see `tix` referenced in a repo, CI job, or agent instructions. There is no MCP server; this CLI is the only interface. Covers the claim/lease/release loop, `tix claim exec`, machine-readable output, exit codes, filtering, `tix watch`, comments/artifacts, dependencies, and multi-target config.
---

# tix

One queue, shared by people and agents. Read this once; you should not need
`tix --help` after.

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
- Empty queue (or nothing eligible) is exit `3`, not an empty success. Treat
  it as "nothing to do right now", not an error.
- `tix claim task REF` claims one named task instead of the next eligible
  one. Exit `4` if it's already held.
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

$ tix task mv default-1 done --lease-token stale-token
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

The release status must be reachable from the *current* status. The
built-in workflow has no `todo`→`done` edge, so skipping `--on-start doing`
means the release fails **after the child already ran**. Always set it
unless you know the workflow allows a direct edge.

Exec's exit codes are not the normal table, because the child's own exit
status is threaded through:

| Exit | Meaning |
| --- | --- |
| `3` | no eligible task — child never started |
| `4` | the lease was lost mid-run |
| anything else | the child's own exit status |

3 and 4 can collide with a real child exit code. If that distinction
matters, claim explicitly with `claim next`/`claim task` instead, or check
whether the child produced its expected output.

If `claim exec` itself is killed, renewal stops with it and the lease just
expires on schedule — no cleanup step needed.

## Machine-readable output

`-o json`, `-o yaml`, `-o ndjson` (default is `table`). `tix task ls`
streams so large listings never buffer. Machine formats **never** carry
colour escapes, even with `--color` forced — verified: piping `-o json
--color` through the terminal produces plain bytes, no ANSI. `-q` suppresses
diagnostics (not errors — those still go to stderr with a nonzero exit).

```console
$ tix claim next -p default -s todo -o json -q
{
  "task": {"ref": "default-1", "status": "todo", "priority": 2, ...},
  "lease_token": "1OKVvsZlacH-KaBNorYe86ZsxwFnCN45k1GhBB0ylIs",
  "lease_expires_at": "2026-09-21T11:25:11.271317441Z"
}
```

`priority` is numeric: 1 highest … 5 lowest, 3 is `normal`. `--priority
high` on `task add` maps to `2`.

## Exit codes

Same table for every command except `claim exec` (see above), printed by
`tix --help` too:

| Exit | Meaning |
| --- | --- |
| 0 | success |
| 1 | error |
| 2 | usage — invalid flag, bad value, illegal input |
| 3 | not found — includes an empty/no-match queue |
| 4 | conflict — held task, lost lease, version clash |
| 5 | permission denied |
| 6 | precondition failed — dependency cycle, illegal transition, task has subtasks |

Never ignore exit `4`. It means your write did not happen and the state you
think you're in is not the state that's actually there.

## Filtering and querying

`task ls` and `claim next` share filter flags; each is a repeatable flag
(pass it more than once, or comma-separate one instance — both work), OR'd
within a flag, AND'd across flags:

```sh
tix task ls -p infra -p ops -s todo -s doing -l ci --unclaimed
tix task ls --query "flaky test"        # substring match over title + body
tix task ls --blocked                   # only tasks blocked by a dependency
tix task ls --assignee alice --sort priority --desc
tix task ls -p infra --all              # follow cursors, read every page
```

`--cursor`/`--limit` (default 50) page manually; `--all` does it for you.
`task show REF -o json` for one task; `task tree REF` for a task and its
descendants.

## Watching instead of polling

**Do not poll `task ls` in a loop.** Subscribe instead:

```sh
tix watch --type task.created --type task.released -o ndjson --since "$last_seq"
```

Prints one JSON event per line as it happens (`--since` resumes without a
gap, `--limit N` stops after N events, `0` follows forever). Default output
(no `-o`) is one human-readable line: `13:53:02  local  created  default-7
watch me`. Filter with `--actor`, `--project`, `--type` (trailing `*` is a
prefix match). A server started with `tix serve` also exposes the same
event stream over WebSocket — see the project's `api.md` if you're
integrating outside the CLI.

## Comments and artifacts

Leave a trail a human can read without re-running your work:

```sh
tix comment add default-1 "reproduced: race in TestRetry, fixed with a mutex"
tix artifact put default-1 --kind result --payload '{"tests_run":42,"fixed":true}'
tix artifact put default-1 --kind log --file build.log --content-type text/plain
```

`artifact put --kind` is `result|log|file|metric` (default `result`).
`--payload -` / `--file -` read stdin. `claim release --comment "..."`
and `--result key=value` (repeatable) are the fast path when you're closing
the task anyway — no separate `comment add` needed.

## Dependencies and subtasks

```sh
tix dep add default-2 default-1     # default-2 waits for default-1 to reach a terminal state
tix task add "child" --parent default-1
```

A task with an unfinished dependency shows `"blocked": true` in its JSON
and `claim next`/`claim task` will not hand it out. `dep add` fails exit
`6` on a cycle. `task rm` on a task with subtasks fails exit `6` unless you
pass `--cascade`.

## Targets and contexts

Everything resolves the same way whether it's a flag, an env var, or a
named context — so the same script works locally, in CI, and in a
container with no code change:

| Flag | Env var | |
| --- | --- | --- |
| `--db` | `TIX_DATABASE_DSN` | local sqlite path |
| `--server` | `TIX_SERVER_URL` | remote API |
| `--token` | `TIX_TOKEN` | API token |
| `--tenant` | `TIX_TENANT` | tenant key |
| — | `TIX_PROJECT` | default project |
| `--ctx` | `TIX_CURRENT_CONTEXT` | named context (see `tix ctx --help`) |
| `-o` | `TIX_OUTPUT_FORMAT` | output format |

Precedence, highest first: flag > env var > `.env`/discovered context file >
config file > default. `tix config show -v` prints what actually resolved
and why. `tix doctor` sanity-checks the target before you trust it.

Give an agent a scoped token, not a user session:

```sh
tix token create ci-agent --scope task:read --scope task:claim \
  --scope task:transition --scope comment:write
```

The value prints once. `task:read`, `task:claim`, `task:transition`,
`comment:write` is normally all an agent needs; `artifact:write` if it
records artifacts too. To scope the token to one project, `--project` wants
the project's **id**, not its key — unlike `-p`/`--project` everywhere
else in the CLI, `token create --project` does not resolve a key and fails
with a foreign-key error if you pass one:
`--project "$(tix project ls -o json | jq -r '.[]|select(.key=="infra").id')"`.

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

## Worked example

```console
$ tix --db /tmp/scratch.db task add "flaky test in ci job 412" -p default --priority high --tag ci -q
┌───────────┬──────────────────────────┬────────┬──────────┬──────────┬──────┬─────┬──────────────────┬─────────┐
│ REF       │ TITLE                    │ STATUS │ PRIORITY │ ASSIGNEE │ TAGS │ DUE │ UPDATED          │ BLOCKED │
├───────────┼──────────────────────────┼────────┼──────────┼──────────┼──────┼─────┼──────────────────┼─────────┤
│ default-1 │ flaky test in ci job 412 │ todo   │ high     │          │ ci   │     │ 2026-09-21 10:57 │ no      │
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
