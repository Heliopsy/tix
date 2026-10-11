# The crash, demonstrated

SKILL.md claims that an agent can crash mid-task without losing its plan, and
that recovery needs nothing to be running. This is the run those claims were
checked against: one agent writes a plan down, claims a task, is killed with
`SIGKILL`, and a second agent picks the task up and finishes it.

Read this if you want to know exactly what survives a crash and what does not,
or if you are about to rely on the lease and want to see its edges first.
Everything below is real output from a scratch SQLite database, trimmed only
where marked.

## 1. the plan, written down before any work starts

```sh
tix task add "ship the retry fix" -p default --priority high -q -o json
tix task add "reproduce the flake" --parent default-1 -q -o json
tix task add "patch and test" --parent default-1 --depends-on default-2 -q -o json
```

```console
$ tix task tree default-1
┌───────────┬─────────────────────┬────────┬──────────┬──────────┬──────┬─────┬──────────────────┬─────────┐
│ REF       │ TITLE               │ STATUS │ PRIORITY │ ASSIGNEE │ TAGS │ DUE │ UPDATED          │ BLOCKED │
├───────────┼─────────────────────┼────────┼──────────┼──────────┼──────┼─────┼──────────────────┼─────────┤
│ default-1 │ ship the retry fix  │ todo   │ high     │          │      │     │ 2026-10-11 15:43 │ no      │
│ default-2 │ reproduce the flake │ todo   │ normal   │          │      │     │ 2026-10-11 15:43 │ no      │
│ default-3 │ patch and test      │ todo   │ normal   │          │      │     │ 2026-10-11 15:43 │ yes     │
└───────────┴─────────────────────┴────────┴──────────┴──────────┴──────┴─────┴──────────────────┴─────────┘
```

`default-3` is `blocked` because of `--depends-on default-2`, so `claim next`
will not hand it out before its dependency finishes. That is the ordering
surviving outside anybody's context.

## 2. agent A claims a task and is killed

A two-second lease, so the demonstration does not take half an hour.

```sh
tix claim exec --ref default-2 --ttl 5s --on-start doing -- sleep 300 &
```

```console
claimed default-2 until 2026-10-11T12:43:20Z
```

While it is alive the row is held:

```console
$ tix task show default-2 -o json
  "status": "doing",
  "claimed_by_actor_id": "01M4NFT9CFSZF6GW58ZVTXBGFM",
  "claimed_at": "2026-10-11T12:43:15.908371792Z",
  "lease_expires_at": "2026-10-11T12:43:22.580966Z",
  "claim_count": 1,
```

and nothing else can take it:

```console
$ tix task ls --unclaimed -s doing -o json -q
[]
```

Then `kill -9` — a crash, so no release, no final comment, no further
renewal. The shell reports exit 137.

Immediately afterwards the lease is still live, so the task is still held:

```console
$ tix claim task default-2 -o json -q
error: conflict: task "default-2" is already held by this worker; renew the lease instead
```

That is exit `4`. (The message says "this worker" because the second claim ran
as the same local actor; a different actor is refused just as firmly.)

## 3. the lease lapses on its own

Wait past the deadline. **No sweeper. No cron. No `tix serve`. Nothing
running.** The stored row is byte-for-byte unchanged — still `doing`, still
carrying agent A's `claimed_by_actor_id` and its expired `lease_expires_at`:

```console
$ tix task show default-2 -o json
  "status": "doing",
  "claimed_by_actor_id": "01M4NFT9CFSZF6GW58ZVTXBGFM",
  "claimed_at": "2026-10-11T12:43:15.908371792Z",
  "lease_expires_at": "2026-10-11T12:43:22.580966Z",
  "claim_count": 1,
```

But the queue reads a lapsed lease as unclaimed:

```console
$ tix task ls --unclaimed -s doing -o json -q
[  {"ref":"default-2", ... }  ]
```

This is the whole mechanism. Expiry is a comparison against the deadline at
read time, not a job that has to run. **Nothing had to notice agent A died.**

**And this is the trap.** The status did not change. A worker draining the
queue with `claim next -p default -s todo` gets:

```console
error: no_task_available: no eligible task is available to claim
```

exit `3`, for ever, while the task sits there claimable in `doing`. `tix claim
sweep` is what reverts it, because the built-in workflow marks `doing` with
`revert_on_lease_expiry`:

```console
$ tix claim sweep
┌───────┐
│ SWEPT │
├───────┤
│ 1     │
└───────┘
```

After which the task is back in `todo` with its claim cleared, and `-s todo`
finds it again. Either run the sweep, or do not scope your claim by status.

## 4. agent B takes over

```console
$ tix claim task default-2 -o json -q
   [exit 0]
```

Agent B now holds the task under a fresh token, and finds it exactly where
agent A left it: same status, same comments, same artifacts. The unit of loss
was whatever agent A had in flight on this one task — not the plan, and not
the other two tasks.

Agent A's token is dead:

```console
$ tix task mv default-2 done --lease-token agent-a-stale-token
error: lease_expired: the lease on task "default-2" is not held by this token
```

exit `4`.

## 5. where the safety stops

Agent A, if it were still running, could not move the task. It **can** still
write to it, with no token at all:

```console
$ tix comment add default-2 "agent A, still running, overwriting B's conclusions"
  "body": "agent A, still running, overwriting B's conclusions",
   [exit 0]

$ tix artifact put default-2 --kind result --payload '{"zombie":true}' -q -o json
  "kind": "result",
  "payload": {
    "zombie": true
  },
   [exit 0]
```

Both exit `0`. The lease token lives on the transition input alone, so it
guards transitions and nothing else. Comments and artifacts are not checked
against it, which means a zombie agent can write a result over the live
holder's work. See "What the lease does not protect" in SKILL.md for what to
do about it.

## 6. the plan carries on

With `default-2` terminal, its dependant unblocks on its own:

```console
$ tix claim release default-2 --token "$token" --status done --result exit_code=0 --comment "done by B"
{
  "ref": "default-2",
  "status": "ok"
}

$ tix task show default-3 -o json
   default-3 blocked = False
```

`default-3` was blocked before, is claimable now, and nobody had to remember
that it was waiting. That is what writing the plan down bought.
