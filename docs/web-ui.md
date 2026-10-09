# The web interface

`tix serve` puts the browser interface on the same service layer the command line calls, so a screen
can do everything a command can. That is enforced rather than aspired to: the capability registry in
`internal/capability` names a web binding for every operation, and the parity test fails the build
when one is missing.

The pages are server-rendered Go templates with no build step. htmx swaps fragments where a full
reload would lose the reader's place, but every form carries a real `method` and `action`, so a
browser that runs no script submits it normally and follows the redirect.

## The task list

The heading counts what is in front of you: how many are still open, how many are done, how many
projects they span, how many an agent is holding, how many are blocked.

**Filtering.** The box takes the same expression language the CLI takes, documented in
[filtering.md](filtering.md), through the same parser, so an expression that selects a set of tasks
here selects the same set from a shell. A project chip on a row writes a filter into that box in one
click, so there is a Clear control to get back out in one click. Clear keeps the sort, because a sort
is not a filter.

**Deadlines.** The control beside the sort narrows the list to a deadline window: overdue, today, this
week, this month. What it selects is answered as a `due:` term by the same parser the box uses, rather
than as a bound of its own, so the same window is sayable by hand in the box, the selection travels in
the URL like the rest of the query, and a window the language does not have is refused under the box like
any other bad term. Whatever is in the box is ANDed with it, and a `due:` term typed there wins.

A row whose deadline has passed carries an `overdue`
badge, one falling due within the week a `due this week` badge, and a row whose deadline is further out
or absent carries nothing -- a badge on every dated row marks nearly every row in a real backlog. The
badge is a word as well as a colour and links to the listing it belongs to. The task screen says the
same thing beside the date in the Details rail. The `Due` column can be turned off in View, like any
other optional column.

An expression the listing cannot answer is reported under the box, with the expression still in the
box. A filter naming a project, status or handle this tenant does not have is a typo in a control,
not a missing page, and the full-page error screen threw the query away and made the reader retype it
to find out what was wrong. Every refusal is treated alike, whether the expression failed to parse or
named something that does not resolve, because a rule written per term goes stale the next time the
filter language learns to refuse something. The page is still served as a success: it is the listing
that was asked for, and htmx swaps nothing out of a response that is not a 2xx, so an error status
would leave the previous page on screen carrying no message at all. A failure that is not the
expression's fault still fails the page.

**Who has it.** The list carries an assignee column, shown by default, naming the actor by handle and
saying `unassigned` where nobody holds it rather than leaving the cell blank, which reads as a value
that failed to load. The handle is a link that filters the listing to that actor. Handles are
resolved once for the page rather than once per row, so the column costs one lookup pass whatever the
page size.

**The View panel** holds two choices that are really one decision, which columns the listing shows
and which projects it includes. They were two separate disclosures and deciding what was on screen
cost two openings and two page loads. Both are per browser, stored in cookies, and both record what
is *hidden* rather than what is shown, so a project created tomorrow, or a column a later release
adds, appears on its own instead of waiting for somebody to tick it. A filter that names a project
explicitly overrides the hiding, and the page says so rather than quietly returning nothing.

**Lease badges.** A lease is the one thing on a row that changes by itself, so the row says which
state it is in:

| Badge | Means |
| --- | --- |
| `held · handle` | An agent holds a live lease. The title carries who and until when |
| `claim expired · 3h ago` | Somebody claimed it and never came back. It is claimable again. A row claimed more than once also says how many times, and the title names the holder and the instant it lapsed |

`held` is computed from the lease expiry against the clock as the page renders, not from whether the
sweeper has been round yet, so a lease that ran out a minute ago stops reading as held.

`claim expired` cannot be computed that way, and trying to was what made it unreachable in practice.
The sweeper clears the lease columns within a minute of a lease lapsing, so from then on a task an
agent took and stopped answering for looked exactly like a task nobody had ever touched, which is the
opposite of what the badge is for. It reads the durable evidence the sweep leaves on the task
instead, `lease_expired_at` and `lease_expired_by_actor_id`, and treats that evidence as current for
a day (`core.Task.ClaimExpiredRecently`, over `core.LeaseExpiryEvidenceWindow`). A day spans the gap
between one person looking and the next, so a claim that lapsed overnight still says so in the
morning, and it is a full maximum lease, so no lease can outlive its own evidence. The columns
themselves are not cleared when the window passes; the bound is a judgement made at read time, and an
operator can still ask about an older expiry directly. A task claimed again reports the live claim,
because the question the badge answers is whether the work was dropped and left dropped.

