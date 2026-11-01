# The web interface

`tix serve` puts the browser interface on the same service layer the command line calls, so a screen
can do everything a command can. That is enforced rather than aspired to: the capability registry in
`internal/capability` names a web binding for every operation, and the parity test fails the build
when one is missing.

The pages are server-rendered Go templates with no build step. htmx swaps fragments where a full
reload would lose the reader's place, but every form carries a real `method` and `action`, so a
browser that runs no script submits it normally and follows the redirect.

## The sidebar

On a window 900px or wider the sidebar is a column, and its right edge can be dragged to make it
anywhere from 180px to 480px wide. The edge is also a keyboard control, a vertical separator in the
tab order: the arrow keys move it 16px (64px with Shift), Home and End jump to the limits, and Enter
or a double-click puts it back to the default width.

Dragging it below 120px, pressing the left arrow at the minimum, or pressing the button beside the
brand collapses it to a 64px rail holding the logo, the navigation icons, the Settings icon, the
scheme glyph and a sign-out icon. The labels are clipped rather than removed, so every entry keeps its
accessible name, and each gets a tooltip while the rail is showing. The same button, or the right
arrow on the edge, expands it to the width it had.

Both are kept in `localStorage`: `tix.sidebar.width` holds a number of pixels and
`tix.sidebar.collapsed` holds `true`, and each key is removed when it holds the default. Neither says
anything about anybody's work, which is the rule for what may live there (see Show completed under
Lanes). `assets/sidebar.js` loads in the head, before the body paints, and applies them to the root
element, so a reload never draws the default sidebar first. Without JavaScript neither control is
shown and the sidebar is its default width. Below 900px neither applies: a phone gets the brand bar
and its menu whatever a desktop chose.

The page content starts the gutter's width from the sidebar on every screen. It used to be centred in
the space beside the sidebar, under a width cap that differs between a listing and a form, which put
its left edge somewhere different on almost every screen; it is anchored now, and a screen narrower
than the window leaves the spare width on its right.

## The task list

The heading counts what is in front of you: how many are still open, how many are done, how many
projects they span, how many an agent is holding, how many are blocked.

**Filtering.** The box takes the same expression language the CLI takes, documented in
[filtering.md](filtering.md), through the same parser, so an expression that selects a set of tasks
here selects the same set from a shell. A project chip on a row writes a filter into that box in one
click, so there is a Clear control to get back out in one click. Clear keeps the sort, because a sort
is not a filter.

**Adding a task.** The one-line box and Add with details both create the task and leave you on the
listing, filter, sort, view and page as they were. The toast says `task created` and links the new task,
and its row or card is ringed. Settings has "Open a new task" for the other way of working, where each add
is opened to write it out in full. That used to be the default; a browser that never touched the
preference now stays, and one that turned it on keeps opening.

**Repeating tasks.** The add form's details and the task screen's Edit card carry a Repeat field taking
a rule such as `every week` or `3 days after done`, emptied to stop the task repeating. The task screen's
About card states the rule, links the occurrence it created once the task is finished, and before that
gives the date finishing it now would make the next one due, or says that this is the last occurrence.
A recurring row or card carries a `↻` marker with the rule beside it, linking every recurring task. The
Repeats field under View turns it off. Completing one from the browser creates the next exactly as the
command line does; see [features.md](features.md).

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
explicitly overrides the hiding, and the page says so rather than quietly returning nothing. Both are
also on Settings, where the same two forms are rendered over the same stored values rather than
copied, so the panel here and the screen there cannot come to disagree.

The task fields apply to every drawing of the tasks, not only the list: a board or lane card draws its
star, status (on a lane card), priority, tags, repeat rule and reference only when that field is ticked.
Star and Repeats are fields like the others, both on by default. Hiding Star removes the toggle and
nothing else; starred tasks are still listed first, and the task screen keeps its star. A lane card's
more actions menu (below) is drawn whatever is hidden, so hiding a field never takes its control away.

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

## List, board or lanes

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

A merged board carries more columns than one project's does, so the board is one row of columns
scrolled sideways inside its own frame rather than scrolling the page sideways. At phone width it
snaps one column to the screen. It does not wrap onto further rows and does not stack: a column is a
fixed-height scroller, and either arrangement grows past that height and runs over whatever the page
draws beneath the board. Cards carry their project's key, icon and accent only on a merged board,
where a column holds more than one project's work.

## Lanes

The third position of the same switch draws the page one lane per project, the way Google Tasks
draws one column per list. It is the same selection again: one filter, one page, one visibility
choice and one pager, except that lanes, like the board, are exempt from the list's open-work default,
because a finished task is drawn in its lane, ticked.

