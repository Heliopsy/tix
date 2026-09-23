# Agents

How an autonomous worker uses tix. Everything here is reachable from a shell, so it works the same from a Python
agent, a shell script, a CI job or a language model with a `bash` tool.

## The shape of a worker

```sh
#!/bin/sh
set -eu

claim=$(tix claim next -p infra -o json -q) || exit 0   # exit 3 means the queue is empty
ref=$(printf '%s' "$claim" | jq -r .task.ref)
token=$(printf '%s' "$claim" | jq -r .lease_token)

tix task mv "$ref" doing --lease-token "$token" -q

# ... do the work, renewing on a ticker ...
tix claim renew "$ref" --token "$token" --ttl 15m -q

tix claim release "$ref" --token "$token" --status done --result exit_code=0 --comment "built and pushed"
```

Three things make this safe: the claim is atomic, the lease expires on its own, and every write after the claim
requires the token the claim minted.

## Claiming

`tix claim next` selects and claims one task in a single compare-and-swap. Two workers racing for the same queue
each get a different task, with no lock, no coordinator and no server process.

| Flag | Effect |
| --- | --- |
| `-p, --project` | restrict to project keys |
| `-s, --status` | restrict to statuses |
| `-l, --tag` | restrict to tags |
| `--ttl` | lease duration; defaults to the workflow's `default_lease` |
| `--actor` | claim on behalf of another actor |

Claim eligibility covers every task that is not blocked by an unfinished dependency, not held by a live lease,
and not already in a terminal state (`done`, `cancelled`). A queue whose tasks are all terminal reports the same
empty-queue result as a queue with nothing in it: exit 3, `no_task_available`. A worker that should only pick up
work in a specific non-terminal status still should say so, e.g. `tix claim next -s todo`, since a task sitting in
`blocked` is non-terminal and would otherwise be handed out.

`tix claim task REF` claims one named task instead of taking the next. It fails with exit 4 when the task is
already held.

## The JSON an agent sees

`tix claim next -o json` prints exactly this:

```json
{
  "task": {
    "id": "01M30B3Y1TVT299QJPV8W8MM4X",
    "tenant_id": "01M30B3Y1QN9WXZH9YK9VJYMFM",
    "project_id": "01M30B3Y1RBSS6JAHPY850QMB6",
    "seq": 1,
    "ref": "default-1",
    "title": "migrate the database",
    "status": "todo",
    "priority": 3,
    "creator_actor_id": "01M30B3Y1SNEPGNQJ489NMR8TT",
    "claimed_by_actor_id": "01M30B3Y1SNEPGNQJ489NMR8TT",
    "claimed_at": "2026-09-20T21:21:24.047605506Z",
    "lease_expires_at": "2026-09-20T21:51:24.047605506Z",
    "claim_count": 1,
    "version": 1,
    "created_at": "2026-09-20T21:21:24.026248465Z",
    "updated_at": "2026-09-20T21:21:24.048938578Z",
    "blocked": false
  },
  "lease_token": "gu-zHLY0JqUx4F767USCHPjyR_rrQovZNgZMstT92Ic",
  "lease_expires_at": "2026-09-20T21:51:24.047605506Z"
}
```

`lease_token` is the only field that is not derivable from a later read. Capture it before anything else can
fail. `priority` is numeric: 1 highest, 3 normal, 5 lowest. Claiming does not by itself transition the task, so
`status` is whatever it was; move it yourself, or let `claim exec --on-start` do it.

## Leases

A lease is a claim with a deadline. `lease_expires_at` is absolute UTC, not a duration, so a worker never has to
reason about its own clock drift relative to the store.

Renew before the deadline:

```sh
tix claim renew infra-42 --token "$TOKEN" --ttl 15m
```

If nobody renews, the lease expires. `tix claim sweep` reaps expired leases and, for any state whose workflow
definition sets `revert_on_lease_expiry`, sends the task back to its `revert_to` state. The default workflow
marks `doing` that way, reverting to `todo`. A server started with `tix serve` runs the sweeper on a ticker
(`--sweep-interval`, default `1m`); on a machine with no server, run `tix claim sweep` from cron or before each
poll. `--dry-run` reports what would be swept without writing.

No supervisor is involved in any of this. A worker that is SIGKILLed, loses power or is scheduled away simply
stops renewing, and its task returns to the queue.

## Why a zombie cannot write

A worker whose lease expired while it was blocked on something has no way to notice on its own. tix makes that
harmless: the lease token is required on every write that touches a claimed task.

- `tix claim renew` needs `--token`
- `tix claim release` needs `--token`
- `tix task mv` needs `--lease-token` when the task is claimed

A token that is not the one currently held fails with exit 4 and a `lease_expired` error:

```console
$ tix claim renew default-1 --token stale
error: lease_expired: the lease on task "default-1" is no longer held under that token; claim it again

$ tix task mv default-1 done
error: lease_expired: task "default-1" is claimed; supply the lease token
```

When the lease expired and another worker re-claimed the task, the store minted a new token. The old one no
longer matches, so the zombie's write is rejected rather than overwriting the new holder's work. The zombie does
not need to cooperate, and the new holder does not need to defend itself.

## tix claim exec

`tix claim exec` is the whole loop in one command: claim, run a child process while renewing the lease, then
release with the child's exit status recorded as the result.

```sh
tix claim exec --on-start doing -- ./worker.sh
tix claim exec --ref infra-42 --on-start doing --on-failure blocked -- make test
```