The claim count is shown only beside an expiry, where it distinguishes two different situations: a
task claimed seven times whose last claim was dropped is a holder that keeps dying, while the same
seven claims with no expiry is ordinary work changing hands.

`tix claim sweep`, and the sweeper a server runs on a ticker, is what actually returns the task to
the queue. See [agents.md](agents.md) for what a lease is and why the token matters.

**Ticking a task** swaps that one row rather than the page. The completion animation belongs to the
act of completing, not to the completed state, so opening a list of finished tasks animates nothing.
Fifteen checkboxes popping in sequence on load says fifteen things just happened when nothing did.

## List or board

The switch beside the filter bar draws the same tasks either as rows or as columns, and remembers
which per browser (`tix_task_view`). The list is what an untouched install shows. The same switch is
on Settings, and it is the same control: both render one partial over one resolved value, so neither
can show a position the other denies.

It is one selection drawn two ways. The filter, the deadline window, the sort, the page size, the
page position and the project visibility choice are resolved once and apply to both, so switching
changes the drawing and never which tasks are on screen, and one pager walks both. That is the whole
point of putting the board on this screen rather than sending the reader to a different one: the
project board at `/projects/{key}` answers a different question -- one project, no filter -- and it
stays as it is, with the project's settings and danger zone on it.

**Columns come from a workflow, and the list mixes projects.** So a board over several projects is
only drawable when those projects agree about their states, and agreement is decided structurally,
not by name. A name is not an identity: two projects can each run a workflow called `default` whose
states differ, and the state-category vocabulary now has six words where it had three, so two
same-named workflows can differ in ways that decide where a task may go. What is compared is
everything a board draws or a move depends on:

| Compared | Why |
| --- | --- |
| The states, in declared order | Declared order is the column order |
| Each state's key | What a task's status is matched against, and what a drop submits |
| Each state's label | The column heading. A heading misnaming a state for half its cards is dishonest |
| Each state's `terminal` flag | What completing means, and what satisfies a dependency |
| Each state's `category` | What colour and grouping are derived from everywhere |
| The transitions, in declared order | The edges the route finder walks, in the order it enumerates them |
| Each transition's `from` and `to` | Whether a move is legal at all |
| `requires_scope`, `requires_comment` | A move legal here and refused there is not one control |

Not compared, because none of them can change a column or a move: `initial`, `default_lease`, and the
per-state `revert_on_lease_expiry` and `revert_to`. Not compared either, and deliberately: the
workflow's name, key and identifier. Two projects pointing at one stored workflow agree trivially;
two pointing at separately stored workflows that describe the same machine agree too, which is the
ordinary case in a tenant that made a project before it had a shared workflow.

The asymmetry of cost is what decides the borderline cases. Being too strict costs a refusal that
explains itself and is one click from a board that draws. Being too loose costs a card that looks
draggable to a column it can never reach, with the refusal arriving from the service after the reader
has already dropped it. So anything not demonstrably harmless is compared.

**Every card's moves come from its own project's workflow**, resolved per card, never from the
definition the columns were drawn from. On a board whose workflows agreed those are the same answer;
asking the card's own project anyway is what keeps it impossible for a change to the merge rule to
start offering a foreign move. Drag-and-drop reads the card's own move control to decide which
columns light up, so a column outside that list is refused before any request is made.

**When the projects do not agree the board is refused and the list is drawn**, with a notice naming
each distinct workflow and the projects running it. Each group links to the same screen narrowed to
exactly its own projects, filter and sort intact, so a board is one click away. Where two of those
workflows carry the same name, which is precisely the case the merge rule exists for, the entry adds
the workflow's key: two lines both reading `Default` would be the refusal explaining nothing. Drawing whichever
workflow came first would show a subset of the selected tasks without saying so, which is the failure
this screen exists to avoid; the same reason a task whose status is in none of the columns is named
in a notice rather than dropped. The preference is not touched by a refusal, so narrowing draws the
board with no second press of the switch.

Which projects decide the columns is the set the listing may draw from -- the keys an explicit
`project:` filter names, or every project not put away -- rather than the projects the current page
happens to carry. A page is a page: deciding on its contents would merge on page one and refuse on
page three, and creating the first task in a disagreeing project would turn a working board into a
refusal with no visible cause.

A merged board carries more columns than one project's does, so the grid wraps onto further rows of
columns rather than scrolling the page sideways, and at phone width the columns stack. Cards carry
their project's key, icon and accent only on a merged board, where a column holds more than one
project's work.

## Moving a task through its workflow

