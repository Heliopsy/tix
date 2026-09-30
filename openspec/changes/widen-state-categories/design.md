# Design

## The vocabulary, not a colour field

The decision on the table was whether a state gets its colour from a widened category vocabulary or from
a free per-state `Color`. The vocabulary won, for three reasons that a per-state colour cannot answer:

- **Colour means the same thing everywhere.** A reader who learns that red is blocked on one deployment
  reads it the same way on the next. With a free field, one tenant's red is another tenant's green, and
  the hue stops carrying information the moment there is more than one workflow.
- **Monochrome degrades sensibly.** A category has a word. `blocked` is `blocked` on a terminal writing
  no escapes, in a pipe, and under `NO_COLOR`. A hex value has nothing to fall back on.
- **Grouping and colour stay the same axis.** Statistics already group by category. A separate colour
  field would let a state be coloured one way and counted another, which is exactly the class of
  disagreement the `cancelled`/`done` defect was.

The hybrid — a category default with a per-state override — was rejected because the override is the free
field, and a field that is usually unset is still a field that can be set: once one workflow overrides,
the shared meaning is gone for every reader of that deployment.

The cost is that a tenant cannot invent a seventh category. That is the point. The vocabulary is a
contract, and the six were chosen to cover what a tracker's states actually are.

## Six, and why `waiting` is one of them

`todo`, `in_progress`, `done` were there. `blocked` and `cancelled` are forced by the shipped workflow's
own states. `waiting` is the one addition not forced by an existing state, and it is here because
`blocked` without it is ambiguous: a task stalled on another task on this board and a task stalled on a
third party outside it are different facts with different remedies, and a board that renders them alike
makes the reader open each card to find out which. Splitting them is cheap now and a breaking vocabulary
change later.

The empty category stays valid. It is how a state declines to say, and `statsCategoryOf` derives one from
what else the state carries: terminal reads as `done`, the initial state as `todo`, anything else as
`in_progress`. That derivation is unchanged, so a workflow written before categories existed still
reports where it always did.

## What `cancelled` stops being

This is the substantive change. Every place category is consulted was worked out first:

| Consumer | Reads | Effect of recategorising `cancelled` |
| --- | --- | --- |
| `Stats.ByCategory` | category | **Changes.** `cancelled` leaves the `done` row for one of its own. |
| `Stats.Completed`, per-day, lead times, leaderboard | `completed_at` | None. |
| `tasks.completed_at` | `Definition.IsTerminal` | None. |
| `WorkflowDefinition.TerminalStates`, `IsTerminal` | `State.Terminal` | None. |
| Claim path (`claim.go`) | `IsTerminal` | None. |
| Dependency release | `completed_at IS NULL` | None. |
| Board colour, CLI colour, browser colour | category | Changes, which is the point. |
| `web.doneState` | prefers a terminal state in category `done` | Now unambiguous. It used to match whichever of `done` and `cancelled` the workflow listed first, so a workflow declaring `cancelled` before `done` had its tick control cancel tasks. |

The one behavioural change to a number an operator reads is `ByCategory`, and it is a fix rather than a
regression: the `done` row previously answered "how much is finished?" with "how much has stopped
moving?", and those are different questions with different answers. The headline `completed` figure is
untouched and still counts cancelled tasks, because it is defined as "reached a terminal state" and a
cancelled task did. That `completed` also blurs the two is arguable and is called out in
`docs/statistics.md` rather than changed here: it is a different decision, it would move a throughput
number every operator has been reading, and it is not what widening the vocabulary is for.

## Stored workflows are left alone

Three options: migrate stored workflows, re-derive their categories on read, or leave them.

Leaving them is the only one that is honest. A stored workflow is what the tenant wrote. A tenant that
put `category: done` on a cancelled state either meant it or wrote it before there was an alternative,
and tix cannot tell which; rewriting it would change that tenant's statistics without their asking, and
re-deriving on read would make the stored document a lie about what the system does. Both also break the
requirement that a hand-authored workflow keeps working.

So: only `service.BuiltinWorkflow` and `sshd`'s seed change. `EnsureDefaults` is idempotent and creates
the workflow only when it is missing, so an existing installation keeps the workflow it has and a fresh
one gets the new categories. A tenant wanting the new behaviour re-puts its workflow, which is the same
gesture as any other workflow edit.

`internal/service/taskWorkflow` — the test fixture — still declares `category: done` on its cancelled
state, deliberately, and a guard asserts it still reports that way. That is the legacy case under test
rather than described.

## Colour

`CategoryColor` keeps its `(color, bool)` shape, which `Theme.Category` already flattens to the
terminal's declared depth and already answers with a plain style when the theme carries no colour. That
is the same shape `PriorityColor`/`Theme.Priority` and `DueColor`/`Theme.Due` have, and nothing about six
categories needs the third return value the other two carry for boldness.

`blocked` borrows the urgent colour and `cancelled` the muted one, for the reason `DueColor` gives for
overdue borrowing urgent: they say the thing a reader scanning a board needs, and a distinct hue per
category past what a reader can name is a distinction without a difference. The six are still mutually
distinct, and a guard on each surface asserts that rather than trusting the table.

In the browser the rules key off the category rather than the state key. The existing `.pill.status.*`
rules key off the state, which is why a tenant's own state has never been coloured; those are left alone,
and the new `.cat-*` rules are the category-keyed set. `waiting` is the one hue the semantic tokens do
not already carry, so it is declared once as `--cat-waiting: var(--proj-violet)`, which resolves against
whichever `--proj-violet` the winning theme scope set and so gives every shipped scheme its own tuned
violet from one line.

The keybinding schemes (`Schemes()`, `TestEveryShippedSchemeIsUsableAndCollisionFree`) are unrelated:
they bind keys, not colours, and no colour in this codebase is per-scheme.
