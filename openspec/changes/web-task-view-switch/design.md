# Design

## What workflow equality means, and why that

Two projects may share a board only when their workflows are *the same state machine as far as a
board is concerned*. The comparison is structural, over the definition rather than the record:

| Compared | Why |
| --- | --- |
| States, in declared order | Declared order is the column order, so a different order is a different board |
| Each state's key | It is what a task's status is matched against and what a drop submits |
| Each state's label | It is the column heading. A heading that misnames a state for half its cards is drawn dishonestly |
| Each state's `terminal` flag | It decides what completing means and what a dependency is satisfied by |
| Each state's `category` | It is what every surface derives colour and grouping from, and the vocabulary has just widened |
| Transitions, in declared order | They are the edges `core.Routes` walks, and its order decides which equal-length routes a menu offers |
| Each transition's `from`, `to` | They decide whether a move is legal at all |
| Each transition's `requires_scope`, `requires_comment` | A move legal for this reader in one project and refused in the other is not one control |

Deliberately **not** compared, because none of them can change a column or a move: `initial` (where a
new task starts, which the board never draws), `default_lease`, and the per-state lease-reversion
fields `revert_on_lease_expiry` and `revert_to`. Including them would refuse boards that are
perfectly drawable.

Deliberately **not** compared, and this is the point: the workflow's name, its key and its stored
identifier. A name is not an identity. Two projects pointing at one workflow row agree trivially;
two pointing at separately stored rows that happen to describe the same machine agree too, which is
the common case in a tenant that created a project before it had a shared workflow. Two rows both
called `default` that disagree about a state do not agree, however alike their names look.

The asymmetry of cost is what settles the borderline fields. Being too strict produces a refusal with
an explanation and a one-click route to a drawable board. Being too loose produces a card offering a
move that the service then refuses after the reader has already dropped it. So anything that could
not be shown harmless is included.

It is implemented as a canonical string (`workflowShape`) rather than a field-by-field comparison,
because grouping needs a map key and a shape is the key. The string is never shown to a reader and
never stored.

## Which projects decide the columns

The candidate set is the projects the *listing may draw a task from*: the keys an explicit `project:`
filter names, or every project this reader has not put away, minus anything the filter excludes
either way. It is read off the assembled `core.TaskFilter`, so visibility, an explicit filter and a
negated `-project:` term are all handled by one rule.

The alternative -- decide from the projects whose tasks are on the current page -- was rejected. A
page is a page: it would merge two workflows on page one and refuse on page three, an empty page
would have no columns to draw at all, and creating the first task in a project whose workflow
disagrees would turn a working board into a refusal with no visible cause. Deciding from what the
listing *selects* means the board a reader sees and the board they will see after paging are the same
board.

The cost is a refusal caused by a project that contributes no task to the page. That is the honest
answer: the listing is selecting from that project, and if it holds no tasks today it will tomorrow.

## What happens when they disagree

The board is refused, the list is drawn, and the screen says why: one line naming the problem, then
one entry per distinct workflow naming the projects that run it. Each entry links to the task screen
narrowed to exactly its own projects, carrying the reader's filter, deadline window, sort and page
size, and dropping the cursor and its trail -- which address positions in the wider result set and
name rows this reader was never on, exactly as the filter form drops them.

The options rejected:

- **Draw the first workflow's board anyway.** It shows a subset of the selected tasks with no
  statement that it has, and offers the other projects' cards columns they can never reach. This is
  the defect the whole change is about.
- **Draw one board per workflow group, stacked.** Tempting, and the reason against it is paging: the
  one page of tasks would be split across boards by a rule the pager cannot express, so the second
  board is of an arbitrary fraction of its own projects' work. It also multiplies the columns on
  screen without bound. The narrowing links give the same outcome one click away, each one a whole
  board of its own page.
- **Silently fall back to the list.** The reader pressed Board and got a list. A control that does
  nothing visible is a broken control.

The preference is not changed by the refusal. Once the reader narrows, the board they asked for is
what they get, with no second press.

## Per-card moves

`buildBoard` takes the per-project workflow map the list already builds and resolves each card's
routes from `moves[task.ProjectID]`, never from the definition the columns came from. On an agreed
board the two answer alike by construction, so this is not a correctness fix for today -- it is what
makes it impossible for a future loosening of the merge rule to start offering a card a foreign move.
The affordance is the assertion: the card's `<select name="route">` holds exactly its own project's
routes, `live.js` reads that select to decide which columns light up as drop targets, and a column
outside it is refused before any request is made.

Multi-hop stays where `multi-hop-transitions` left the board. A drop names a column and nothing else,
so drag applies the single-hop route whose value is that state key; the card's Move control offers
multi-hop routes spelled out, the same control the project board has.

## Where the preference lives

A cookie, `tix_task_view`, with the other per-browser display preferences: how somebody reads a
shared queue is a property of the reader, not of the tenant. It stores `board` and stores *nothing*
for the list, so the default and "no cookie" are one state and going back to the default leaves
nothing behind -- the same shape as "Show all" projects and "Reset" columns. Any other value reads as
the list, so a stale or planted value degrades to what an untouched install shows.

The control is on the task screen rather than on the settings screen, for the reason the project
visibility control is: it is a decision about the screen in front of the reader, made while reading
it. The cookie is documented in the settings table either way.

## Rendering

The board reuses the project board's markup and classes -- `.board[data-drag]`,
`.column[data-state]`, `.card[data-task]`, `details.card-move` with a `select[name=route]` -- so
`live.js` needs no knowledge of which screen it is on, and so the two boards cannot drift in how a
drag behaves. One real change there: the post-move refresh re-fetches `pathname + search`, because on
the task screen the query *is* the selection; a project board has no query to lose.

A merged board carries more columns than the single-project board was designed for, and its cards
come from several projects:

- The grid stays `repeat(auto-fit, minmax(...))`, with a narrower track minimum when merged, so many
  columns wrap onto a second row of columns rather than overflowing the page sideways.
- At phone width the existing rule stacks the columns one per row, merged or not.
- Each card carries its project's key, icon and accent only on a merged board. On a single project's
  board the heading already said it and repeating it on every card is noise.