The status pill on a row is a control. Opening it lists the states that row's own workflow can reach
from where it is, which is per project: a listing mixes projects, and a state legal in one need not
be legal in another.

States that are not adjacent are offered too. The panel spells out the whole route before it is
applied, `todo → doing → review`, and says how many steps it takes, so the reader agrees to the
states it passes through rather than finding them in the history afterwards.

Each step is then an ordinary transition. A three-state route performs three service calls, writes
three audit entries and emits three events, exactly as making those moves one at a time would. That
is the intended behaviour and not an implementation leak: a task that passed through `doing` really
did pass through it, and the trail should say so. If a step is refused after earlier ones have
applied, the message names the state actually reached and says it stopped there, rather than
reporting a move that did not happen.

The panel is a popover. It was an absolutely positioned disclosure first, inside a list that clips
its overflow, so on a low row the choices were cut out of the scrollable area entirely: they took no
pointer events and could not be scrolled to at any position, which reads as a frozen page. The top
layer is outside every clipping ancestor, and `popovertarget` gives light dismissal and Escape with
no script. A browser that does not understand the attribute gets the panel inline, which is the list
of choices it always was.

## Walking a listing

Every listing paged by a cursor renders the same control: which page the reader is on, how many rows
it carries, and a step in each direction. There were six controls before, five of them a bare
paragraph holding a link and one a row of actions, with two different words for the same direction
and no styling on any of them. None said where the reader was and none offered a way back. A listing
that fits on one page renders no control at all, rather than a pair of steps that lead nowhere.

Previous is the part worth explaining. Keyset pagination only goes forward: a cursor names where the
next page starts and carries nothing about where the current one did, so there is no backwards cursor
to ask the store for. The page remembers, in its own URL, the cursors it came through, and Previous
pops the last one off:

```text
/tasks?cursor=C3&trail=C0.C1.C2
```

Nothing about that reaches the domain, the store or the API, and it puts the whole position in the
URL, so a page deep in a listing can be linked, reloaded and bookmarked.

The trail arrives from the address bar, so it is whatever anybody cares to type. Every entry has to
decode as a cursor this build issues, and the whole value has to be within forty entries and 4096
bytes. A value failing any of those is discarded entirely rather than repaired: half a trail would
send Previous to a page the reader was never on, which is worse than sending them back to the first
page, and an unbounded one would lengthen with every page walked. Walking past the bound keeps
working and only shortens the memory, dropping the oldest entries, since Previous is walked from the
newest end.

Changing the filter or the sort discards the trail. Its cursors address positions in one ordered
result set, and under a different filter, or a different ordering, they name rows that were never on
the reader's screen. The filter form submits neither the cursor nor the trail, so submitting one is a
return to the first page without anything having to clear it.

## The task screen

Dependencies name each task this one waits for by reference, title and status, linked to the task,
with the full identifier on the row's title. They read back a raw identifier while the field directly
under them was placeholdered `infra-2`, so the screen disagreed with itself about what identifies a
task. A dependency this reader may not read keeps its shortened identifier and gains no link, so a
dependency they cannot open is still visible as one rather than missing from the list.

## The directory

`/actors` lists everyone and everything work can be assigned to in this tenant: handle, kind, display
name and identifier. Kind is on every row because it is the one thing a handle does not tell you, and
work is assigned to agents as often as to people.

The same listing feeds the assignee field on the task screen. It is a `datalist` rather than a
`select`, so the field suggests without constraining: an actor from another tenant is deliberately
assignable by identifier, and a control that only accepted what it listed would remove that without
saying so. A listing that fails leaves the suggestions empty rather than failing the page, because
the field takes a typed value either way.

The field takes a handle as readily as an identifier. The reference is resolved in the service, so
the browser, the command line and the API obey one rule rather than each transport obeying its own,
and it is classified by shape: a value of exactly the length and alphabet of a generated identifier
is an identifier and is passed through without a lookup, which is what keeps an actor of another
tenant assignable. Anything else is a handle, and one this tenant does not have is refused as not
found, naming the reference, rather than reaching storage, where it used to fail a foreign key and
surface the constraint text to the reader.

The listing answers who is here and never what anybody may do: scopes, roles and token identifiers
are dropped before it leaves the service, so being signed in is the only thing it asks for. It orders
by handle and refuses any other sort rather than returning rows in an order that is not the one asked
for.

On the command line the same directory is `tix actor ls`, and `tix actor show` resolves one
identifier.

## Statistics

`/stats` carries the same figures as `tix stats` and the terminal interface's `S` view, over one
window and optionally one project. [statistics.md](statistics.md) covers what each figure means and
what the leaderboard does and does not measure.

