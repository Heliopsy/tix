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

## Templates

tix ships twelve ready-made workflows. Each one leads with a one-line purpose, which is what every list and
picker shows first, then its name, its states and the custom fields a project using it usually wants.

```sh
tix workflow template ls                                   # purpose first
tix workflow template apply tix-dev-review                 # creates the workflow tix-dev-review
tix workflow template apply tix-sales --as deals           # the same workflow under another key
tix workflow template apply tix-bugs --on-collision rename # tix-bugs-2 when tix-bugs is taken
tix project create web "Website" --workflow-template tix-dev-review
```

Applying a template creates a workflow, nothing more. A key that is already taken is refused (exit 4) unless
`--on-collision` says to `skip` it, `rename` the new one to the next free `KEY-2`, or `replace` the existing
definition; replacing is refused for `default`, and like `tix workflow put` it is refused while tasks sit in a
state the template does not have.

Custom fields belong to projects, not to workflows, so a template carries its fields in a `fields` section
beside the definition. They are added when a project is created from the template: `tix project create
--workflow-template`, the browser's New project dialog and the terminal's new project form all do that. The
project runs on the workflow under the template's key, created the first time and reused after, so ten
projects made from `tix-bugs` share one workflow. A project that already exists gets the fields with `tix field
put`.

The `tix-` prefix is reserved for the templates. `tix workflow put` refuses a new workflow whose key starts
with it (exit 2), so a key that does start with it always came from a template. A workflow a template created
can still be edited by key, and `--as` can put a template's workflow under any key you like. Display names
carry no prefix; the pickers mark a template with a Built-in label instead. `default` stays the default
workflow of a project created without one.

The browser lists the templates in the New project dialog's workflow picker and in the Workflows screen's Add
from template dialog; both work without scripting. Names, purposes and state labels are translated into
Bulgarian there. The files themselves are plain YAML in `internal/workflows/templates/`, in the format `tix
workflow put` reads plus `key`, `name`, `purpose` and `fields`, so one is a starting point for a workflow of
your own: copy it, change the key and put it.

Edges not listed below are refused, as in any workflow. Every template allows the obvious back moves (sending
work back a step, reopening something finished) unless it says otherwise, and every state has a route to a
final one.

### Simple Kanban (`tix-kanban`)

Move cards left to right, nothing else.

States: `backlog` → `todo` → `doing` → `done` (final). Each step goes forward or back one column, and `done`
reopens to `doing`. No rules. No fields.

### Personal (GTD) (`tix-gtd`)

Capture everything, decide later, do the next thing.

States: `inbox` → `next` → `waiting` → `someday` → `done` (final). The inbox is triaged into any of the
others, including straight to `done`. `waiting` is in the `waiting` category: it is stalled on something
outside the board. No rules. Fields: `context` (enum: home, work, errand).

### Development with review (`tix-dev-review`)

Code goes through review and QA before it's done.

States: `triage` → `ready` → `in_progress` → `in_review` → `qa` → `done` / `cancelled` (final). An expired
lease in `in_progress` moves the task back to `ready`. Sending work back from `in_review` or from `qa` to
`in_progress` needs a comment. Every open state can be cancelled; `done` reopens to `ready` and `cancelled` to
`triage`. Fields: `pr` (string, the PR link), `size` (enum: xs, s, m, l, xl).

### Bug triage (`tix-bugs`)

Confirm bugs before anyone fixes them.

States: `new` → `triaged` → `confirmed` → `fixing` → `fixed` → `verified` / `wontfix` / `duplicate`
(final). There is no path to `fixing` that skips `confirmed`. `wontfix` and `duplicate` are in the `cancelled`
category and both need a comment, as do a failed verification (`fixed` back to `fixing`) and a regression
(`verified` back to `confirmed`). Fields: `severity` (enum: critical, major, minor, trivial),
`affected_version` (string), `reproducible` (bool).

### Releases (`tix-releases`)

Ship a change and prove it worked.

States: `planned` → `staged` → `deploying` → `deployed` → `verified` / `rolled_back` (final). Rolling back,
from `deploying` or `deployed`, needs a comment; a rolled back release can be planned again. Fields: `version`
(string), `environment` (enum: staging, production).

### Agent queue with sign-off (`tix-agent-review`)

Agents do the work, a human approves it.

States: `queued` → `claimed` → `needs_review` → `approved` / `rejected` (final). An expired lease in `claimed`
puts the task back in `queued`. Approving and rejecting need the `task:approve` scope, which no role but admin
carries: an agent's token does the work and cannot sign it off. Putting a rejected task back in the queue needs
a comment saying what to change. Fields: `agent` (actor), `output` (string, a link to the result).

### Incident (`tix-incident`)

Restore service first, learn from it after.

States: `detected` → `investigating` → `mitigated` → `resolved` → `postmortem_done` (final). A mitigation that
does not hold goes back to `investigating`, as does a resolved incident that recurs. No rules. Fields:
`severity` (enum: sev1 to sev4), `impact` (text), `started_at` (datetime).

