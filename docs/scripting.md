# Scripting

tix is meant to be driven from a shell. Every command has a machine-readable form, a documented exit code, and no
interactive prompt.

## Output formats

`-o` selects the renderer, and `TIX_OUTPUT_FORMAT` or `output.format` sets the default.

| Format | Shape | Use |
| --- | --- | --- |
| `table` | aligned columns with a box border (default) | reading |
| `json` | one pretty-printed document | `jq`, a single record, a small list |
| `ndjson` | one compact object per line | streaming, large lists, `while read` |
| `yaml` | a YAML document | editing by hand, diffing |

```console
$ tix task ls -o ndjson
{"id":"01M30...","ref":"default-1","title":"seed","status":"todo","priority":3, ...}
{"id":"01M30...","ref":"default-2","title":"acme work","status":"todo","priority":3, ...}
```

An unsupported format is rejected before anything is opened, with exit 2.

## Colour

`json`, `yaml` and `ndjson` never carry colour. Whatever the colour mode, a machine-readable format is written
without a single escape code, so a parser never has to strip one:

```console
$ tix project ls --color -o json | grep -c $'\e['
0
```

Only `table` is coloured, and only when standard output is a terminal. A pipe, a redirect to a file or a
subshell capture is detected and drops the colour, which is why `out=$(tix task ls)` holds plain text with no
flag needed. `--no-color` or `TIX_OUTPUT_COLOR=never` turns it off everywhere; `--color` or
`TIX_OUTPUT_COLOR=always` keeps it on through a pipe, for feeding a pager. `NO_COLOR` and `TIX_NO_COLOR` are
honoured while the mode is `auto`. See [configuration.md](configuration.md).

## Watching the event stream

`tix watch` follows domain events as they are committed, until it is interrupted. Once the stream is up it prints a
one-line banner naming what it is connected to, any active filter, and how to stop:

```console
$ tix watch
watching local /tmp/scratch/tix.db (from --db); ctrl-c to stop
000043  10:15:03  alice  created         homelab-9   Move backups off the old NAS
000044  10:15:12  alice  claimed         homelab-9   lease 30m
000045  10:15:40  alice  transitioned    homelab-9   todo → doing
000046  10:16:02  bob    claimed         homelab-4   for agent-pax, lease 1h
000047  10:16:20  alice  updated         homelab-9   the title and priority
000048  10:46:02  bob    lease expired   homelab-4   held by agent-pax, reverted to todo
000049  10:47:11  alice  released        homelab-9   → done
```

Each line leads with the event's sequence number, and that number is the resume cursor: whatever handled line
`000047` can pass `--since 47` after a restart and pick up from there. The rest of the line answers "what
changed", not just "something changed": a transition names both states, an edit names the attributes it
touched, a claim names the lease length and, when the lease was taken on somebody else's behalf, its holder,
and a lease expiry names the holder it was taken from and the state the task fell back to.

The banner always goes to standard error, never standard output, and `--quiet` suppresses it the same way it
suppresses every other diagnostic: a working stream, a hung one, a wrong filter and a failed connection no
longer all look identical -- silence -- but stdout stays exactly as parseable as before.

The `updated` line's detail is only ever as specific as the event's own payload: a restore, a lease renewal, a
tag or dependency removal and a comment deletion each say what happened, because their payload carries a flag
for it, and a plain field edit now does too -- the `task.updated` payload carries a `fields` array naming every
attribute the edit touched, so the line reads `updated  homelab-9  the title, priority and due date` for a
multi-field `task edit`, not just "updated" with no detail.

That default line-per-event format is what `table` renders for this command: a table cannot size its columns
until the stream ends, which a live tail never does, so `watch` draws one readable line per event instead. Pass
`-o ndjson` for a machine to parse, matching every other listing; the startup banner still lands on standard
error only, so a pipeline never sees it:

```console
$ tix watch -o ndjson --since 42 | jq .
{"seq":43,"type":"task.claimed","project_id":"...","subject_type":"task","subject_id":"...","actor_id":"...",
 "payload":{"ref":"homelab-9","actor_handle":"alice","claimed_by":"...","lease_expires_at":"..."},
 "occurred_at":"..."}
```

The structured formats carry the whole event -- sequence number, identifier, tenant, type, actor, subject type
and identifier, timestamp and the full payload -- so a watching process never has to go back and query for
what the event already knows. The human line is the summary; `ndjson` is the record.

`--project`, `--type` and `--actor` narrow the stream, each repeatable; `--type` takes a trailing `*` for a
prefix match such as `task.*`. `--limit` stops after a fixed number of events instead of running until
interrupted.

```sh
tix watch --actor agent-pax --type task.*
```

### The delivery guarantee

**At-least-once, with a cursor.** Not exactly-once. Concretely:

- Every event is a durable row with a sequence number that is monotonic within a tenant. That number is the
  only thing a consumer has to remember.
- With no `--since`, the stream starts at the *next* event. Anything committed before the command started is
  skipped, deliberately: a tail is a tail.
- With `--since N`, the durable log is replayed from just after `N` before live delivery begins. Every event
  committed while nothing was watching arrives, in order. That is the gap-free property, and it holds whether
  `tix watch` is talking to a database file directly or to a server over the WebSocket. Both paths were tested
  against the same property; a divergence between them would be a bug, not a documented difference.