## Display preferences

Everything under Settings is a property of the person reading, not of the tenant, so each one is
stored in a cookie on that browser and reaches no other reader. Two people sharing a tenant are
frequently in different timezones for the same reason they may want different keyboard shortcuts.

Settings carries every one of them. Three are also editable from the screen they affect, because that
is where the decision is actually made: whether a column reads better absent is answerable only by
looking at the listing, and ticking projects off a task list is something you do while watching the
list redraw. Those three are **one control rendered in two places, not two controls** -- one
constructor builds the state, one template renders the form, one function reads the cookie and one
handler writes it -- so changing a preference on either screen is what the other shows. A preference
with two editors has two defaults, two validations and two readings of one value, and the drift that
follows is not noticed until a reader is looking at two screens that disagree.

| Preference | Cookie | Values |
| --- | --- | --- |
| Colour scheme | `tix_theme` | empty follows the system, else `light`, `dark`, `dim` |
| Keyboard shortcuts | `tix_keyscheme` | the shipped schemes, `?` shows the active one |
| Date format | `tix_time_format` | one of the layouts this build renders |
| Timezone | `tix_timezone` | one of the offered zones, empty follows the deployment |
| Advanced screens | `tix_advanced` | whether the configuration and data screens are in the sidebar |
| Drag to move | `tix_drag_move` | whether a board card can be dragged between columns |
| Columns | `tix_columns` | which optional columns each listing leaves out |
| Hidden projects | `tix_hidden_projects` | which projects a listing leaves out |
| Task view | `tix_task_view` | `board` draws the task screen as columns; anything else, including no cookie, is the list |

`tix_advanced` has three states rather than two: shown, hidden, and absent, which leaves the answer
to who is reading. A tenant administrator who has never chosen gets the Configure and Data groups,
because the tenant screen was otherwise reachable by typing its URL and by no other means: the only
entry to it lived behind a preference on a settings page they had no reason to open. Anybody else
gets the short navigation, since every screen in those groups refuses them and an entry leading to a
refusal is worse than no entry. Being on one of those screens expands its group whatever the
preference says, so a reader is never on a page the navigation beside them denies exists. Once the
preference is set, in either direction, it decides.

`tix_columns` is per listing, so Settings offers one picker per listing rather than one global
control: the listings share no column vocabulary, and the columns worth keeping on Tasks are not the
ones worth keeping on API tokens. Each picker is the same form that listing carries beside itself.
Every listing keeps the column naming its rows whatever is chosen, so no choice can put a record out
of reach, and hiding a column withholds nothing, since every value stays on the record's own screen.
Reset returns a listing to its declared defaults, which is not the same as unticking everything: a
column declared off by default is shown by the first and not the second.

`tix_hidden_projects` names each project as well as keying it, because on Settings there is no task
list in front of the reader to decode a key against. A key naming a project that no longer exists --
deleted, or archived since the choice was made -- has no checkbox, since the project it names is
gone, so the control names it in a sentence instead and the next submission of the form forgets it.
Such a key excludes nothing: an unknown project key in a filter is refused as not found by the
service, deliberately, so sending one would answer the task screen with a not-found page and leave
the cookie unreachable from the only screen that could clear it.

`tix_task_view` has two states and no third. The board is the one stored value; absent, empty, stale
or anything another program left behind is the list, which is what an untouched install shows.
Choosing the list stores nothing and expires the cookie, so both switches offer two positions and
neither has an "unset" to show.

`tix_columns` carries a version marker, `v2~`, and names the columns each listing *hides*. Recording
the columns shown cannot tell a column the reader turned off from a column that did not exist when
the reader chose, so every column added afterwards read as one that reader had refused, and the
readers it silenced were exactly those who had used the picker at all. A value written in the older
form is recognised by the absence of the marker and converted rather than read as it stands, which
would mean the opposite: every column a reader put away stays away, every column they kept stays,
and the columns that form could not name are shown. The marker covers the whole value rather than
each column, because the inversion changes what the empty-list sentinel means as well as what a key
means.

An unusable value is never stored and never reaches a page. A format the build does not implement, a
zone that does not resolve, a scheme that is not shipped, an oversized cookie some other program left
behind: each falls back to the deployment's own configuration rather than taking the screen down.

The binary embeds the zone database (`time/tzdata`) rather than trusting the host to carry one, so
the zones on offer are the same on a distroless image as on a developer's laptop, and a reader's
chosen zone is not quietly replaced by the deployment default on a base image with no `tzdata`. The
fallback stays regardless: a value that does not resolve must not take a page down.