| Flag | Effect |
| --- | --- |
| `--on-start` | status to move the task to before running the child |
| `--on-success` | status to release to when the child exits 0 (default `done`) |
| `--on-failure` | status to release to when the child exits non-zero |
| `--ref` | claim this task instead of the next eligible one |
| `-p`, `-s`, `-l`, `--ttl`, `--actor` | as for `claim next` |

The release status must be reachable from the task's current state. The default workflow has no `todo` to `done`
edge, so `--on-start doing` is not optional there; without it the release fails after the child has already run.

The child's exit status becomes the exit status of `tix claim exec`, which is why the exec exit codes are
different from every other command:

| Exit | Meaning |
| --- | --- |
| 3 | no eligible task; the child was never started |
| 4 | the lease was lost |
| other | the child's own exit status |

Because 3 and 4 collide with plausible child exit codes, distinguish them by checking whether the child produced
output, or claim explicitly with `claim next` when the distinction matters.

If `tix claim exec` is killed, renewal stops with it. The lease expires on schedule and the task becomes
claimable again with no cleanup step.

## Exit codes

Every command uses the same table, which `tix --help` also prints.

| Exit | Meaning |
| --- | --- |
| 0 | success |
| 1 | error |
| 2 | usage, including an invalid flag or a dependency cycle |
| 3 | not found, including an empty queue |
| 4 | conflict, including a held task, a lost lease and a version clash |
| 5 | permission denied |
| 6 | precondition failed, such as an illegal transition or a task with subtasks |

An empty queue is exit 3 with a `no_task_available` error on stderr, not exit 0 with an empty result. A polling
worker should treat 3 as "sleep and try again" and anything above it as a real failure.

## Tokens and scopes

Give an agent a token, not a user session:

```sh
tix token create ci --scope task:read --scope task:claim --scope task:transition
```

`--project` on `token create` takes either the project's key or its id, the same as `-p/--project` on `task ls`,
`claim next` and friends: `tix token create ci --project infra ...` restricts the token to that project.

The token value is printed once and cannot be retrieved again. Supply it with `--token`, or the `TIX_TOKEN`
environment variable, or a named context.

| Scope | Grants |
| --- | --- |
| `task:read` | read tasks |
| `task:write` | create and edit tasks |
| `task:transition` | move a task between states |
| `task:claim` | claim, renew, release |
| `task:delete` | delete and restore tasks |
| `project:read` / `project:write` | read and change projects |
| `workflow:read` / `workflow:write` | read and change workflows |
| `comment:write` | comment on tasks |
| `artifact:write` | attach artifacts |
| `event:subscribe` | subscribe to the event stream: task, comment, artifact, dependency, label, project, workflow and field events. Events about people, credentials, webhooks and sync sources additionally need the admin scope that reads them |
| `user:admin` | manage users |
| `token:admin` | manage tokens |
| `webhook:admin` | manage webhooks |
| `tenant:admin` | manage tenants, members and domains |
| `sync:admin` | manage and run import sources |
| `audit:read` | read the audit log |
| `export` / `import` | stream a tenant snapshot out or in |

`*` grants everything and is what the `admin` role carries. Human roles map to scope sets: `viewer` gets the read
scopes plus `event:subscribe` and `audit:read`; `member` adds task write, transition, claim, comments, artifacts
and `export`; `admin` gets `*`.

`--project` on a token pins it to one project. A project-scoped credential subscribing to the event stream has
its subscription narrowed to that project, and naming a different project is refused.

A useful agent token is `task:read`, `task:claim`, `task:transition` and, if the agent comments on its work,
`comment:write`. Anything more is unnecessary.

### Public keys

A public key is the third kind of credential, beside a password and a token. `tix ssh` accepts any key and
treats the fingerprint as the identity, which is what lets the demo listener hand every visitor their own
sandbox with no signup. See [deployment.md](deployment.md#the-terminal-interface-over-ssh).

Enrolling a key against an existing user, so that a real account can be reached over SSH rather than a
throwaway sandbox, is not built yet. The credential is already the right shape for it: resolution is
fingerprint to actor behind one interface, and enrolment replaces the lookup without touching anything above it.
An agent today should still use a token.

## Reporting results

`tix claim release` records what happened:

```sh
tix claim release infra-42 --token "$TOKEN" --status done \
  --result exit_code=0 --result artifact=s3://bucket/build.tar.gz \
  --comment "built at $(git rev-parse --short HEAD)"
```

`--result` takes repeatable `key=value` pairs, `--comment` records a comment with the release, and `--status`
transitions the task if the workflow allows that edge from the current state.

## Watching instead of polling

An agent that reacts to work rather than polls for it should subscribe to the event stream over WebSocket and
claim when a `task.created` or `task.released` event arrives. See [api.md](api.md) for the protocol and for
resuming a subscription after a disconnect without missing events.

`tix watch -o ndjson` gives the same stream on the command line, for a script or for a person watching agents
work: `--type` narrows to event types, `--project` to projects, and `--actor` to the agents whose activity
matters right now, each repeatable. `--since` resumes without a gap. On connecting it prints one line to
standard error confirming the stream is live and what it is filtered on; standard output carries nothing but
events, so a script reading ndjson off stdout never has to skip it. See
[scripting.md](scripting.md#watching-the-event-stream) for the human-readable default a person reads instead
of ndjson.

## Related

- [scripting.md](scripting.md) for output formats, piping and NDJSON
- [workflows.md](workflows.md) for state machines and what `revert_on_lease_expiry` does
- [api.md](api.md) for the same operations over HTTP