### Support tickets (`tix-support`)

Answer people and close the loop.

States: `new` → `open` → `waiting` → `resolved` → `closed` (final). `waiting` is waiting on the customer, in the
`waiting` category. `resolved` is in the `done` category but not final, so a reply reopens it to `open`; so can
a closed ticket. No rules. Fields: `customer` (string), `channel` (enum: email, chat, phone, web), `urgency`
(enum: urgent, high, normal, low, default normal).

### Approval or request (`tix-approval`)

Someone asks, someone authorised decides.

States: `requested` → `under_review` → `approved` / `rejected` (final). Approving and rejecting need the
`task:approve` scope, and rejecting needs a comment. A reviewer who needs more information sends the request
back to `requested` with a comment. Fields: `requester` (actor), `amount` (float).

### Sales or BizDev pipeline (`tix-sales`)

Track deals from first contact to signed.

States: `lead` → `contacted` → `qualified` → `proposal` → `negotiation` → `won` / `lost` (final). A deal can
be won from `proposal` or `negotiation`, and lost from any open stage; losing one needs a comment saying why.
A lost deal can come back as a lead. Fields: `company` (string), `deal_value` (float), `expected_close` (date).

### Hiring (`tix-hiring`)

Move candidates through interviews to a decision.

States: `applied` → `screening` → `interview` → `offer` → `hired` / `rejected` / `withdrew` (final).
Rejecting a candidate, from any open stage, needs a comment; a candidate can withdraw from any open stage
without one. Fields: `role` (string), `source` (enum: referral, job-board, agency, direct), `salary_range`
(string).

### Content or editorial (`tix-content`)

Turn ideas into published pieces.

States: `idea` → `drafting` → `editing` → `approved` → `published` / `dropped` (final). Editing sends a piece
back to `drafting`, and an approved piece back to `editing`. Any open piece can be dropped, and a dropped one
can return as an idea. No rules. Fields: `channel` (enum: blog, newsletter, social, docs), `publish_date`
(date).

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
      category: cancelled
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
| `category` | one of the six below; groups states for boards, colour and reporting |
| `revert_on_lease_expiry` | when a lease expires in this state, move the task back |
| `revert_to` | the state to revert to |

`terminal` and `category: done` are independent. `category` is presentation and grouping; `terminal` is the one
that decides whether a dependency is satisfied and whether the task is out of the working set.

### Categories

| Category | Means |
| --- | --- |
| `todo` | not started |
| `in_progress` | someone is working on it |
| `blocked` | stalled on something inside this board |
| `waiting` | stalled on something outside it |
| `done` | finished |
| `cancelled` | will not be done |

A state may also name no category at all, which is how a workflow written before categories existed
still reports somewhere: a terminal state is read as `done`, the initial state as `todo`, and anything
else as `in_progress`. Naming a word that is not on this list is refused, and the refusal lists the six.

The vocabulary is fixed, and a state carries a category rather than a colour of its own. That is on
purpose: a category is what every surface derives colour from, so `blocked` is the same hue in one
tenant's terminal as in another's browser, and a terminal that renders no colour still has the word.

`cancelled` and `done` differ in more than the hue. The statistics screen's *where the work is*
breakdown counts them in separate rows, so abandoned work no longer inflates the completed column of
that table. The `completed` figure itself, the lead times and the leaderboard are unchanged: they read
`completed_at`, which follows `terminal`, and a cancelled task is still terminal.

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

### Reaching a state through another

A transition is a single edge, but a workflow often reaches a state only by passing through another.
`--hops` finds that route, prints it, and then applies it one ordinary transition per hop, so the task's
history records every state it went through rather than a jump that never happened:

```console
$ tix task mv rv-1 shipped --hops
rv-1: triage -> working -> shipped (2 steps)
```

`--dry-run` prints the same route and writes nothing. The terminal's `t` picker and the browser's task
list, task screen and board offer the same routes, each naming the states it passes through before it is
chosen, and `POST /api/v1/tasks/{ref}/route` is the same thing over the wire.

When two routes of the same length reach the state, none is chosen for you:

```console
$ tix task mv rv-1 shipped --hops
error: invalid: two routes of 2 steps lead from "triage" to "shipped", working>shipped and
review>shipped; name the route you want instead of the state

$ tix task mv rv-1 review>shipped --hops
rv-1: triage -> review -> shipped (2 steps)
```

A route that is refused part way through leaves the task where the last successful hop put it, and says
so rather than reporting only a failure.

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

## Changing a project's key

The key is the start of every task reference and of the project's address, and it can be changed:

```sh
tix project edit infra --key ops              # infra-8 is now ops-8
tix project edit ops --name "Operations" --description "on call and the fleet"
```

The old key stays an alias of the project for as long as the project exists. `tix task show infra-8`,
`-p infra`, `project:infra`, a `project:` default in the configuration, a sync mapping and a link to
`/projects/infra` or `/tasks/infra-8` all keep working and answer with the new key, and the browser is sent
on to the new address. Because an old key keeps pointing at its project, no other project can take it: a key
held by another project, current or old, is exit 4, and a key that breaks the key rules is exit 2. A project
can take back one of its own old keys. Deleting a project frees every key it held.

