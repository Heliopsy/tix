# Targets, credentials and the rest of the product

Read this when you need to point tix somewhere other than the default, mint a
credential for an agent, define a workflow, or move data in or out. None of it
is on the hot path.

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
config file > default. `--no-discovery` ignores the per-directory context
files, which is what you want when a script must not be steered by whatever
directory it happens to run in.

Three commands answer "where am I writing?", and it is worth asking before a
first write rather than after:

```sh
tix config show -v    # what resolved, and why
tix doctor            # whether the target is sound; fails when a check does
tix status            # what the installation holds and what is serving it
```

`tix status` and `tix doctor` are different questions. Doctor asks whether
this installation is sound and fails when it is not. Status reports what
exists — version, schema, engine, attached servers, tenants, projects, tasks,
claims, **unswept expired leases**, pending and failed webhooks — and a server
on another machine being down does not make it fail. A server registers itself
when it starts and refreshes that registration while it runs, so a row nobody
is refreshing is a process that died; a graceful shutdown removes its own row.

```console
$ tix status
INSTALLATION
  version  0.15.1  store   local /tmp/scratch.db (from flag)
  schema   10      engine  sqlite

SERVERS (0 attached)
  none registered; nothing is serving this store right now

WORK
  1 tenant   4 projects  1 task
  0 claimed  0 leases expired and unswept
  webhooks: 0 pending, 0 failed
```

"none registered; nothing is serving this store right now" plus a nonzero
unswept count is the signal that no sweeper is running, which is the trap in
SKILL.md's crash-recovery section.

## Tokens and scopes

Give an agent a scoped token, not a user session:

```sh
tix token create ci-agent --scope task:read --scope task:claim \
  --scope task:transition --scope comment:write
```

The value prints once. `task:read`, `task:claim`, `task:transition`,
`comment:write` is normally all an agent needs; add `task:write` if it will
call `dep add` or `task add` (so, if it writes its own plan down, it needs
`task:write`), `artifact:write` if it records artifacts, `task:delete` to
remove tasks, `event:subscribe` for `tix watch`, and `task:reclaim` only for a
supervisor that takes leases away from dead workers. `task:reclaim` is not
implied by `task:claim`. An unknown scope is `invalid`, exit `2`:

```console
$ tix token create x --scope nosuch
error: invalid: unknown scope "nosuch"
```

`--project` on `token create` takes either the project's key or its id, same
as `-p`/`--project` everywhere else (`--project infra` works); an unknown key
or id is a clean `not_found`, exit `3`.

Two live tokens of one tenant cannot share a name, since the name is what you
pick by when you revoke:

```console
$ tix token create ci-agent --scope task:read
error: conflict: a token named "ci-agent" already exists; revoke it or choose another name
```

That is exit `4`. Revoking frees the name, so rotation is `tix token rm ID`
then the same `token create` again.

`--expires` (default 90d) bounds the token: a duration such as `90d`, a date,
an RFC 3339 timestamp, or `never`. **A token you mint without saying anything
expires in ninety days.** That changed: omitting the flag used to mean "never",
so a script written against the old behaviour now produces a credential that
stops working, with `invalid_credentials` and exit `5` on the first call after
it lapses. `--expires never` is the only way to get a token that does not
expire, and the browser form proposes the same ninety days.

```sh
tix token create ci-agent --expires 2027-01-01 --scope task:read
tix token create ci-agent --expires never --scope task:read
```

`token:admin` reaches the whole tenant's tokens, not only yours:
`tix token ls --actor carol` lists hers, and `tix token rm ID` revokes any of
them. Revoking a token that is not yours is recorded as `token.revoke_other`
in the audit trail, so it can be told apart from somebody rotating their own.

## Workflows and custom fields

A workflow is the state machine a project's tasks move through. Read one
before you move a task, because nothing guarantees a tenant uses the built-in
five states:

```sh
tix workflow ls -o json
tix workflow get default -o json
tix workflow put --file ./workflow.json   # the key comes from the document
```

Each state carries a `key`, a `label`, `terminal`, a `category` from the six
(`todo`, `in_progress`, `blocked`, `waiting`, `done`, `cancelled`) and
optionally `revert_on_lease_expiry` with `revert_to`, which is what
`tix claim sweep` acts on. The definition also carries `default_lease`, the
TTL a claim gets when it does not ask for one — 30m in the built-in workflow.

Custom fields are per-project typed slots on a task:

```sh
tix field ls default
tix field put default severity --type string --required --indexed
tix task add "outage" -p default --field severity=high
tix task ls --filter 'field.severity:high'
```