Lanes follow the order the screen lists projects in, and a lane keeps the listing's sort. A project
gets a lane when a task on this page is in it, or when a `project:` term names it, in which case an
empty lane says so. No other empty lane is drawn: a tenant with hundreds of projects would otherwise
draw hundreds of columns of nothing, most of them because their tasks are on another page. A task
whose project the screen does not list is named under the lanes rather than dropped.

The lane is the project, so the status moves onto the card as a badge: the state's label, coloured by
its category, which is the one part of a state a tenant cannot rename.

**The tick completes and reopens.** On an unfinished card it considers the shortest route from the
task's state into each terminal state of the done category. On a finished card it considers the
routes back to an open state that take the fewest steps: in the built-in workflow that is `done →
todo`, so unticking a card simply reopens it. A route is usable when no step of it needs a comment
and the reader holds the scope to move tasks and every scope a step requires.

- One candidate, usable: the tick is a box that applies it. Its label spells out any state it passes
  through.
- More than one: the tick opens a menu of the usable ones, each naming its journey. It is never a box
  in that case, even if only one of them is usable, because picking for the reader writes a history
  nobody chose.
- None usable: the circle is dashed and opens a panel that says why in words (no route, a comment is
  needed, a scope is missing, the reader cannot move tasks) and links to the task. It used to be a
  link whose reason sat in a tooltip, which a touch screen never shows.

**The badge and the priority are menus**, the same popover controls a list row has. The status menu
offers every state the card's own workflow reaches from here, leaving out routes that need a comment
or a scope the reader lacks, since a lane has nowhere to type one. The priority menu offers every
priority and marks the current one; a normal priority is drawn faintly so it can still be raised. A
reader who may not move tasks sees the badge as a label, and one who may not write tasks sees the
priority as the filter link it was, normal not drawn at all.

**Everything happens on the card.** Each of those forms is a plain post that works without
JavaScript and returns to the listing at the card. With htmx it replaces that card and nothing else:
the response is the redrawn listing and `hx-select` takes the one card out of it, so the card is drawn
by the same template as every other. While the request is out the card dims and takes no second press.
Focus returns to the control that was pressed, on the new card. A refusal, say because somebody else
moved the task meanwhile, leaves the card where it is and says why underneath the title rather than
swapping the error page in, which used to make the card disappear.

