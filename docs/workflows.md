# Workflows and custom fields

A workflow is a state machine: the states a task can be in, the edges between them, and what happens when a lease
on a task expires. Every project has exactly one. A fresh install has one built-in workflow called `default`, and
nothing has to be configured to start working.

## The default workflow

```sh
tix workflow ls -o json
```

States: `todo`, `doing`, `blocked`, `done`, `cancelled`. `done` and `cancelled` are terminal. `doing` reverts to
`todo` when a lease expires. The default lease is 30 minutes.

There is no `todo` to `done` edge. Work goes through `doing`. That is why `tix claim exec` takes `--on-start`.

## Defining a workflow

`tix workflow put` reads a YAML or JSON document from a file or standard input:

```yaml
key: review
name: Review
definition:
  initial: triage
  default_lease: 15m
  states:
    - key: triage
      label: Triage
      category: todo
    - key: working
      label: Working
      category: in_progress
      revert_on_lease_expiry: true
      revert_to: triage
    - key: review
      label: In review
      category: in_progress
    - key: shipped
      label: Shipped
      category: done
      terminal: true
    - key: dropped
      label: Dropped
      category: done
      terminal: true
  transitions:
    - from: triage
      to: working
    - from: working
      to: review
    - from: review
      to: working
    - from: review
      to: shipped
      requires_comment: true
    - from: triage
      to: dropped
    - from: working
      to: dropped
      requires_scope: task:delete
```

```sh
tix workflow put -f review.yaml
tix workflow put -f review.yaml --dry-run    # validate and write nothing
cat review.yaml | tix workflow put -f -
```

### States

| Field | Meaning |
| --- | --- |
| `key` | the identifier used in `tix task mv` and in filters |
| `label` | what humans see |
| `terminal` | the task is finished; dependents waiting on it become unblocked |
| `category` | `todo`, `in_progress` or `done`; groups states for boards and reporting |
| `revert_on_lease_expiry` | when a lease expires in this state, move the task back |
| `revert_to` | the state to revert to |

`terminal` and `category: done` are independent. `category` is presentation and grouping; `terminal` is the one
that decides whether a dependency is satisfied and whether the task is out of the working set.

### Transitions

A transition is an ordered edge. Anything not listed is refused:

```console
$ tix task mv rv-1 shipped
error: invalid: workflow "review" has no transition from "triage" to "shipped"
```

That is exit 2.

| Field | Meaning |
| --- | --- |
| `from`, `to` | the edge |
| `requires_comment` | the transition is refused without `--comment` |
| `requires_scope` | the caller must hold this scope |

```console
$ tix task mv rv-1 shipped
error: invalid: moving from "working" to "shipped" requires a comment

$ tix task mv rv-1 shipped --comment "reviewed by ops"
```

### Terminal states and dependencies

`tix dep add A B` records that A waits for B. A is blocked until B reaches a terminal state. `tix task ls
--blocked` and `--unclaimed` filter on that, and `tix claim next` never hands out a blocked task.

Cycles are refused with exit 6.

## Assigning a workflow

```sh
tix project create rv "Review" --workflow review
tix project edit rv --workflow review
```

Removing a state that tasks currently sit in is refused with exit 6, as is removing a workflow still assigned to a
project.

## Marking a project

A project carries a colour and an icon so its rows are recognisable at a glance in the CLI table and in the
browser interface:

```sh
tix project create rv "Review" --workflow review --color violet --icon 🚀
tix project edit rv --color teal --icon RV
tix project edit rv --color "" --icon ""        # clear both
```

`--color` is one of `slate`, `red`, `amber`, `green`, `teal`, `blue`, `violet` or `pink`; anything else is exit 2.
`--icon` is one emoji or a monogram of at most two characters. Both are optional, and a project with neither
renders with those columns empty. They travel with `tix export`/`tix import` and with a project template in a
bundle.

The `COLOR` column is drawn as a swatch in the project's own colour when the output is a terminal, and as the
bare colour name otherwise.

## Custom fields

Fields are defined per project:

```sh
tix field put rv severity --type enum --option low --option high --label Severity --indexed
tix field put rv owner --type actor --required
tix field ls rv
tix field rm rv severity
```

| Flag | Effect |
| --- | --- |
| `--type` | `string`, `text`, `int`, `float`, `bool`, `date`, `datetime`, `enum`, `actor`, `json` (default `string`) |
| `--option` | an enum option, repeatable |
| `--required` | every task in the project must carry a value |
| `--indexed` | see below |
| `--position` | ordering in the UI |
| `--label` | human label, defaulting to the key |
| `--dry-run` | validate without writing |

Values are set on tasks with repeatable `key=value` pairs:

```sh
tix task add "harden the ingress" -p rv --field severity=high --field owner=ops
tix task edit rv-1 --field severity=low
tix task mv rv-1 working --field owner=alice
```

Marking a field `--required` does not retroactively invalidate existing tasks. Changing `--indexed` later is
allowed and does not rewrite data.

## Indexed versus scanned field filters

Filtering on a custom field is always allowed and never an error. What changes with `--indexed` is the cost.

An indexed field is one the engine is expected to answer efficiently. A field that is not indexed is filtered by
reading the JSON document on each candidate row, so the work grows with the number of rows the rest of the filter
leaves behind.

The practical rule: a custom field you filter or sort a queue on should be indexed. A custom field you only read
back on a task you already fetched should not be, because an index you never query is write cost with no payoff.

Either way, narrow the scan first. A custom field predicate is cheap once the project, status and tag predicates
have already cut the candidate set:

```sh
tix task ls -p rv -s todo -l urgent
```

`tix task ls` has no flag for custom field values. The HTTP API is where a custom field filter is expressed:
`GET /api/v1/tasks?field.severity=high`. See [api.md](api.md).

## Sharing a workflow between installations

```sh
tix bundle export --kind workflow --workflow review --name review-kit > review.bundle
tix bundle import review.bundle --on-collision rename
```

Bundles carry workflows, field definitions, tags, project templates and webhook endpoints: the way of working,
never the work itself. See [migrating.md](migrating.md).