### The terminal and the command line keep their own

A browser preference is a cookie on that browser. The terminal interface's settings screen writes the
CLI's own configuration keys instead, because somebody at a terminal already has a configuration file
and a second place to write a timezone down would be a second place to disagree with the command
line. The two surfaces therefore do **not** share a store, and setting a preference in one does not
change the other:

| Preference | Browser | Terminal and CLI |
| --- | --- | --- |
| Date format | `tix_time_format` | `output.time_format` |
| Timezone | `tix_timezone` | `output.timezone` |
| Keyboard scheme | `tix_keyscheme` | `tui.keymap` |
| Colour | `tix_theme`, which palette | `output.color`, whether colour at all |
| Advanced screens, drag to move, columns, hidden projects, task view | cookie | no equivalent |
| Selected-row pulse | no equivalent | `tui.motion` |

So a timezone chosen in the browser does not change what `tix task ls` prints, and one chosen in the
terminal is what `tix task ls` prints and not what a browser renders. See [tui.md](tui.md) for the
terminal screen and [configuration.md](configuration.md) for the keys.

The zone list is a selection rather than every name the system carries. A `select` of six hundred
entries is not a control anybody can use, and a fixed list also bounds how many template sets one
process can be made to parse: the formatting helpers are bound into a template at parse time, so a
per-reader zone has to be a separately parsed set rather than a value handed to `Execute`.

The preference forms deliberately opt out of htmx boosting. `data-theme` lives on `<html>` and a
boosted form swaps only the body, so the cookie changed while the page kept its old scheme, which
reads as a button that does nothing. A full navigation costs one round trip and is correct.

The colour scheme is a separate axis from the tenant accent, which is the same everywhere and covered
in [theming.md](theming.md).

## Static assets

The stylesheet, the scripts and the icon are compiled into the binary and served from memory with a
strong `ETag` derived from their bytes and `Cache-Control: public, max-age=60, must-revalidate`. A
conditional request for an unchanged asset gets a 304 with no body.

The validator is the point. The asset URLs do not change between releases, and the responses used to
carry no validator at all, so a browser cached them heuristically and after an upgrade a reader got
the new markup with the previous release's stylesheet. That happened during review and was reported
as the interface being broken. Each file is fingerprinted independently, so changing one does not
invalidate the others.

## Something to look at

An empty install is a poor demonstration of screens whose whole job is showing work in progress.

```sh
tix demo seed --db /tmp/demo.db          # 90 days by default
tix demo seed --db /tmp/demo.db --days 30 --reset
tix serve --db /tmp/demo.db
```

It writes projects, people, agents, custom field definitions, and tasks carrying descriptions, tags,
priorities, assignees, due dates and comments, with completions spread across the window by several
actors and lead times that vary.

Two of those tasks are claims an agent took and never gave back, one of them picked up and dropped
four times, so the `claim expired` badge and the claim count beside it can be seen without arranging
that state by hand. They are produced by the ordinary claim and sweep calls like everything else
here, and placed against the end of the window rather than on a fixture day, because the evidence
only reads as recent for a day and the fixture days are backdated by months.

A fresh installation is given four starter lists before any command runs. Any of them still holding
no task is dropped, so the projects screen and the project filter name only the lists the backlog
works out of, and a first seed and a re-seed show the same thing rather than ten lists and six. Only
a list with no task at all is removed, so nothing that has been worked in can be lost to a seed.

The summary says what it wrote: projects, actors, custom fields, tasks, completions, comments,
dependencies and `ABANDONED CLAIMS`. `-o json` carries the same counts, the last of them as
`abandoned`.

The history is replayed through the ordinary service calls against a clock the command advances,
not written behind them. That is not fastidiousness: statistics attribute a completion by matching
the transition entry written in the same instant, so data inserted underneath the service would leave
every leaderboard and lead-time figure empty, which is the opposite of the point.

It refuses a database that already holds tasks or users unless `--reset` is passed, exiting 4 like any
other conflict, so nobody discovers this command by finding a few dozen invented tasks in their real
tracker. Point `--db` at a scratch file regardless.

## See also

- [deployment.md](deployment.md) for running `tix serve`, TLS, proxies and the bind guard
- [filtering.md](filtering.md) for the expression the filter box takes
- [theming.md](theming.md) for the tenant accent, and what is deliberately not themed
- [statistics.md](statistics.md) for the figures behind `/stats`
- [tenancy.md](tenancy.md) for what a tenant owns and how isolation is enforced