**More actions.** Every lane card has three dots in its corner, drawn whatever the View menu hides.
They open a menu with the whole task in it: open the task (in the dialog when Task opens in is set
to Dialog), copy its link or its reference, complete or reopen it along the tick's routes (or the
tick's reason when it offers none), move it to any state the status menu offers, follow a link to the
task for a move that needs a comment, change the priority, star or unstar it, rename the title in
place, set the assignee (the field suggests the tenant's handles), add and remove tags, and delete it.
A reader sees only what their scopes allow: moves need `task:transition`; priority, rename,
assignee and tags need `task:write`; delete needs `task:delete`. A reader who may only read gets
open, copy and their star.

Each item is a form on an existing route or a link, so it works without JavaScript and returns to
the listing at the card, and with htmx it swaps the card like the controls above, dimmed while it
waits and with a refusal said on the card. The edit, tag and restore routes return to a posted `next`
for this, exactly as moves already did; without one they go to the task screen as before. A delete is
soft, the card goes, focus moves to the next card in the lane, and the toast offers Undo, which brings
the card back on the same listing. Rename starts the same in-place editor a title click does with
Clicking a title set to Edits the title, whatever that setting says; without JavaScript it is a link
to the task. A due date is not offered: the web has no form that sets one.

The menu behaves as the badge's does: Escape closes it and puts focus back on the dots, no shortcut
acts while it is open, and a live refresh waits for it. It takes focus when it opens, and the arrow
keys move through it (Home and End go to its ends), in every lane menu; a text field keeps its own
arrows. On a touch screen the dots, the fields and every row answer a 40px press.

A live refresh holds back while a lane request is in flight or any menu in the listing is open, and
puts focus back on the same control when it lands. It also hands what it drew to htmx: it writes the
listing with `innerHTML`, which htmx never sees, so before this every form a refresh had drawn posted
as a whole page, and the second tick on a card reloaded the screen and threw the reader to the top.

On a phone (below 640px) the lanes are one row that snaps a lane at a time, each lane 86% of the
screen wide so the next one shows its edge, and each lane is as tall as its cards so the page, not the
lane, scrolls down. It was a frame of fixed height whose lanes scrolled inside it, three scroll axes
for one thumb, and the guessed height cut the last card off below the fold. On a touch screen the
tick, the badge and the priority each answer a press 40px tall, and the menu rows are 40px.

**Show completed** hides finished cards in every lane. It is kept in the browser's `localStorage`
rather than a cookie, as a single key, `tix.lanes.showCompleted`, which holds `false` when finished
cards are hidden and is removed when they are shown. The sidebar's width and collapsed state are the
only other keys there (see The sidebar). Nothing about work goes there: no project key, no task
reference, no count. Screens are served `no-store` so that a page does not outlive the
session on a shared machine, and browser storage is the same disk with no logout to clear it, so only
a choice that says nothing about anybody's work may live in it. The script that applies it loads in
the head, before the body paints, and sets a class on the root element, so hidden cards never flash
in and a live refresh of the lanes does not bring them back. Without JavaScript the toggle is not
shown and finished cards are.

## Arranging columns and lanes

Every column of a board, every lane and every heading of a grouped list carries three dots. They
show while the pointer is over the heading or focus is inside it, and always on a touch screen. The
menu behind them acts on that group:

- **Star** puts the group before every unstarred one. A starred heading carries a star.
- **Collapse** draws the group as a narrow strip with its name and count and no cards. A collapsed
  column still takes a card dragged onto it.
- **Move left** and **Move right** (up and down in a list) swap the group with its neighbour. A
  starred group moves among the starred ones and the rest among the rest.
- **Hide** stops drawing the group. **Hidden: N** at the end of the view, and the View menu, list
  every hidden group with a Show button; the View menu also resets the whole arrangement.
- A project heading opens the project, shows only its tasks and adds a task to it. A status heading
  shows only that status and adds a task in that state, to a project the board draws.

The arrangement is yours and kept with your account on the server, not in the browser, so it
follows you to any browser and device, to `tix view` on the command line and to the terminal board,
and changes nothing anybody else sees. It is kept per view: `board` is the task screen grouped by
status (the board and the list grouped by status), `lanes` is the task screen grouped by project
(the lanes and the list grouped by project), and `project:KEY` is one project's own board. A lane
is kept by the project, so renaming a project's key keeps it.

Every action is a form, so it works without scripting. With scripting it redraws the view in place
and puts focus back on the same heading's dots, or on Hidden after a hide. While a menu is open the
keyboard shortcuts stand down and a live refresh waits; Escape closes only the menu.

The list can group its rows: View, **Group the list**, or Settings, Listings. The grouping splits the
page being shown, so paging is unchanged and a group can run on to the next page.

## Starring a task

Every task the browser draws carries a star: beside the tick on a list row or a lane card, before the
reference on a board card (the task screen's board and a project's own board alike), and beside the
title on the task screen. Pressing it stars the task, pressing it again unstars it, and the reader lands back on
the screen they pressed it from, filters and all, at the row they pressed.

A starred task comes first. The list puts the reader's starred tasks above everything else and keeps
the chosen sort inside each group, and each board column and each lane does the same, because they
are the same listing split by state or by project. Paging stays exact across the point where the starred tasks end.

A star is personal. It belongs to the reader who set it, so two people sharing a queue each see their
own stars first and never reorder each other's screen. It is kept on the server, so it follows the
reader into the command line and the terminal interface, and starring in one tab refreshes the
reader's other open task screens through the same live updates any other change uses.

The button is a plain form posting to `/tasks/{ref}/star` or `/tasks/{ref}/star/remove` with the
CSRF token, so it works with no script. It is a real button, `aria-pressed` says whether the task is
starred, and its name says what pressing it does: "Star infra-2" or "Unstar infra-2". The icon is an
outline until pressed and filled after, so the state is a shape and not a colour alone. Starring
needs only what reading the task needs.

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

## Changing a project's key

The project board's settings carry the key beside the name and the description, with an (i) that says what a
change does. Saving a different key does not change it straight away: the next screen names the old and the
new key and shows two of the project's references before and after, such as `infra-1 → ops-1`, and the key
changes only when that is confirmed. The confirmation carries the rest of the form, so the name or
description changed in the same save are kept, and so is the icon. A browser cannot carry an uploaded file to
a second screen, so when the same save uploaded an icon image the confirmation asks for the file again. It needs
no script.

The old key stays an alias of the project for as long as the project exists. Opening `/projects/infra`,
`/projects/infra/fields`, `/tasks/infra-8` or a screen under it answers `301 Moved Permanently` to the same
screen under the new key, query kept, so bookmarks and pasted links keep working. A form posted to an old
address is served where it was sent, so a tab left open across the rename still saves. The text of task bodies
and comments is not rewritten: a reference typed there stays as it was typed, and still resolves.

## The task screen

The link above the title goes back to the listing the task was opened from, and says which: "Back to
tasks" for the task list, with its filter, sort and page, or "Back to the infra board" for a project
board. A list row carries the listing in its link; anything else, a board card, the toast, a subtask, is
answered from the page that linked here. Every save on the screen returns to it with the listing still
attached, so the link still goes to the same place after a tag, a comment or an edit, and deleting the
task returns there too.

The About card says whether you starred the task, in words, links the task it is a subtask of, and shows
when it started and finished alongside when it was created and last changed.

When a page fails, the error screen offers the page that linked to it and the task list, each named for
where it goes. It used to offer "the dashboard", which was `/`, a redirect to the projects list.

Dependencies name each task this one waits for by reference, title and status, linked to the task,
with the full identifier on the row's title. They read back a raw identifier while the field directly
under them was placeholdered `infra-2`, so the screen disagreed with itself about what identifies a
task. A dependency this reader may not read keeps its shortened identifier and gains no link, so a
dependency they cannot open is still visible as one rather than missing from the list.

## Copying a task

**Copy to…** at the right of the task screen's title bar, offered to a reader who may write tasks, opens a
small form: the project the copy goes to, the reader's own project selected, the title ready to edit, and
what else to bring. The copy is a new task with its own reference, in that project or the same one. Its
description, priority, tags, assignee, due date and repeat rule come with it, and so do the custom fields
the target project defines. It starts in the target workflow's first state unless **Keep the status** is
ticked and the target has a state of that name. **Comments** copies each comment as a new one by you that
names who wrote it and when, **Subtasks** copies the whole live subtree under the copy, and
**Dependencies** makes the copy wait for what the original waits for. **Link the copy to this task**, ticked
by default, records where the copy came from: its About card then shows **Copied from** with a link back.
Stars, claims, history and artifacts never travel.

The form is laid out like New task: the title is the large field, the project sits beside it, the options
are one group of checkboxes, and Cancel and Copy close the form at the bottom right, with Ctrl+Enter (Cmd+Enter
on a Mac) to copy and Escape to cancel. A copy that is refused, such as one into a project that no longer
exists, comes back with every choice kept and the reason under the field it is about.

When tasks open in a dialog the form opens in the same dialog. Copying returns to the original with a
toast linking the copy, or opens the copy when the "Open a new task" setting is on, exactly as adding a
task does. A lane card's three dots menu offers **Copy to…** too, opening the same form over the lanes
when tasks open in a dialog; copying from there returns to the lanes, filter and all, with the toast. The
command line has the same operation as `tix task copy`, and the API as `POST /api/v1/tasks/{ref}/copy`.

## Acting in place

Every action on a list row and on the task screen answers where it was taken. A row's star, status
and priority swap that row and nothing else, as the tick always did, and a lane card's star swaps that
card, as its other controls already did; a star, move, priority, tag or
comment on the task screen swaps the page content and leaves the sidebar alone. The list keeps its
scroll position and the control you used keeps focus. The element the forms sit in declares this
with `data-inplace` (`templates/tasks.html`, `templates/task.html`) and `assets/feel.js` reads it at
submit; with scripting off the forms post and redirect exactly as before. In the task dialog the
dialog's own body is what its forms swap, so nothing there declares it.

The answer is drawn before the server gives it: the star fills, a moved row's pill names its new
state, a priority chip shows the one chosen, and the row fades slightly while the request is out (only
after 80ms, so a quick answer does not flicker). A refusal or a lost connection puts back what was
there and says why in a toast.

**The toast** sits in `#toast`, which every signed-in screen carries even when empty so an in-place
answer can bring its toast with it. It leaves on its own after five seconds, ten when it offers Undo,
and stays while the pointer or focus is on it. Undo is offered for a move whose way back the task's
workflow allows (directly or through other states; a workflow with no way back offers none), for a
priority change, and for a soft delete, which it restores. It is an ordinary form, so it works with
scripting off. The redirect carries it as `flashundo`, which every address the screen builds from the
request drops, as it drops `flash`.

Undo is a browser convenience over ordinary operations, not an operation of its own, so the command line
and the API undo the same things directly: a move with `tix task mv` (or `--hops` when the way back passes
through another state), a priority with `tix task edit --priority`, and a soft delete with
`tix task restore`, `POST /api/v1/tasks/{ref}/restore`. `tix task ls --include-deleted` finds what was
deleted. A delete with `--hard` has no way back on any surface.

**Live updates** never take the screen from you. A refresh already waited for a request in flight
or an open menu in the region it replaces, and now also for a row drawn ahead of its answer
(`is-pending`). When it lands it keeps the scroll position of boxes inside the region: the task list
scrolls inside its own box, and every update put it back on its first row. Focus is put back by the
same rule everywhere (`tix.focusMemo` in `decide.js`, used by live updates, lane cards and in-place
actions): the nearest id and the control's `data-focus` name, and a choice made in a menu counts as
the button that opened it.

**Moving between screens.** A link into a task or a listing starts on the mouse press rather than the
release, which takes 80 to 150ms off each one (main button, no modifiers, never touch or keyboard; the
click that follows is swallowed). Screens cross-fade over 150ms where the browser has view
transitions, and not at all for a reader whose system asks for reduced motion. Back to the list
restores where it was scrolled: the list scrolls inside its own box, which htmx's history does not
remember, so the positions are kept in `sessionStorage` per address. The filter box applies as you
type, 300ms after the last key, and the deadline and sort selects apply when changed; Apply stays for
a browser without scripting. The list, board and lanes switch swaps the page instead of reloading it.

**Keyboard shortcuts** are behind "?" on every screen, and behind the Keyboard shortcuts button at the
foot of the sidebar for anyone who does not know to press it. The overlay used to be looked up once
when the page first loaded, so after the first boosted navigation "?" opened a copy nobody could see.

## The command palette

ctrl+k, or command+k on a Mac, opens the command palette on every screen, in every key scheme and
from inside a text field too; the Search button under the brand in the sidebar, and in the bar on a
phone, opens it as well. It is one search box over four groups:

- **Tasks**, searched on the server once something is typed: the query is read by the same filter
  language as the filter bar, so `is:starred deploy` narrows the way it does there, and a half-typed
  term is searched as plain words. A ref, in any case, puts its task first. Ten at most, most
  recently changed first.
- **Projects**, read once when the palette opens, each going to its board.
- **Screens**, every destination the sidebar shows (the Configure and Data ones only with Advanced
  screens on) and every section of Settings.
- **Actions**: New task, New task in a project, New project, Open the filter, Show tasks as a list, a
  board or in lanes, Switch the colour scheme, Keyboard shortcuts and Sign out. Each one presses a
  control the screen already has; from a screen without it, the palette goes to the one that has it.

Screens and actions are matched as you type with a forgiving match (`stg` finds Settings) and draw the
moment the palette opens, with no request behind them. Nothing is asked of the server for tasks until
something is typed, and then only 150ms after the last key. With nothing typed, the last things chosen
in this browser come first under Recent, along with recently viewed tasks when the sidebar keeps them.

Up and Down move through the options, Enter chooses, Escape closes and focus goes back where it was.
A task chosen here opens the way **Task opens in** says: in the dialog over the page, or as its own page.

The palette reads `GET /palette/tasks?q=` and `GET /palette/projects`, two JSON routes behind the
session and the tenant like every screen. They are not a new capability: over the API the same
questions are `GET /api/v1/tasks?q=...&limit=10`, `GET /api/v1/tasks/{ref}` and `GET /api/v1/projects`.
The terminal interface has its own palette on the same ctrl+k.

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

## Activity

`/activity` is the tenant's audit log as a feed, newest first, grouped by record, and it updates as
events arrive. It is the browser's `tix audit ls` and the terminal's history view. The filter bar is a
plain `GET` form, so a narrowed feed is a link somebody can send: `q` searches the records (a task
reference such as `infra-5` works), `kind` picks a kind of record (tasks, comments, artifacts,
projects, workflows, users, memberships, sign-ins, API tokens, webhooks, the tenant), `source` the
surface a change came from (`web`, `cli`, `api`, `tui`, `system`), and the actor's name on any row
narrows to that actor. Each active filter is a chip that removes itself, and the live refresh keeps the
filter rather than widening back out. On the command line the same narrowing is
`tix audit ls --filter "kind:task source:web"`; [filtering.md](filtering.md) has the expression.

## Server administration

Three screens in the Configure group answer for the installation rather than the work in it, and each
is the browser's view of a command. `/admin/status` is `tix status`: what the installation holds and
which servers are running against it. `/admin/connections` is `tix connection ls` and `kill`: the live
event streams and SSH sessions this server holds, and ending one. `/admin/ssh-keys` is `tix user key`:
the public keys the SSH listener accepts, enrolled and revoked. [deployment.md](deployment.md) covers
the listener and live connections.

## Choosing a colour

Every place the browser asks for a colour, a project's on the new project form and in its settings, and
the tenant's accent on the tenant screen, uses one control: the predefined colours as swatches, none or the
default where the colour is optional, and Custom, with a colour picker, a `#rrggbb` box and a preview. The
box alone is enough with scripting off. A custom project colour is stored as `#rrggbb` in the same field a
palette name is, and is drawn wherever a palette colour is, in a shade per colour scheme that clears 4.5:1
on that scheme's page and surface, the same floor the palette is tuned to. A colour that already clears it
is drawn exactly as chosen.

## Deleting a project

Delete project in a board's danger zone opens a confirmation screen at `/projects/{key}/delete`; nothing
is deleted by the button itself. A project with no tasks gets one button. A project with tasks, soft
deleted ones counted, offers two choices:

- **Move them to another project.** A select lists the other live projects, and the screen shows the
  status mapping that target implies: the same state when its workflow has it, otherwise its first state
  of the same category, otherwise its initial state. Choosing another target redraws the mapping (with
  scripting off, through the Show the mapping button). Each task gets the target's next number and keeps
  its comments, attachments, tags, dependencies, stars and history. The reader lands on the target's board.
- **Delete them with the project, permanently.** The project key has to be typed. Nothing is restorable
  afterwards: project deletion is permanent, and Archive is the reversible way to put a project away.

The command line spells the same choice `tix project rm KEY --move-tasks-to OTHER` or `--delete-tasks`, and
the API `DELETE /api/v1/projects/{ref}?move_tasks_to=OTHER` or `?delete_tasks=true`.

## Display preferences

Every display preference under Settings is a property of the person reading, not of the tenant, so
each one is stored in a cookie on that browser and reaches no other reader. The Second factor and
Database sections are not preferences: the first is your enrolment, kept by the service and the same
from every surface, and the second only shows what this server is talking to. Two people sharing a tenant are
frequently in different timezones for the same reason they may want different keyboard shortcuts.

Settings carries all but one of them. Three are also editable from the screen they affect, because
that is where the decision is actually made: whether a column reads better absent is answerable only by
looking at the listing, and ticking projects off a task list is something you do while watching the
list redraw. Those three are **one control rendered in two places, not two controls** -- one
constructor builds the state, one template renders the form, one function reads the cookie and one
handler writes it -- so changing a preference on either screen is what the other shows. A preference
with two editors has two defaults, two validations and two readings of one value, and the drift that
follows is not noticed until a reader is looking at two screens that disagree.

The exception is the colour scheme, whose only control is the switch at the foot of the sidebar, on
every signed-in screen. It is the preference a reader changes while looking at the screen that made
them want to change it, which is the same argument as the three above, and it is the one that could
be made small enough to live beside the Settings link. It cycles: one press posts the next of System,
Light, Dark and Low contrast and returns to the page it was pressed on, so it stays a button rather
than becoming a select with an Apply beside it, which is what a four-position control costs with
scripting off. Its accessible name says both the scheme in force and the one a press moves to, since
the glyph says neither and a `title` never reaches a keyboard user who does not hover. Settings names
the current scheme and points at the switch rather than carrying a second form.

A shortcut never takes a key from the control it is pressing. Enter on a focused button or link, and
Space on a focused button, do what they do natively; the `open` shortcut is bound to Enter, and it
used to claim the key everywhere, so no button on the page could be pressed from the keyboard. While a
status or priority menu is open no shortcut acts at all, so Escape closes the menu rather than also
going back a page.

| Preference | Cookie | Values |
| --- | --- | --- |
| Colour scheme | `tix_theme` | empty follows the system, else `light`, `dark`, `dim` |
| Keyboard shortcuts | `tix_keyscheme` | the shipped schemes, `?` shows the active one |
| Date format | `tix_time_format` | one of the layouts this build renders |
| Timezone | `tix_timezone` | one of the offered zones, empty follows the deployment |
| Advanced screens | `tix_advanced` | whether the configuration and data screens are in the sidebar |
| Drag to move | `tix_drag_move` | `0` stops board cards being dragged between columns; absent or anything else lets them |
| Open a new task | `tix_open_after_add` | `1` opens the new task after adding. Absent or `0` stays on the listing. An empty value, which earlier builds wrote for "on", also opens it |
| Hover effects | `tix_hover` | `0` turns hover highlights off and puts `data-hover="off"` on `<html>`; absent or anything else leaves them on |
| Columns | `tix_columns` | which optional columns each listing leaves out |
| Hidden projects | `tix_hidden_projects` | which projects a listing leaves out |
| Task opens in | `tix_task_open` | `dialog` opens a task followed from a listing in a dialog over it; absent or anything else opens its own page |
| Clicking a title | `tix_title_click` | `edit` makes a click on a listed title rename it in place, with an open link beside it; absent or anything else opens the task |
| Task view | `tix_task_view` | `board` draws the task screen as columns, `lanes` as one lane per project; anything else, including no cookie, is the list |
| List groups | `tix_list_group` | `project` or `status` draws the list under a heading per project or per state; anything else, including no cookie, is one ungrouped list |
| Language | `tix_lang` | `en` or `bg` draws every screen in that language; anything else, including no cookie, follows the browser's `Accept-Language` |
| Colour rows by | `tix_tint` | What the stripe down a task row and the edge of a board card stands for: `status`, `priority` or `none`. Anything else, including no cookie, is the project's own colour |

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

`tix_task_view` has three positions and no unset one. The board and lanes are the stored values;
absent, empty, stale or anything another program left behind is the list, which is what an untouched
install shows. Choosing the list stores nothing and expires the cookie, so both switches offer three
positions and neither has an "unset" to show.

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

### The settings screen

Settings is six sections (Appearance, Listings, Keyboard, Dates and times, Second factor, Database)
with a navigation of plain anchors to each. Where the settings block has room the navigation is a
column that stays in view beside the sections; where it does not, it is a wrapped row of links above
them. The choice is a container query on the block rather than a viewport breakpoint, because the
sidebar is resizable and collapsible and the room beside it is what decides. The section last jumped
to is marked in the navigation through `:target`, with no script.

Each setting is a row: its name and one line saying what it does on the left, its control on the
right in a column every row shares, so selects are one width and switches line up. The longer
explanation is behind the (i) beside the name. At 900px and below each row stacks its control under
its description. On/off preferences are switches: a submit button with `role="switch"` and
`aria-checked`, so they still post with scripting off.

Every form on the screen returns to `/settings?flash=...#section`, so after the full-page reload the
reader is back at the section they changed with a toast naming what was saved. The shared Listings
controls (task view, projects, columns) are pointed back the same way when rendered on Settings and
keep returning to their own screen elsewhere.

### Hover effects

Rows, cards, lane cards, sidebar links, buttons and links respond to the pointer: a row tints and
gains an accent edge, a card lifts and gains a shadow, a link in running text thickens its underline.
The rules are written once under `:root:not([data-hover="off"])` and inside `@media (hover: hover)`,
so a touch screen never gets a sticky highlight. Turning the preference off sets `data-hover="off"`
on `<html>`, which removes them and resets the older hover tints to their resting look; focus rings
are not hover effects and stay. Movement and transitions are declared only under
`prefers-reduced-motion: no-preference`, so a reader whose system asks for less motion gets the
colour without the lift whatever the preference says.

### Opening a task in a dialog

With Task opens in set to Dialog, following a link to a task from a listing, a board, the lanes, the
activity feed, search results or the toast after adding opens the task screen in a modal `<dialog>`
over the page instead of leaving it. It is the same screen: the server answers a request carrying
`X-Tix-Dialog: 1` with the task screen's content block and its flash, without the layout, inside an
element marked `data-task-dialog`. The ids the task screen shares with a listing's rows (the status
and priority menus, the description, priority and repeat fields) are prefixed in that answer, and the
task's own live binding is left out, so nothing in the dialog addresses the page behind it.