`--required` refuses a task that omits the field; `--indexed` makes it
filterable at scale rather than scanned. Note that an unknown field key in a
filter answers empty rather than `not_found`, so a typo looks like "nothing
matches".

## The audit log

Every mutation writes its domain rows, its audit entry and its outbox event in
one transaction, so the audit log cannot disagree with the data:

```sh
tix audit ls -o json --limit 50
tix audit ls --actor ci-agent
```

Audit actions are imperative and name who did what (`task.reclaim`,
`token.revoke_other`); the event stream's types are past tense
(`task.reclaimed`). They are different vocabularies on purpose. Use the audit
log to explain what an agent did; use the event stream to react to it.

## Running a server

```sh
tix serve --db /var/lib/tix/tix.db --listen 127.0.0.1:8080
```

One process serves the HTTP API, the WebSocket event stream and the web UI
over one database, and runs the lease sweeper, the retention pruner and the
webhook dispatcher on tickers. For an agent queue that is the difference
between a crashed task coming back to `todo` on its own and sitting in `doing`
until somebody sweeps. `tix serve --ssh-listen <addr>` adds the SSH listener
beside it; see [ssh.md](ssh.md). `tix serve --help` has the deployment flags,
and the project's `docs/deployment.md` owns that subject.

`tix tui` is the terminal interface over the same store, which is worth
knowing exists because it is what a human watching your queue will be looking
at.

## History retention

Events, audit entries and webhook deliveries are kept for a bounded window,
not for ever:

```sh
tix retention show
tix retention set --events 30d --audit 8760h --webhook-deliveries 30d
tix prune --dry-run
tix prune
```

Defaults are 720h for events and webhook deliveries and 8760h for audit
entries. `tix prune` deletes what is past the policy and `--limit` caps one
run; `tix serve` runs it on a ticker. This matters to an agent for one reason:
a `tix watch --since` cursor older than the events policy cannot be resumed
from, because the events are gone. The prune reports
`RETAINEDFORSUBSCRIBERS`, which is what it kept back for a live subscriber
rather than removing under it.

## Moving data in and out

A snapshot is the whole tenant's work:

```sh
tix export --artifacts --comments > snapshot.json
tix import --mode merge --dry-run < snapshot.json
tix import --mode merge < snapshot.json
```

`--artifacts` and `--comments` include those bodies, which are the bulk of a
snapshot, so leaving them off gives you the task graph alone. `--dry-run`
reports what an import would apply; without it the import writes.

`--mode` is **required** and has no default, because one of its two values is
destructive: `merge` adds and updates, `replace` overwrites. Omitting it is
exit `2`:

```console
$ tix import < snapshot.json
error: invalid: import mode is required; there is no default because replace is destructive
```

A bundle is the opposite choice: reusable configuration and **never work
items**, so sharing a way of working cannot disclose what anyone is working
on. Webhook endpoints travel without their signing secrets and arrive
inactive. Both directions stream, so one installation pipes straight into
another:

```sh
tix bundle export --kind workflow --workflow default > bundle.json
tix bundle import --on-collision skip --preview < bundle.json
tix bundle import --on-collision skip < bundle.json
```

`--preview` reports what the bundle would change and writes nothing at all.
`--on-collision` is **required**, for the same reason `--mode` is: `skip`,
`rename` and `replace` are not interchangeable and the destructive one must be
asked for.

```console
$ tix bundle import < bundle.json
error: invalid: a collision policy is required; there is no default because replace overwrites a component others may be using
```

`bundle export` narrows what travels with `--kind`
(`workflow|field_def|tag|project_template|webhook`), `--workflow`,
`-p`/`--project`, `--webhook` and `--name`.

`tix sync` pulls from Jira, OpenProject and shaped files:

```sh
tix sync source ls
tix sync source add jira production-jira
tix sync run SOURCEID --dry-run
tix sync run SOURCEID --full
```

A run names the source it pulls from, which `sync source ls` lists. A run is
incremental by default; `--full` ignores the stored cursor and reconsiders
every source record. Adding a source only registers it: credentials come from
`TIX_SYNC_<NAME>_*` in the environment and the field mapping from a file, and
a run before either is in place is exit `2` saying which is missing. A source that answers badly is exit `7`,
`upstream_failed` — that is the one exit code an agent will otherwise never
see, and it means the remote system failed, not that your request was wrong.

`tix webhook` registers outgoing HTTP calls on events:

```sh
tix webhook ls -o json
tix webhook put https://example.test/hook --event task.released
tix webhook put https://example.test/hook --event task.released --inactive
tix webhook redeliver DELIVERYID
```

`--inactive` registers the endpoint without arming it. `tix status` reports
pending and failed deliveries, and `tix serve` is what dispatches them: with
no server running, a registered webhook fires nothing.
