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
| 2 | usage, invalid input, illegal transition |
| 3 | not found, empty queue |
| 4 | conflict, held lease, version clash |
| 5 | permission denied |
| 6 | precondition failed |

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