The address changes to the task's, with an entry of its own in history, so a reload or a copied link
draws the full page, Back closes the dialog, and Forward opens it again over the same page. Escape,
the Close button where the page has its back link, and a click on the backdrop all close it by going
Back. A popstate leaving the dialog's entry is stopped before htmx sees it, so htmx does not restore
an older snapshot of the page over the live one. Focus returns to what opened the dialog, or to the
task's row when a refresh has redrawn it; while the dialog is open the rest of the page is inert,
which is the focus trap, and the keyboard shortcuts stand down. A link to another task inside the
dialog replaces the entry; any other link leaves for its own page.

Every form in the dialog posts as it does on the page, and the request's header survives the
redirect, so the answer redraws the dialog in place. The listing underneath is refreshed through its
live binding (`tix:refresh`), whether or not its socket is open. A save that redirects somewhere other
than the task, which is what deleting it does, closes the dialog and loads that page; a refusal keeps
the task and shows the error screen's message above it. A task that cannot be opened at all opens as
its own page, which explains why. Under 600px the dialog fills the screen.

A click with a modifier, a middle click, and any click without scripting open the full page.

### Renaming a title in place

With Clicking a title set to Edits the title, the title of a task in the list, on a board card and on
a lane card carries its reference and version, and a small open link is drawn beside it. A click turns
the title into a text field. Enter or leaving the field posts `version` and `title` to the task's
edit address (`POST /tasks/{ref}`), the same address and the same version check as the task screen's
form; Escape puts the title back, and an unchanged title sends nothing. A refusal, an empty title or
a conflict with a newer version included, is shown under the field and the field stays open; leaving
it does not resend a refused value. Enter or F2 on a selected row edits its title, and the open link
opens the task as Task opens in says. The edit address changes the description and assignee only
when the submission names them, so renaming leaves them alone. Without scripting a title is a link.

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
| Advanced screens, drag to move, open a new task, hover effects, task opens in, clicking a title, columns, hidden projects, task view, colour rows by, language | cookie | no equivalent |
| Selected-row pulse | no equivalent | `tui.motion` |