In the browser the key is in the project's settings, and a changed key is confirmed on a second screen that
shows references before and after. In the terminal interface it is the `key` attribute of the project
screen's edit form.

## Marking a project

A project carries a colour and an icon so its rows are recognisable at a glance in the CLI table and in the
browser interface:

```sh
tix project create rv "Review" --workflow review --color violet --icon 🚀
tix project edit rv --color teal --icon RV
tix project edit rv --color "" --icon ""        # clear both
```

`--color` is one of `slate`, `red`, `amber`, `green`, `teal`, `blue`, `violet` or `pink`, or a custom colour as
`#rrggbb` (quote it, the shell reads `#` as a comment); anything else is exit 2. The browser offers the same
choices as swatches plus a colour picker. Each colour scheme draws a custom colour as close to it as stays
readable on that scheme's background, and the CLI table draws it in truecolor when `COLORTERM` says so and as
the nearest palette colour otherwise.
`--icon` is one emoji or a monogram of at most two characters. Both are optional, and a project with neither
renders with those columns empty. They travel with `tix export`/`tix import` and with a project template in a
bundle.

The icon can also be a built-in symbol or an uploaded image:

```sh
tix project edit rv --icon-name rocket          # one of the built-in symbols, listed in --help
tix project edit rv --icon-file logo.svg        # PNG, JPEG, WebP or SVG, at most 64 KiB
tix project icon rv --out copy.svg              # save the uploaded image again
```

A project carries one kind at a time: setting one replaces the other, naming two is exit 2, and any of the
three flags given empty clears the icon. Where a terminal cannot draw them, a built-in symbol shows as a
fallback glyph and its name, and an image as `▣ image`. A built-in icon travels in snapshots and bundles; an
uploaded image does not, and an import that updates a project keeps the image it has unless the record names
another icon.

The `COLOR` column is drawn as a swatch in the project's own colour when the output is a terminal, and as the
bare colour name otherwise.

## Custom fields

Fields are defined per project:

```sh
tix field put rv severity --type enum --option low --option high --label Severity --default low
tix field put rv owner --type actor --required
tix field ls rv
tix field rm rv severity
```

| Flag | Effect |
| --- | --- |
| `--type` | `string`, `text`, `int`, `float`, `bool`, `date`, `datetime`, `enum`, `actor`, `json` (default `string`) |
| `--option` | an enum option, repeatable |
| `--required` | every task in the project must carry a value |
| `--default` | the value a task created without one is given; it has to fit the type and, for an enum, the options |
| `--position` | ordering in the UI |
| `--label` | human label, defaulting to the key |
| `--dry-run` | validate without writing |

Values are set on tasks with repeatable `key=value` pairs:

```sh
tix task add "harden the ingress" -p rv --field severity=high --field owner=ops
tix task edit rv-1 --field severity=low
tix task mv rv-1 working --field owner=alice
```

`tix field put` replaces the whole definition, so repeat `--default` when redefining a field that has one.
A field of type `string`, `text`, `enum`, `actor`, `date` or `datetime` keeps its default as text; any other
type reads it as JSON first, as `--field` does, so `--default 3` is a number and `--default true` a boolean.
The browser's field form carries the same controls, and the API takes `default` on `PUT
/projects/{ref}/fields`.

Marking a field `--required` does not retroactively invalidate existing tasks.

Field definitions used to take `--indexed`. Nothing ever read it, so it was removed. The flag is still
accepted and ignored, with a note on stderr, and an API body, bundle or snapshot that sends `indexed` is accepted
with the key dropped.

## What a custom field filter costs

Filtering on a custom field is always allowed and never an error. It is answered by reading the JSON document on
each candidate row, so the work grows with the number of rows the rest of the filter leaves behind.

Narrow the scan first. A custom field predicate is cheap once the project, status and tag predicates
have already cut the candidate set:

```sh
tix task ls -p rv -s todo -l urgent
```

`tix task ls` has no flag for custom field values, but the filter expression carries them:
`tix task ls --filter 'field.severity:high'` on the command line, `GET /api/v1/tasks?field.severity=high` over
HTTP. See [filtering.md](filtering.md#every-term) and [api.md](api.md).

## Sharing a workflow between installations

```sh
tix bundle export --kind workflow --workflow review --name review-kit > review.bundle
tix bundle import review.bundle --on-collision rename
```

Bundles carry workflows, field definitions, tags, project templates and webhook endpoints: the way of working,
never the work itself. See [migrating.md](migrating.md).

A workflow made from a built-in template travels like any other, key and all. Another installation already has
the same templates, so `tix workflow template apply` there is usually the shorter way to the same workflow; a
bundle is for one you changed after applying it. Importing a `tix-` workflow is allowed, because the key says
it came from a template wherever it was made.