- Resuming from a cursor at or before an event you already handled delivers that event **again**. Handling has
  to be idempotent. There is no acknowledgement protocol and no exactly-once mode.
- A cursor the retention sweep has already passed is **refused** with an error naming the oldest retained
  sequence, rather than silently handing back the oldest survivor as though nothing were missing. Both paths
  refuse it.
- Against a server, a consumer too slow to keep up is disconnected with `slow consumer: ... resume from your
  last seq` rather than being quietly starved. Record the cursor, reconnect, resume.

A minimal resumable loop:

```sh
cursor=$(cat .tix-cursor 2>/dev/null || echo 0)
while :; do
  tix watch --since "$cursor" -o ndjson \
    | while IFS= read -r line; do
        handle "$line"                                   # must be idempotent
        printf '%s' "$(jq -r .seq <<<"$line")" > .tix-cursor
      done
  cursor=$(cat .tix-cursor)
done
```

## NDJSON everywhere

The same line-per-record shape shows up in four places, and they compose:

```sh
tix task ls -o ndjson                       # listings
tix export > snapshot.ndjson                # tenant snapshots
tix bundle export > kit.bundle              # workflows, fields, tags, project templates
curl -H 'Accept: application/x-ndjson' ...  # HTTP listings
```

Snapshots and bundles are written as the data is walked, so a dump of any size streams in constant memory and
pipes straight into its counterpart:

```sh
tix export | tix import --mode merge
tix bundle export | tix bundle import --on-collision rename
```

## Piping

```sh
# Close every task that mentions a term
tix task ls -o ndjson --query "flaky" \
  | jq -r .ref \
  | xargs -r -n1 -I{} tix task mv {} done --comment "superseded"

# Count open tasks per project
tix task ls --all -o ndjson -s todo | jq -r .project_id | sort | uniq -c

# Feed a body in from another process
git log -1 --format=%B | tix comment add infra-42 -
```

`--all` follows cursors until every page is read. Without it, a listing returns one page of 50.

`--body -` and a bare `-` where a body is expected read standard input, which is what makes `tix comment add`
and `tix task add --body -` composable with anything that writes to a pipe.

## Exit codes

| Exit | Meaning |
| --- | --- |
| 0 | success |
| 1 | error |
| 2 | usage, invalid input, dependency cycle |
| 3 | not found, empty queue |
| 4 | conflict, held lease, version clash |
| 5 | permission denied |
| 6 | precondition failed, illegal transition |

`tix tui` additionally uses 130 for an interrupt, and `tix claim exec` passes the child's exit status through.
See [agents.md](agents.md).

```sh
if ! claim=$(tix claim next -o json -q); then
  case $? in
    3) exit 0 ;;                       # nothing to do
    *) echo "claim failed" >&2; exit 1 ;;
  esac
fi
```

Diagnostics go to standard error, data to standard output, always. That is why `tix export > file` and
`tix bundle export | tix bundle import` cannot be polluted by a progress line.

## Dry runs

Most mutations take `--dry-run` and report a plan instead of writing:

```console
$ tix task rm default-1 --cascade --dry-run -o json
[
  {
    "action": "task.delete",
    "target": "default-1",
    "status": "planned",
    "detail": {"cascade": true, "hard": false},
    "dry_run": true
  }
]
```

`tix bundle import --preview` is the equivalent for bundles, and `tix sync run --dry-run` reports every creation,
update and skip with the reason for each skip.

## Batch commands and partial failure

`tix task mv`, `tix task rm` and `tix task restore` take several references. They report per reference:

```console
$ tix task rm default-1 -o json
[{"ref":"default-1","status":"failed","code":"precondition_failed",
  "error":"task \"default-1\" has 1 descendant task(s); pass cascade or delete them first"}]
```

The process exit code reflects the failure, and the per-record `status` and `code` say which reference failed and
why. Check both: a non-zero exit does not mean nothing happened.

`--cascade` on `tix task rm` also deletes the task's subtasks. `--hard` deletes permanently instead of soft
deleting; a soft delete is reversible with `tix task restore` and visible with `tix task ls --include-deleted`.

## Optimistic locking

`tix task edit --version N` refuses the edit with exit 4 if the task has changed since version N was read. Read
the version from the task's `version` field. Without `--version`, last write wins.

## Quiet and verbose

`-q` suppresses diagnostics on standard error while leaving the data alone. `-v` prints how the target was
resolved:

```console
$ tix task ls -v
target: local /home/you/.local/share/tix/tix.db (from config)
```

## Talking to a server instead of a database

Every command works against either:

```sh
tix task ls --db sqlite://./tasks.db
tix task ls --server https://tix.example.com --token "$TIX_TOKEN"
```

The same service code runs in both cases, so behaviour including conflict reporting is identical. Name the
target once with a context instead of repeating flags. See [configuration.md](configuration.md).

## Shell completion

`tix completion bash|zsh|fish` generates a script offering dynamic completion of task references, project keys,
tags and statuses. See [shell-completion.md](shell-completion.md).