So a timezone chosen in the browser does not change what `tix task ls` prints, and one chosen in the
terminal is what `tix task ls` prints and not what a browser renders. See [tui.md](tui.md) for the
terminal screen and [configuration.md](configuration.md) for the keys.

The zone list is a selection rather than every name the system carries. A `select` of six hundred
entries is not a control anybody can use, and a fixed list also bounds how many template sets one
process can be made to parse: the formatting helpers are bound into a template at parse time, so a
per-reader zone has to be a separately parsed set rather than a value handed to `Execute`.

The preference forms, the sidebar's scheme switch among them, deliberately opt out of htmx boosting.
`data-theme` lives on `<html>` and a boosted form swaps only the body, so the cookie changed while
the page kept its old scheme, which reads as a button that does nothing. A full navigation costs one
round trip and is correct.

The colour scheme is a separate axis from the tenant accent, which is the same everywhere and covered
in [theming.md](theming.md).

## Language and info tips

Every screen is drawn in the language the browser asks for in `Accept-Language`, and says so in the
root element's `lang`. English and Bulgarian ship; a header naming neither gets English. Keys are the
English text (`internal/i18n`), so a word a catalog lacks reads in English rather than as a blank.
Settings can override the browser's choice for this browser (`tix_lang`).

A small (i) beside a heading, a filter or a figure opens a sentence or two on what it means and names
the page in `docs/` that says more. It is a disclosure, so it works with no script, from the keyboard
and on touch.

To add a language, add `internal/i18n/<lang>_*.go` files registering one entry per key. The tests in
`internal/web/i18n_internal_test.go` name every key it is missing.

## The second factor

A tenant can require a time-based code in the browser, and any reader can enrol in one without being
asked to. Either way the sign-in screen grows a second step: the password form answers with a code
form instead of a session, carrying a signed ticket in a hidden field. The ticket says only that the
password was accepted in the last five minutes, so it grants nothing on its own, and no session
exists until the code is taken. The destination a deep link was bounced from rides through both
steps, so the reader still lands where they were going.

An account the tenant requires a factor from that holds none is taken through enrolment on that same
step rather than refused. This is what keeps turning the requirement on from locking out the
administrator who turned it on, and it is why the recovery codes appear there: they exist for exactly
one response and are shown before the reader goes on.

The code field also takes a recovery code. A verifier that has been locked by repeated failures says
so and says to wait, because a reader told their code is wrong tries again immediately, which is what
the lock exists to stop.

Settings carries the rest: enrolment, how many recovery codes are unused, replacing them, and
removing the factor where the tenant does not require one. The user administration list offers an
administrator holding `totp:admin` the clear, which is the way back in for somebody who lost both the
authenticator and the codes. No secret and no recovery code appears on either screen afterwards.

The `otpauth://` code is drawn as inline SVG by the server, by the encoder in `internal/web/qr.go`
rather than by a client-side library. Every form here is a plain form: a sign-in that needed a script
would be a sign-in some readers could not complete, and a script that drew the code would need the
enrolment secret handed to it.

## Project icons

The new-project form and a project's settings carry an icon picker. Emoji takes one emoji or one or two
letters, Icon offers a grid of built-in symbols drawn in the same style as the navigation, Upload takes a PNG,
JPEG, WebP or SVG of up to 64 KB, and No icon removes it. The preview beside the choice shows what the project
will carry; saving the form with Upload chosen and no new file keeps the image already there.

An uploaded image is drawn through `<img>`, where an SVG runs no script and fetches nothing, and an SVG that
carries script is refused on upload anyway. It is served from `/projects/{key}/icon` with its own content
type, `nosniff`, a sandboxing content security policy and an ETag; the screens address it with the image's
digest, so a browser keeps it until the image changes.

## Static assets

The stylesheet, the scripts and the icon are compiled into the binary and served from memory with a
strong `ETag` derived from their bytes and `Cache-Control: public, max-age=60, must-revalidate`. A
conditional request for an unchanged asset gets a 304 with no body.

The validator is the point. The asset URLs do not change between releases, and the responses used to
carry no validator at all, so a browser cached them heuristically and after an upgrade a reader got
the new markup with the previous release's stylesheet. That happened during review and was reported
as the interface being broken. Each file is fingerprinted independently, so changing one does not
invalidate the others.

**Compression.** A browser that accepts gzip gets every screen, fragment and asset compressed. The
task list was about 185KB of HTML per load, swap and live refresh, and is about 17KB on the wire
now; the stylesheet goes from 200KB to about 56KB. Screens are compressed as they are written, at the
fastest level; assets once, at startup, at the best level, and each encoding carries the digest of
its own bytes as its `ETag`. Every response says `Vary: Accept-Encoding`.

Compressing a page that echoes input (the filter box) beside a constant secret leaks the secret to
anyone who can steer requests and watch their sizes, which is the BREACH attack. So the CSRF value a
form carries is masked with a fresh one-time pad on every response, and the cookie's own value is
never on a page. The check accepts the masked value or the plain one.

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
