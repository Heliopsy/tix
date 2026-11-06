# web-tasks Specification

## Purpose

Covers the browser's task screens: the listing, the board, and the controls that switch between them.

## Requirements

### Requirement: The task listing offers a deadline control

The task list screen SHALL offer a control selecting a deadline window from the vocabulary the filter
language's deadline term accepts.

The control SHALL narrow the listing by writing that term into the filter expression the screen parses,
rather than by setting a bound of its own, so that the question asked is visible in the filter box and is
answered by the one shared parser.

#### Scenario: Selecting overdue narrows the listing

- **GIVEN** one task whose deadline has passed, one whose deadline is far off, and one with no deadline
- **WHEN** the reader selects the overdue window
- **THEN** only the task whose deadline has passed is listed

#### Scenario: The control and the box ask the same question

- **WHEN** the same deadline term is typed into the filter box instead
- **THEN** the same tasks are listed

### Requirement: The deadline control shows its window and refuses an unknown one

The deadline control SHALL show the window currently selected. Its selection SHALL be carried across
pages of the listing. A window the filter language does not accept SHALL be reported beside the filter
box, with the listing withheld, like any other refused expression.

#### Scenario: The control shows its own state

- **WHEN** the listing is rendered for a selected window
- **THEN** that window is the selected option of the control

#### Scenario: An unknown window is refused, not answered

- **WHEN** a request names a deadline window the filter language does not accept
- **THEN** the message is rendered beside the filter box
- **AND** no rows are listed

### Requirement: A task row marks a deadline only while it is pressing

A row of the task listing SHALL carry a deadline badge when, and only when, the task's deadline classifies
as passed or as near, following the same classification the terminal board follows.

The badge SHALL name the state in words as well as by its colour. It SHALL be one of the listing's
optional columns, so a reader can turn it off.

#### Scenario: An overdue row is badged

- **WHEN** a task's deadline has passed
- **THEN** its row carries a badge naming it overdue

#### Scenario: A distant or absent deadline is not badged

- **WHEN** a task's deadline is further off than the notable window, or the task carries none
- **THEN** its row carries no deadline badge

### Requirement: The task screen names the deadline state

The task screen SHALL name the deadline's state beside the date where the deadline is pressing, in the
same words the listing and the terminal use.

#### Scenario: The task screen says a deadline has passed

- **WHEN** the screen of a task whose deadline has passed is rendered
- **THEN** the deadline is shown with a badge naming it overdue

#### Scenario: A distant deadline is shown without a badge

- **WHEN** the screen of a task whose deadline is further off is rendered
- **THEN** the deadline is shown and no badge is drawn

### Requirement: The task screens reflect a change made anywhere else

The task listing, the project board and a single task's screen SHALL update
from the tenant's event stream while they are open, without the reader
reloading, whatever surface the change was made on.

Each refresh SHALL be rendered by the server, from the same template blocks
the full page was rendered by. The browser SHALL NOT be sent rendered markup
over the event stream, and SHALL NOT hold a model of task state of its own.

#### Scenario: A transition made in the terminal reaches an open task screen

- **GIVEN** a task screen open in a browser
- **WHEN** the task is moved from todo to done over SSH
- **THEN** the screen's status reads done without the reader reloading

#### Scenario: A listing updates when a task it shows changes

- **GIVEN** a task list open in a browser
- **WHEN** an agent claims a task on that list
- **THEN** the row says so without the reader reloading

#### Scenario: A refreshed row is the row the page renders

- **WHEN** the fragment a refresh fetches and the page it belongs to both render the same task
- **THEN** the markup of that row is identical, because one template block produced both

#### Scenario: Without JavaScript the screens still work

- **WHEN** a task screen is opened with JavaScript disabled
- **THEN** it shows the current state on load and every control on it still works

### Requirement: A live refresh preserves what the reader is looking at

A refresh SHALL answer the same question the screen was already answering. The
filter, the deadline window, the sort, the page cursor and page size, the
list-or-board choice, the chosen columns, the project visibility choice and
the project or task in view SHALL all survive it.

#### Scenario: A filtered listing stays filtered

- **GIVEN** a task list narrowed by a filter expression and a sort
- **WHEN** an event refreshes it
- **THEN** the rows shown are the filtered, sorted rows, not the first page of everything

#### Scenario: A reader on page two stays on page two

- **GIVEN** a task list showing a page reached through its pager
- **WHEN** an event refreshes it
- **THEN** the same page of the listing is shown

#### Scenario: The board stays the board

- **GIVEN** a reader whose task screen is drawn as a board
- **WHEN** an event refreshes it
- **THEN** the board is redrawn, not the list

#### Scenario: A put-away project stays away

- **GIVEN** a task list with a project hidden by the visibility control
- **WHEN** an event refreshes it
- **THEN** that project's tasks are still absent

#### Scenario: A row action still returns to the screen

- **WHEN** a reader completes a task on a row that arrived through a refresh
- **THEN** they are returned to the listing they were on, filter and position intact, and never to the fragment route

### Requirement: A live refresh never discards work in progress

A refresh SHALL NOT replace a region in which the reader is working. A card
held in a drag, a field holding the caret, and a field changed away from what
the server rendered into it each SHALL defer the refresh. A deferred refresh
SHALL be applied once the region is idle, and SHALL NOT be dropped.

The forms that edit a task, its move control, its comment box and the
screen's own filter and quick-add controls SHALL NOT be inside a replaced
region at all.

#### Scenario: A refresh during a drag waits

- **GIVEN** a card being dragged between columns
- **WHEN** an event would refresh the board
- **THEN** the board is not replaced while the drag is in progress, and is refreshed once it ends

#### Scenario: A half-written comment survives

- **GIVEN** a reader part way through typing in a field inside a refreshed region
- **WHEN** an event would refresh it
- **THEN** what they typed is still there, and the refresh is applied after they stop

#### Scenario: The filter box is never swapped

- **WHEN** a task list is refreshed
- **THEN** the filter expression in the box and the text in the quick-add field are untouched

### Requirement: A screen subscribes only to what it can draw

Each screen SHALL subscribe with a filter naming the event types it renders,
and a screen about one project SHALL name that project. A screen about one
task SHALL further ignore an event that names neither that task nor a record
belonging to it.

#### Scenario: An unrelated tenant event changes nothing

- **GIVEN** an open task screen
- **WHEN** a webhook delivery or a sign-in is recorded for the tenant
- **THEN** the screen does not re-render

#### Scenario: A neighbouring task does not rebuild a task screen

- **GIVEN** a task screen open on one task
- **WHEN** another task in the same project is transitioned
- **THEN** that screen does not re-render

#### Scenario: A comment on the task does reach it

- **GIVEN** a task screen open on one task
- **WHEN** a comment is added to that task
- **THEN** the comment appears without the reader reloading

### Requirement: A star button on every task the browser draws

The task list rows, the board cards, the lane cards and the task screen SHALL each offer a star toggle for
a reader who may star. The toggle SHALL be a real button inside a form posting with the CSRF
token, SHALL work without JavaScript, SHALL report its state with `aria-pressed`, and SHALL carry
an accessible name saying what pressing it does. Pressing it SHALL star an unstarred task and
unstar a starred one and SHALL return the reader to the screen they were on.

#### Scenario: Starring a row

- **GIVEN** a task list with an unstarred task
- **WHEN** the reader presses its star button
- **THEN** the task is starred, the reader is returned to the same listing, and the task is drawn first with its button pressed

#### Scenario: Unstarring from the task screen

- **GIVEN** a starred task's screen
- **WHEN** the reader presses the pressed star button
- **THEN** the task is unstarred and the button is no longer pressed

#### Scenario: No button without the authority

- **GIVEN** a reader who may not star
- **WHEN** the list is drawn
- **THEN** no star button is drawn

#### Scenario: Another open tab follows

- **GIVEN** the same reader with the task list open in a second tab
- **WHEN** they star a task in the first tab
- **THEN** the second tab refreshes from the starred event and draws the task first

### Requirement: Starred tasks are drawn first

Starred tasks SHALL be drawn first in the list, first within each board column and first within each
lane.

#### Scenario: A starred task leads its lane

- **GIVEN** the lanes view with two tasks in one project, the later one unstarred
- **WHEN** the reader stars the later one from its lane card
- **THEN** that card is drawn first in its lane with its button pressed

### Requirement: The browser sets and shows a repeat rule

The task screen's Repeats row and the add-task form SHALL offer a Repeat field taking the rule
syntax, with a hint naming an example. Saving the Repeats row with the field emptied SHALL remove
the rule; a form that does not carry the field SHALL leave the rule unchanged, and no other form on
the task screen SHALL carry it. A rule the service refuses SHALL be reported on the form rather
than dropped. The task screen SHALL state the rule and, once the task has spawned, link its
successor.

#### Scenario: Setting a rule from the task screen

- **WHEN** a reader enters `every week` in the Repeats row and saves
- **THEN** the task carries `every week` and the screen states it

#### Scenario: Refused rule

- **WHEN** a reader saves the Repeat field as `every fortnight`
- **THEN** the form reports the refusal and the task is unchanged

### Requirement: A repeating task is marked in the listing and repeats from the browser

A listing row and a board card for a task carrying a rule SHALL show a small repeat marker whose
accessible name states the rule. Completing a recurring task from the browser SHALL create the next
occurrence exactly as on every other surface.

#### Scenario: Marker on a row

- **WHEN** the listing shows a task carrying `every month`
- **THEN** its row shows the repeat marker naming `every month`, and a row of a task with no rule shows none

### Requirement: The task screen states the star, the parent and the next occurrence

A subtask's screen SHALL link the task it belongs to. A repeating task's screen SHALL state its rule and, when the next occurrence exists, link it
with its due date; when it does not exist yet, SHALL state the date finishing the task now would
give it, computed by the same rule completion applies; and when the rule has run out, SHALL say
that this is the last occurrence.

#### Scenario: A weekly task not yet finished

- **GIVEN** an unfinished task due on a Monday that repeats `every week`
- **WHEN** its screen is drawn
- **THEN** it states the rule and that finishing now makes the next one due the following Monday

#### Scenario: A finished occurrence

- **GIVEN** a repeating task that has been finished and created its next occurrence
- **WHEN** its screen is drawn
- **THEN** it links the next occurrence with that occurrence's due date

#### Scenario: The last occurrence

- **GIVEN** a task whose rule has one occurrence left
- **WHEN** its screen is drawn
- **THEN** it says this is the last occurrence

#### Scenario: Starred and a subtask

- **GIVEN** a starred subtask
- **WHEN** its screen is drawn
- **THEN** its star control is pressed and named Unstar, and the screen links its parent

### Requirement: The task screen says when the task started and finished

When the task has started or finished, the task screen SHALL say when.

#### Scenario: A finished task states its times

- **GIVEN** a task that has been started and then finished
- **WHEN** its screen is drawn
- **THEN** it states when the task started and when it finished

### Requirement: The task screen can draw one lane per project

The task screen SHALL offer a lanes view that draws the selected page of tasks as one lane per
project. The lanes SHALL hold exactly the tasks the list would draw for the same filter, deadline
window, sort, page, page size and project visibility choice, except that the open-work default the
list applies SHALL NOT narrow the lanes, so finished tasks are drawn in their lanes.

#### Scenario: Tasks are grouped into their own project's lane

- **GIVEN** tasks in two projects
- **WHEN** the reader draws the lanes
- **THEN** each project has one lane holding exactly its own tasks
- **AND** the lanes follow the order the screen lists projects in

#### Scenario: A project put away has no lane

- **GIVEN** a reader who has put one project away
- **WHEN** they draw the lanes
- **THEN** no lane and no card of that project is drawn

#### Scenario: Finished tasks are in their lanes

- **GIVEN** a project with one open and one finished task and no filter
- **WHEN** the lanes are drawn
- **THEN** both tasks are in the project's lane

### Requirement: Lanes follow the project order and only named or occupied projects get one

Lanes SHALL be ordered by the order the screen lists projects in, and the tasks within a lane SHALL
keep the listing's order. A lane SHALL be drawn for every project with a task on the page, and for
every project a `project:` term in the filter names, even when it holds no task. No other empty lane
SHALL be drawn. A task on the page whose project the screen cannot name SHALL be reported on the
screen rather than omitted.

#### Scenario: A project named by the filter has a lane even when empty

- **GIVEN** a filter naming a project with `project:` that holds no matching task
- **WHEN** the lanes are drawn
- **THEN** that project's lane is drawn and says it holds no tasks

#### Scenario: An empty project not named by the filter has no lane

- **GIVEN** a project with no task on the page and no `project:` term naming it
- **WHEN** the lanes are drawn
- **THEN** no lane is drawn for it

### Requirement: A lane card shows its status

Every lane card SHALL show its task's status as a badge naming the state's label, falling back to
the state key, and styled by the state's category. A state with no declared category SHALL be styled
as done when terminal, as todo when it is the workflow's initial state, and as in progress otherwise.

#### Scenario: The badge names the state and its category

- **GIVEN** a task in the built-in workflow's `doing` state
- **WHEN** its lane card is drawn
- **THEN** the card carries a badge reading `Doing` with the in-progress category style

### Requirement: Finished lane cards can be hidden per browser

The lanes view SHALL offer a Show completed toggle that hides or shows the cards of finished tasks in
every lane. The choice SHALL be kept per browser in local storage as a single boolean under the key
`tix.lanes.showCompleted`, and showing finished cards, the default, SHALL store nothing. Local storage
SHALL hold nothing else for this view: no project key, task reference, count or other tenant data.

#### Scenario: Hiding finished cards is remembered

- **GIVEN** a reader on the lanes view
- **WHEN** they turn Show completed off
- **THEN** finished cards are hidden
- **AND** local storage holds `false` under `tix.lanes.showCompleted` and nothing else for lanes

#### Scenario: Turning it back on stores nothing

- **GIVEN** a browser that hid finished cards
- **WHEN** the reader turns Show completed on
- **THEN** the key is removed and finished cards are shown

### Requirement: The stored lanes choice is applied early and falls back to showing

The stored choice SHALL be applied before the lanes are painted and SHALL survive a live refresh of
the listing. Storage that cannot be read SHALL read as the default. Without scripting the toggle
SHALL NOT be shown and finished cards SHALL be shown.

#### Scenario: Storage that throws reads as shown

- **GIVEN** a browser whose local storage refuses access
- **WHEN** the lanes are drawn
- **THEN** finished cards are shown

### Requirement: Lanes stay current and fit the screen

The lanes view SHALL refresh from the event stream through the same listing fragment as the list and
the board, keeping the view, filter, sort, page and visibility choice.

#### Scenario: A refresh keeps the lanes

- **GIVEN** a reader on the lanes view
- **WHEN** the listing fragment is fetched for a live refresh
- **THEN** it draws lanes holding the tasks on screen

### Requirement: Lanes fit a narrow screen and a coarse pointer

From 640px wide, lanes SHALL scroll sideways inside their own frame rather than scrolling the page.
Below 640px each lane SHALL take most of the width of the screen, the lanes SHALL snap sideways one
lane at a time, and each lane SHALL be its natural height so that the page, not the lane, scrolls
vertically. On a coarse pointer the tick, status and priority controls of a lane card SHALL be at
least 40px in both dimensions.

#### Scenario: A phone scrolls the page, not the lane

- **GIVEN** a viewport 390px wide
- **WHEN** the lanes are drawn
- **THEN** a lane does not scroll vertically inside itself and the lanes snap sideways

### Requirement: A lane card's tick completes and reopens along its workflow

A lane card's tick SHALL complete an unfinished task and reopen a finished one, following the task's
own project's workflow. For an unfinished task the candidate routes SHALL be the shortest routes from
its state to each terminal state of the done category. For a task in a terminal state the candidate
routes SHALL be the routes to non-terminal states that have the fewest steps.

#### Scenario: Ticking completes the task along its only route

- **GIVEN** a task in `todo` in the built-in workflow
- **WHEN** the reader ticks its box in the lanes view
- **THEN** the task moves through `doing` to `done`
- **AND** the reader returns to the same listing

#### Scenario: Unticking reopens the task along its nearest route

- **GIVEN** a task in `done` in the built-in workflow
- **WHEN** the reader ticks its box again
- **THEN** the task moves to `todo`

### Requirement: A tick route is usable only without a comment and with every scope it needs

A tick's candidate route SHALL be usable only when no step of it requires a comment and the reader holds the
scope to transition tasks and every scope a step requires.

#### Scenario: A route that needs a comment says so

- **GIVEN** a workflow whose only route to done requires a comment
- **WHEN** the lane card is drawn
- **THEN** no route is submitted by the tick
- **AND** its panel says a comment is needed and links to the task

#### Scenario: A reader without the scope is not offered a route

- **GIVEN** a reader who cannot transition tasks
- **WHEN** the lane card is drawn
- **THEN** no route is submitted by the tick and its panel says they cannot move tasks

### Requirement: The tick is a box, a menu or a panel by how many routes are usable

When there is exactly one candidate and it is usable, the tick SHALL be a box that applies it. When
there is more than one candidate, the tick SHALL open a menu offering every usable candidate and
naming the states each passes through. When no candidate is usable, or there is none, or the reader
cannot transition tasks, the tick SHALL open a panel that says why in visible text and links to the
task's own screen.

#### Scenario: Two equally near open states offer the choice

- **GIVEN** a workflow in which two non-terminal states are each one step from a terminal state
- **WHEN** the lane card of a task in that terminal state is drawn
- **THEN** the tick opens a menu offering both routes and applies none by itself

#### Scenario: Two done states offer the choice

- **GIVEN** a workflow in which two terminal done-category states are reachable from the task's state
- **WHEN** the lane card is drawn
- **THEN** the tick opens a menu offering both routes

#### Scenario: A workflow with no way back says so

- **GIVEN** a workflow with no transition out of its done state
- **WHEN** the lane card of a finished task is drawn
- **THEN** the card renders ticked and its panel says no route leads back to an open state

### Requirement: A tick is a plain form that names the states it passes

Every tick SHALL submit its route to the task's transition route as a form post carrying the CSRF
token and the listing to return to, and SHALL work without scripting. Where a route passes through
other states the control SHALL name them in its accessible name before it is applied. A finished
card SHALL render ticked whichever control it offers.

#### Scenario: The tick box is a form naming the state it passes

- **GIVEN** a task in `todo` in the built-in workflow
- **WHEN** its lane card is drawn
- **THEN** the tick is a form posting the route through `doing` to `done` to the task's transition route
- **AND** the form carries the CSRF token and the listing to return to, and names `doing`

### Requirement: A lane card's status and priority are menus

A lane card SHALL draw its status badge as a control that opens a menu of the routes the reader may
take from the task's state in its own project's workflow, omitting any route that needs a comment or
a scope the reader lacks, each submitted to the task's transition route. A reader who cannot
transition tasks, or a card with no route to offer, SHALL see the badge as a label.

#### Scenario: The status menu offers the workflow's moves

- **GIVEN** a task in `todo` in the built-in workflow and a reader who can transition tasks
- **WHEN** the reader opens the card's status menu
- **THEN** it offers `doing`, `blocked` and `cancelled` and the route through `doing` to `done`
- **AND** choosing `blocked` moves the task to `blocked`

#### Scenario: A move needing a comment is left out

- **GIVEN** a workflow whose transition into `blocked` requires a comment
- **WHEN** the card's status menu is drawn
- **THEN** it does not offer `blocked`

### Requirement: A lane card's priority is a menu and both menus are plain forms

A lane card SHALL draw its priority as a control that opens a menu of every priority, marking the
current one, each submitted to the task's priority route, for a reader who may write tasks. A reader
who may not SHALL see the priority as a label, and a normal priority SHALL then not be drawn.

Both menus SHALL work as plain forms without scripting and SHALL be operable from the keyboard.

#### Scenario: The priority menu changes the priority

- **GIVEN** a reader who may write tasks
- **WHEN** they choose `high` in a card's priority menu
- **THEN** the task's priority is high

#### Scenario: A reader who may not change them sees labels

- **GIVEN** a reader who may only read tasks
- **WHEN** the lanes are drawn
- **THEN** no card carries a status or priority form

### Requirement: Lane actions change only their own card

With scripting, every tick, status and priority action on a lane card SHALL replace only that card
with its newly drawn self, SHALL mark the card busy while the request is in flight, and SHALL return
keyboard focus to the same control on the new card. Markup a live refresh draws SHALL behave as the
markup it replaced, so a lane action after a refresh is still swapped in place. A refused action SHALL leave the card in place and
show the refusal inside it.

#### Scenario: A tick swaps one card

- **GIVEN** a reader on the lanes view with scripting
- **WHEN** they tick a card
- **THEN** only that card is replaced and focus is on its tick

#### Scenario: A refusal is shown on the card

- **GIVEN** a card whose task another reader has just moved where the route no longer applies
- **WHEN** the reader ticks it
- **THEN** the card stays and says why the move was refused

### Requirement: A live refresh waits for a lane action

A live refresh SHALL NOT replace the listing while a lane action is in flight or a menu in it is open,
and SHALL return focus to the element that held it when it lands.

#### Scenario: A refresh waits for an open menu

- **GIVEN** a reader with a lane card's status menu open
- **WHEN** a live update arrives
- **THEN** the listing is not replaced until the menu closes

### Requirement: A focused button keeps its own keys

A keyboard shortcut SHALL NOT take Enter or Space from a focused button or summary, or Enter from a
focused link, so that every control on a page can be pressed from the keyboard whatever scheme is
active. While a menu is open no shortcut SHALL act, so Escape closes the menu and nothing else.

#### Scenario: Enter presses a focused lane control

- **GIVEN** the default shortcut scheme, which binds Enter to opening the selected row
- **WHEN** the reader focuses a lane card's status badge and presses Enter
- **THEN** the status menu opens

#### Scenario: Escape closes a menu and stays on the page

- **GIVEN** a lane card's status menu is open
- **WHEN** the reader presses Escape
- **THEN** the menu closes and the page does not go back

### Requirement: Every lane card has a more actions menu the View settings cannot hide

Every card in the lanes view SHALL carry a "more actions" button, drawn as three dots, whatever
fields the reader has hidden through the View menu or the column picker. While the menu is open the
Down and Up arrow keys SHALL move focus between its items.

#### Scenario: The dots survive every hidden field

- **GIVEN** a reader who has hidden every optional task field
- **WHEN** the lanes view is drawn
- **THEN** every card still has its more actions button and menu

### Requirement: The lane card's more actions menu behaves like the card's other menus

The "more actions" button SHALL open a popover menu inside the card with the same behaviour as the
card's other menus: Escape SHALL close the menu and return focus to the button without leaving the
page, no keyboard shortcut SHALL act while it is open, a live refresh SHALL wait while it is open,
and on a coarse pointer the button and every item SHALL answer a press at least 40px tall.

#### Scenario: Escape closes only the menu

- **GIVEN** the more actions menu of a card is open
- **WHEN** the reader presses Escape
- **THEN** the menu closes, focus is on the card's more actions button, and the page stays

### Requirement: The more actions menu gives full control of the task in place

The menu SHALL offer, to a reader allowed each one: opening the task, honouring the reader's choice
of page or dialog; copying the task's link and reference; completing or reopening it along the
routes the tick offers, or the reason the tick gives when it offers none; every move the status
menu offers; and a link to the task for each move that needs a comment.

#### Scenario: A move that needs a comment opens the task

- **GIVEN** a workflow whose route out of the card's state requires a comment
- **WHEN** the card's menu is drawn
- **THEN** that move is offered as a link to the task's screen and not as a form

### Requirement: The more actions menu edits the task's fields within the reader's scopes

The menu SHALL offer, to a reader allowed each one: every priority; starring or unstarring; renaming
the title in place; setting the assignee; adding and removing tags; and a soft delete. A reader
without the scope for an action SHALL NOT be offered it: moves need the transition scope, priority,
rename, assignee and tags need task write, delete needs task delete, and starring needs task read.

#### Scenario: Changing the priority from the menu

- **GIVEN** a reader who may write tasks, with Priority hidden
- **WHEN** they choose High in a card's more actions menu
- **THEN** the task's priority is high and the reader returns to the same listing at that card

#### Scenario: Setting the assignee from the menu

- **GIVEN** a reader who may write tasks
- **WHEN** they submit an assignee in a card's more actions menu
- **THEN** the task is assigned to that actor and the reader returns to the listing at that card

#### Scenario: Adding and removing a tag from the menu

- **GIVEN** a reader who may write tasks, with Tags hidden
- **WHEN** they add a tag in the menu and then remove it
- **THEN** each change is saved and each returns to the listing at that card

#### Scenario: A read-only reader is offered only what they may do

- **GIVEN** a reader who may read tasks but not write, move or delete them
- **WHEN** the lanes view is drawn
- **THEN** each card's menu offers opening, copying and starring, and no form that changes the task

### Requirement: Each more actions menu action works without scripting and swaps only its card

Each action SHALL be a form post carrying the CSRF token and the listing to return to, or a link,
and SHALL work without scripting. Under htmx a form SHALL swap only its own card, mark the card busy
while the request is in flight, and show a refusal inside the card. A delete SHALL bring the toast
with its Undo, and the Undo SHALL return the reader to the listing with the card restored.

#### Scenario: Deleting from the menu offers Undo

- **GIVEN** a reader who may delete tasks
- **WHEN** they delete a task from its card's menu
- **THEN** the task is soft deleted, the listing says so with an Undo
- **AND** the Undo restores the task and returns to the listing

### Requirement: The task edit, tag and restore routes return to a posted listing

The task edit route, the tag and untag routes, and the restore route SHALL, when the form posts a
`next` listing, redirect to that listing at the task's card or row with the outcome in a toast.
Without `next` each SHALL redirect to the task's own screen as before.

#### Scenario: Editing without next stays on the task screen

- **GIVEN** the task screen's edit form, which posts no next
- **WHEN** it is saved
- **THEN** the reader is returned to the task's screen

#### Scenario: Restoring with next returns to the listing

- **GIVEN** a soft deleted task and a restore form posting a listing as next
- **WHEN** it is submitted
- **THEN** the task is restored and the reader is returned to that listing at the task

### Requirement: Every group heading carries a menu

Each column of the task screen's board, each lane of the lanes view, each column of a project's
board and each group heading of a grouped list SHALL carry a three dots button opening a menu of
actions on that group. The button SHALL be visible while the pointer is over the heading or focus
is inside it, and SHALL always be visible on a coarse pointer, where it and every menu item SHALL be
at least 40px tall.

#### Scenario: Hovering a column reveals its dots

- **GIVEN** the task screen drawn as a board
- **WHEN** the reader moves the pointer over the `todo` column heading
- **THEN** its three dots button becomes visible and opens a menu of actions on `todo`

### Requirement: A group menu holds the keyboard and refresh and works without scripting

Escape SHALL close only the group menu and return focus to its button; keyboard shortcuts SHALL not
act and a live refresh SHALL wait while the menu is open. Every action SHALL be a form post that
works without scripting and, with scripting, SHALL redraw the view in place without reloading the
page.

#### Scenario: The menu works without scripting

- **WHEN** the reader submits Hide from a column's menu in a browser without JavaScript
- **THEN** the screen returns to the same listing with the column hidden

### Requirement: A reader hides, stars, collapses and reorders groups

The menu SHALL offer to hide the group, to star or unstar it, to collapse or expand it, and to move
it left or right (up or down in a list). A hidden group SHALL not be drawn. A starred group SHALL be
drawn before every unstarred one. A collapsed group SHALL be drawn as a strip naming the group and
its count with no cards or rows. Move SHALL swap the group with its drawn neighbour of the same star
state and SHALL be absent where there is none.

#### Scenario: A starred lane comes first

- **GIVEN** lanes for projects `alpha` and `beta`, in that order
- **WHEN** the reader stars the `beta` lane
- **THEN** the `beta` lane is drawn before `alpha`

#### Scenario: A collapsed column still takes a dropped card

- **GIVEN** a collapsed `doing` column
- **WHEN** the reader drags a card from `todo` onto it
- **THEN** the task moves to `doing`

### Requirement: A reader's group arrangement is kept per actor and view on the server

The arrangement of groups SHALL be stored per actor and per view on the server, so it applies in
any browser the reader signs in to, and SHALL never change what another reader sees.

#### Scenario: A hidden column stays hidden in another browser

- **GIVEN** a reader who hid the `done` column of the task screen's board
- **WHEN** they sign in from a fresh browser and open the board
- **THEN** the `done` column is not drawn

#### Scenario: Another reader is unaffected

- **WHEN** one reader hides a column
- **THEN** another reader of the same tenant still sees it

### Requirement: Hidden groups are counted and can be shown again

When a view hides any group it would otherwise draw, a control at the end of the view SHALL say how
many are hidden and offer to show each one, and the task screen's View menu SHALL list the same
groups with the same choice and offer to reset the arrangement of the view in force.

#### Scenario: Showing a hidden column again

- **GIVEN** a reader who hid the `done` column
- **WHEN** they press Show beside `done` under "Hidden: 1"
- **THEN** the `done` column is drawn again

### Requirement: A group's menu offers what belongs to its kind

A project group's menu SHALL link to the project, to the task screen filtered to the project, and
SHALL offer a field adding a task to that project. A status group's menu SHALL link to the task
screen filtered to that status and SHALL offer a field adding a task in that state, to a project the
group draws, when the reader may create tasks.

#### Scenario: Adding a task from a column

- **WHEN** the reader adds "Write release notes" from the `doing` column's menu
- **THEN** a task with that title is created in state `doing` and the reader stays on the board

### Requirement: The list can group its rows

The list view SHALL offer a choice, kept per browser, to draw its rows ungrouped, grouped by project
or grouped by status. A grouped list SHALL draw one heading per group present on the page, carrying
the group's name, its count and the group menu, and SHALL share the arrangement of the lanes view
when grouped by project and of the board when grouped by status.

#### Scenario: Grouping the list by project

- **WHEN** the reader chooses "Group by project" under View
- **THEN** the list draws a heading per project with its rows under it

#### Scenario: A lane hidden in the lanes view is hidden in the grouped list

- **GIVEN** a reader who hid the `homelab` lane
- **WHEN** they draw the list grouped by project
- **THEN** no `homelab` group is drawn and the hidden count names it

### Requirement: Add with details leads with the title and folds the rare fields

The Add with details form SHALL put the title first, as the largest control of the form, and SHALL
focus it when the form opens. The title SHALL be marked as required. Every control other than the
title SHALL be `--control-h` tall.

#### Scenario: Opening the form

- **WHEN** the reader opens Add with details on the task list
- **THEN** the title field is first and focused, and the repeat rule is inside a closed More options disclosure

### Requirement: Add with details folds the repeat rule, parent and dependencies under More options

The Add with details form SHALL offer the description, project, priority, due date, assignee and tags
without further disclosure, and SHALL offer the repeat rule, the parent task and the dependencies
behind a "More options" disclosure. That disclosure SHALL be drawn open when any of its fields holds
a value or a refusal.

#### Scenario: Creating with a due date, an assignee and a parent

- **GIVEN** a task `infra-1` and a member `bob`
- **WHEN** the reader adds a task due 2026-12-01, assigned to `bob`, with parent `infra-1`
- **THEN** the task is created with that due date, assignee and parent

### Requirement: The add forms preselect the listing's project

When the task list is filtered to exactly one project, both the one-line add box and Add with details
SHALL preselect that project.

#### Scenario: A filtered list

- **GIVEN** the task list filtered to `project:web`
- **WHEN** it is drawn
- **THEN** both add forms have `web` selected

### Requirement: A refused add keeps what was typed

A create the service refuses for a reason the reader can correct SHALL be answered with the task list
and the Add with details form drawn open in place, every submitted value kept, and the message under
the field the refusal names, or above the form when it names none. The response SHALL carry the
refusal's status. Adding that succeeds SHALL still return to the listing unless the browser chose to
open new tasks.

#### Scenario: An unknown parent

- **WHEN** the reader adds a task titled "write it down" with parent `infra-99`, which does not exist
- **THEN** the task list is drawn with the form open, the title still "write it down", and the refusal under the parent field

#### Scenario: A bad repeat rule

- **WHEN** the reader adds a task with the repeat rule `whenever`
- **THEN** the form is drawn open with More options open and the refusal under the repeat field

### Requirement: Copy to another project in the browser

The task screen and each lane card's more actions menu SHALL offer "Copy to…" to a reader who
may create tasks. It SHALL open a form
with the target project, defaulting to the task's own, the title prefilled and editable, and the
options keep status, with comments, with subtasks, with dependencies and link, link checked.

#### Scenario: Read-only reader

- **WHEN** a reader without task write opens OPS-3
- **THEN** no Copy to… action is drawn

### Requirement: The copy form uses the shared create form layout and keeps a refused submission

The copy form SHALL be drawn in the shared create form layout: the title as the lead field, the
project in the side column, the options as one checkbox group, Cancel and Copy at the bottom
right, Ctrl or Cmd+Enter to copy and Escape to cancel. A refused copy SHALL draw the form again
with every submitted value kept and the reason under the field it is about.

#### Scenario: A refused copy keeps the reader's choices

- **WHEN** a reader submits the copy form with subtasks ticked and a project that no longer exists
- **THEN** the form is drawn again with the same title and subtasks still ticked, and the reason is shown under the project field

### Requirement: After a browser copy the reader stays where the copy form was opened

The copy form SHALL be drawn in the task dialog when the task is open in one, and as a page
otherwise. After copying, the reader SHALL stay where the form was opened from, the source task
or the listing, with a toast linking the copy, unless "open after add" is on, in which case the
copy opens.

#### Scenario: Copy and stay

- **WHEN** a reader opens Copy to… on OPS-3, picks WEB and submits
- **THEN** the screen shows OPS-3 again with a toast linking the new WEB task

#### Scenario: Copy and open

- **WHEN** "open after add" is on and a reader copies OPS-3
- **THEN** the copy's task screen opens

#### Scenario: Copy from a lane card

- **WHEN** a reader opens a lane card's three dots menu on the lanes filtered by `tag:ops`, chooses Copy to… and submits
- **THEN** the reader returns to the lanes filtered by `tag:ops` with a toast linking the copy

### Requirement: The About card names the task a copy was copied from

The About card SHALL name the task a copy was copied from.

#### Scenario: Copied from on the About card

- **WHEN** a reader opens a linked copy of OPS-3
- **THEN** the About card shows "Copied from" with a link to OPS-3

### Requirement: The browser has a trash

The sidebar SHALL carry a Trash entry linking to the task list filtered by `is:trashed`, marked
current while that listing is shown, with Tasks not marked current at the same time. That listing
SHALL be titled Trash, SHALL say that its rows are deleted tasks, SHALL be drawn as the list whatever
view the reader chose, SHALL NOT narrow to open work, and SHALL NOT offer the quick-add row.

#### Scenario: Opening the trash

- **GIVEN** a tenant with a deleted task
- **WHEN** the reader follows Trash in the sidebar
- **THEN** the screen is titled Trash, lists the deleted task and no live task, and Trash is the current entry

#### Scenario: The trash ignores the board preference

- **GIVEN** a reader whose task screen is drawn as the board
- **WHEN** they open the trash
- **THEN** the deleted tasks are drawn as list rows

### Requirement: A deleted row offers Restore

A task row for a soft-deleted task SHALL be marked deleted, SHALL NOT offer the complete tick, the
status and priority menus or a link to the task screen, and SHALL offer a Restore button posting to the task's restore route with
the listing as the place to return to.

#### Scenario: Restoring from the trash

- **GIVEN** the trash listing two deleted tasks
- **WHEN** the reader presses Restore on one
- **THEN** that task is live again, the browser returns to the trash with a "task restored" message, and only the other task is listed

### Requirement: Selecting rows on the task list

Each row of the web task list SHALL carry a selection checkbox. With scripting on, shift-click
SHALL select the range from the last box clicked, `x` SHALL toggle the focused row's box,
Ctrl or Cmd+A SHALL select every row while focus is in the list, and Escape SHALL clear the
selection before it does anything else. A live refresh SHALL wait while any row is selected.

#### Scenario: Range selection

- **WHEN** a reader ticks the second row and shift-clicks the fifth
- **THEN** rows two to five are selected and the bar reads "4 selected"

#### Scenario: Escape clears

- **WHEN** three rows are selected and the reader presses Escape
- **THEN** no row is selected and the page does not navigate back

### Requirement: The bulk action bar

While rows are selected the task list SHALL show a sticky bar naming how many are selected and
offering: move to a status, priority, add tag, remove tag, assignee, star, unstar, copy to a
project, delete and clear. Each action SHALL be one request for the whole selection, after which
the list SHALL show a toast naming how many tasks changed and listing each refused task with its
reason. With scripting off the bar SHALL be drawn under the list and work the same way.

#### Scenario: Bulk move with a refusal

- **WHEN** a reader selects OPS-1 and WEB-2 and moves them to `done`, which WEB-2's workflow cannot reach
- **THEN** OPS-1 is done and the toast says one task moved and names WEB-2 with the reason

### Requirement: One Undo for a bulk delete

A bulk delete SHALL be soft and its toast SHALL offer one Undo that restores every task the
delete removed, in one request.

#### Scenario: Undo a bulk delete

- **WHEN** a reader deletes three selected tasks and presses Undo on the toast
- **THEN** all three tasks are back on the list

### Requirement: Export the list as CSV

The View menu of the task list SHALL offer Export CSV, which downloads every task the current
filter, deadline window, sort and project visibility select, up to 10000 rows, with the title
and the optional columns the reader's column picker shows, escaped by RFC 4180 and with the
formula guard.

#### Scenario: Export follows the column picker

- **WHEN** a reader has hidden the Tags column and exports the list filtered by `tag:ops`
- **THEN** the download holds only tasks tagged ops and has no tags column

### Requirement: The one-line add box reads quick-add syntax

The task listing's one-line add box SHALL read its line through the quick-add parser, using
the reader's chosen time zone, with a project the line names taking the place of the box's
project picker. Its (i) tip SHALL list the syntax. While the reader types, a row of chips
under the box SHALL show what the line will set, and a line that cannot be parsed, or that
names a project or handle the tenant does not have, SHALL show the refusal there.

#### Scenario: Preview

- **WHEN** the reader types `Fix login !high #auth` into the box
- **THEN** chips for priority high and tag `auth` appear under it

### Requirement: A refused quick add keeps its line and adding works without scripting

A refused add SHALL redraw the listing with the line still in the box and the refusal under it.
Adding SHALL work with scripting off.

#### Scenario: Refusal keeps the line

- **WHEN** the reader submits `Fix login @nobody` and no actor has that handle
- **THEN** the listing is drawn again with `Fix login @nobody` in the box and a message naming the handle under it, and no task is created

### Requirement: Task text renders as sanitised Markdown

The task screen SHALL render a task's description and each comment as CommonMark with the GitHub
tables, task lists, autolinks, strikethrough and fenced code. Raw HTML SHALL be shown escaped as the
text that was typed. The rendering SHALL carry no `style` attribute, no event handler attribute and no
script.

#### Scenario: Hostile text renders inert

- **GIVEN** a description containing `<script>`, an `onerror` attribute, a `javascript:` link and a
  `data:` image
- **WHEN** the task screen is drawn
- **THEN** the HTML is shown as escaped text, neither address is a link, and no element carries an
  event handler or a style

### Requirement: Task text links only to web and mail addresses

A link SHALL be rendered only when its address is absolute http or https with a host, or mailto, and
SHALL carry `rel="noopener noreferrer nofollow"`; any other address SHALL render as its label text.

#### Scenario: A relative address is not a link

- **GIVEN** a description containing `[a](https://example.com/x)`, `[b](mailto:ops@example.com)` and
  `[c](/relative)`
- **WHEN** the task screen is drawn
- **THEN** the first two are links carrying `rel="noopener noreferrer nofollow"` and `c` is plain text

### Requirement: An image in task text is never loaded

An image SHALL NOT be loaded: it SHALL render as a link to its address labelled by its alt text, or as
the alt text alone when the address is not allowed or the image is inside a link.

#### Scenario: An image is a link

- **GIVEN** a description containing `![chart](https://example.com/c.png)`
- **WHEN** the task screen is drawn
- **THEN** it shows a link to `https://example.com/c.png` labelled `chart` and no `<img>`

### Requirement: References and handles link to their pages

A task reference in task text whose project key belongs to a project the reader can see SHALL link to
that task's screen, and an `@handle` this tenant holds SHALL link to the task list filtered to that
assignee. Text inside code or an existing link SHALL NOT be linked.

#### Scenario: A known reference links and an unknown one does not

- **GIVEN** a project `infra` and a description reading `see infra-1 and covid-19`
- **WHEN** the task screen is drawn
- **THEN** `infra-1` links to `/tasks/infra-1` and `covid-19` is plain text

### Requirement: The editor offers a Preview

The comment composer SHALL stay a plain textarea, and the description editor SHALL keep one as its form
field. With scripting, each SHALL offer a Preview toggle (the description editor in its Markdown mode)
that renders the typed text through the same renderer without saving it, with checklist items drawn
inert. The preview route SHALL require the CSRF token.

#### Scenario: Preview writes nothing

- **WHEN** a writer previews a description
- **THEN** the rendered text is shown and the task's version is unchanged

### Requirement: A reader with task write ticks a checklist item in place

Each GitHub task-list item of a description SHALL render, for a reader with `task:write`, as a control
exposed as a checkbox with its checked state and named by the item's text, inside a form that carries
the CSRF token and the version the screen was drawn at. Activating it SHALL post the item's number and
the state asked for, and SHALL work without scripting. The service SHALL rewrite only that item's
marker.

#### Scenario: A tick lands on its item

- **GIVEN** a description with three items, the second open
- **WHEN** a writer ticks the second
- **THEN** only the second item's marker changes, and the screen shows 2 of 3 done

### Requirement: A stale checklist tick is refused and readers cannot tick

A post against a version the task has moved past SHALL be refused with the conflict refusal and change
nothing. A reader without `task:write` SHALL see disabled checkboxes and no form, and comments' items
SHALL always be disabled.

#### Scenario: A tick on a changed description is refused

- **GIVEN** a screen drawn at version 4 while the task is at version 5
- **WHEN** the writer ticks an item
- **THEN** the conflict refusal is shown and the description is unchanged

### Requirement: Rows and cards show how much of a checklist is done

The task list row, the board card, the lane card and the project board card of a task whose description
has checklist items SHALL show a badge reading done/total, and the board cards SHALL also show a progress
element for the same values. A task without items SHALL show neither.

#### Scenario: A card counts its checklist

- **GIVEN** a task whose description has one of three items ticked
- **WHEN** the board is drawn
- **THEN** its card shows `1/3` and a progress element at 1 of 3

### Requirement: Snooze from the browser

A reader with task write SHALL find Snooze on the task screen and in the lane card's more actions
menu, offering Later today, Tomorrow morning, Next week and a picked date
and time, resolved in the reader's time zone. A snoozed task SHALL offer Unsnooze in the same
places. Each choice SHALL be a plain form post that works without scripting, swaps in place under
htmx, and returns to the listing it came from. A reader without task write SHALL NOT be offered
either.

#### Scenario: Snooze from a lane card

- **GIVEN** a reader who may write tasks
- **WHEN** they choose Tomorrow morning in OPS-3's more actions menu
- **THEN** OPS-3 is snoozed until 09:00 tomorrow in their zone and leaves the lanes

#### Scenario: A picked time in the past

- **WHEN** a reader picks a date and time that has passed
- **THEN** the task is not snoozed and the refusal is shown where they asked

### Requirement: A snoozed task carries its wake time

Wherever a snoozed task is drawn, on its own screen and in a listing filtered to show snoozed
tasks, it SHALL carry a badge saying it is snoozed and until when, in the reader's time format.

#### Scenario: The task screen names the wake time

- **GIVEN** OPS-3 is snoozed until Friday 09:00
- **WHEN** a reader opens OPS-3
- **THEN** the screen shows that it is snoozed until Friday 09:00 and offers Unsnooze

### Requirement: Templates in the browser

The task screen's actions SHALL offer Save as template, a form taking the name, the scope and
whether subtasks are kept. A Templates screen in the Admin group SHALL list the templates a reader
can see with their scope and use count, and let a reader rename, edit the fields of, use and
delete each. The add box menu and the Add with details form SHALL offer New from template,
which takes a template, a title, a project for a tenant-wide template and a due offset.

#### Scenario: Save from the task screen

- **WHEN** a reader opens OPS-3, picks Save as template and saves it as "release"
- **THEN** the task screen returns with a confirmation and the Templates screen lists "release"

#### Scenario: New from template

- **WHEN** a reader picks New from template in the add box menu, chooses "release" and submits
- **THEN** the new task opens, holding the template's title and subtasks

### Requirement: The task screen is a header, a main column and a properties panel

The task screen SHALL draw a header holding the task's reference, its title, the reader's star and
one actions menu; a main column holding, in order, the description, the subtasks, the
dependencies, the artifacts, the comments and the history; and a properties panel. On a viewport
at least 1024 pixels wide the properties panel SHALL sit to the right of the main column.

#### Scenario: Wide screen

- **WHEN** a reader opens a task on a 1440 pixel wide screen
- **THEN** the description and comments are in the left column and the properties in the right

### Requirement: A narrow task screen puts the properties under the header

On a viewport narrower than 1024 pixels the properties panel SHALL come directly under the header
and the record's facts SHALL come last. The dialog a listing opens a task in SHALL draw the same
screen.

#### Scenario: Phone

- **WHEN** a reader opens a task on a 390 pixel wide screen
- **THEN** the properties follow the title and precede the description

### Requirement: Every property is one row of the properties panel

The properties panel SHALL hold one row per property, each a label and a value: status, priority,
snooze state when snoozed, claim state when claimed or once claimed, assignee, due date, project,
tags, repeat rule and each custom field the project defines, followed by the record's facts:
creator, created, updated, started and finished when set, version and identifier.

#### Scenario: An enum field

- **WHEN** a project defines an enum field
- **THEN** its row offers the field's options

### Requirement: A property is edited only in its own row and only by a writer

For a reader who may write, the status and priority SHALL open their menus, and the assignee, due
date, tags, repeat rule and each custom field SHALL be changed from a form in their own row that
names only that property. A reader who may not write SHALL see the same rows with no form in them.
No property SHALL be edited anywhere else on the screen.

#### Scenario: Changing the assignee in place

- **WHEN** a writer types a handle into the assignee row and saves
- **THEN** the task is assigned to that actor and no other property changes

#### Scenario: A reader who may not write

- **WHEN** a reader without write access opens a task
- **THEN** the properties panel holds no form

### Requirement: One actions menu holds every action on the task as a whole

The header SHALL open exactly one menu, and that menu SHALL hold Copy link, Copy the task id, and
for a writer Copy to, Save as template and Snooze (or Unsnooze for a snoozed task), Reclaim the
lease where the reader may reclaim a live lease, and Delete with its permanent and subtask options
where the reader may delete. None of these SHALL be drawn anywhere else on the screen.

#### Scenario: The menu

- **WHEN** a writer opens the actions menu
- **THEN** it offers Copy link, Copy the task id, Copy to, Save as template, Snooze and Delete

#### Scenario: No second copy

- **WHEN** the task screen is drawn
- **THEN** Copy to, Save as template and Delete each appear once, in the menu

### Requirement: The description edits in place

The description SHALL be drawn as rendered text with its checklist tickable. A writer SHALL be
offered an Edit switch that replaces the rendered text with the editor in place and reads Cancel
while open, and a task with no description SHALL offer Add a description in its place. The switch
SHALL work without a script. There SHALL be no separate edit form repeating the description or the
other properties.

#### Scenario: Editing

- **WHEN** a writer presses Edit over a description
- **THEN** the text gives way to the editor holding it, with Save and a Cancel switch

#### Scenario: Empty

- **WHEN** a writer opens a task with no description
- **THEN** the screen offers Add a description and draws no empty editor

### Requirement: The browser sets and clears a due date

The task form SHALL accept `due_at` as a date. A form naming it with a date SHALL set the due date
to that day; a form naming it blank SHALL clear it; a form that does not name it SHALL leave the
due date unchanged.

#### Scenario: Set, keep, clear

- **WHEN** a writer saves the due date row as 2099-03-04, then saves another property, then saves the due date row empty
- **THEN** the task is due 2099-03-04, still due then, and finally has no due date

### Requirement: A shortcut reaches a field behind a disclosure

A keyboard shortcut whose target is inside a closed disclosure SHALL open the disclosure before
focusing the target.

#### Scenario: Edit the title

- **WHEN** a reader presses the edit-title shortcut on a task screen
- **THEN** the Rename disclosure opens and the title field has focus

### Requirement: The task screen's star control states whether the reader starred the task

The task screen's star control SHALL state in words, through its accessible name and pressed
state, whether the reader has starred the task.

#### Scenario: An unstarred task's star control

- **GIVEN** a task the reader has not starred
- **WHEN** its screen is drawn
- **THEN** its star control is not pressed and is named Star followed by the task's reference

### Requirement: The browser follows the tenant's trash

The Tenant screen SHALL offer the trash setting with an explanation and, when the trash holds tasks,
how many. With the trash off, the sidebar SHALL NOT carry a Trash entry, the trash address SHALL show
the refusal in the filter bar, and a delete, single or bulk, that has not been confirmed SHALL answer
with a confirmation that names the tasks and says the delete cannot be undone, deleting nothing.

#### Scenario: The trash leaves the sidebar

- **GIVEN** a tenant whose trash is off
- **WHEN** the reader opens the task list
- **THEN** the sidebar has no Trash entry

### Requirement: A confirmed delete without the trash offers no Undo

A confirmed delete SHALL NOT offer Undo. The confirmation SHALL be decided by the delete handlers so it
applies to every place a Delete is drawn.

#### Scenario: Deleting with the trash off

- **GIVEN** a tenant whose trash is off
- **WHEN** the reader deletes a task
- **THEN** a confirmation is drawn and the task still exists, and confirming deletes it with no Undo

### Requirement: The description editor has Rich text and Markdown modes

With scripting, the description editor on the task screen and in the New task form SHALL offer Rich
text and Markdown modes. Markdown SHALL be the textarea with its Preview. Rich text SHALL edit the
same text formatted, with tools for bold, italic, strikethrough, headings, lists, checklists, quotes,
code, links and tables. Without scripting the textarea SHALL be all there is.

#### Scenario: Switching modes

- **GIVEN** a description with a table and a checklist
- **WHEN** a writer switches to Rich text and back to Markdown without editing
- **THEN** the textarea holds exactly the text it held before

### Requirement: Rich text submits Markdown

The textarea SHALL remain the form field in both modes, and Rich text SHALL write the Markdown it
serialises into it on every edit, so the form posts Markdown in the same field. Opening Rich text
without editing SHALL NOT change the field.

#### Scenario: Ticking a box in Rich text

- **WHEN** a writer ticks the second checklist item in Rich text and saves
- **THEN** the stored description has `- [x]` on that item and is otherwise the same text

### Requirement: A text Rich text would change stays in Markdown

Before opening a text, Rich text SHALL parse it, serialise it and compare the renderings. A text whose
rendering would change, or which holds a single-tilde strikethrough, SHALL stay in Markdown with a
notice saying why.

#### Scenario: A list item that opens with a code block

- **WHEN** a writer switches a description whose list item opens with a fenced code block to Rich text
- **THEN** the textarea stays, unchanged, with a notice that the text stays in Markdown

### Requirement: The chosen mode is remembered per browser

The chosen mode SHALL be kept in the browser's local storage and used for every description editor
the browser opens next. Markdown SHALL be the mode when nothing was chosen or storage is refused.

#### Scenario: Storage is refused

- **GIVEN** a browser that refuses local storage
- **WHEN** a task screen opens
- **THEN** the description editor is in Markdown mode

### Requirement: Rich text loads nothing a description names

Rich text SHALL NOT load an image a description names, and SHALL give a link an address only when it
is http, https or mailto. Raw HTML SHALL be shown as typed.

#### Scenario: An image in Rich text

- **WHEN** a description holding `![x](https://example.invalid/x.png)` opens in Rich text
- **THEN** no request is made for the image and its alt text is shown in its place

### Requirement: Mentions complete in Rich text

Typing `@` and part of a handle in Rich text SHALL offer the same completion list as the textarea,
chosen with the arrows and Enter or Tab and closed with Escape.

#### Scenario: Completing a handle

- **WHEN** a writer types `@to` in Rich text and presses Enter
- **THEN** the text reads `@tom` followed by a space, and the list closes

### Requirement: Task rows line their attributes up in columns

From a viewport 640px wide, every row of a task listing SHALL draw each shown attribute (status,
priority, tags, assignee, project, flags, reference) in a cell of the same width at the same
horizontal position as every other row, whatever the title, tags or reference of the row hold.

#### Scenario: A longer reference moves nothing

- **GIVEN** a listing at 1440px holding VUL-9 and VUL-10
- **WHEN** the list is drawn
- **THEN** the status, priority, assignee, project and reference of both rows start at the same x

### Requirement: A task row stays bounded whatever its content

A task row SHALL clamp its title to two lines with an ellipsis, SHALL cut a tag, handle or flag
too long for its cell with an ellipsis rather than wrapping it, SHALL never be wider than its
listing, and SHALL never render the task's description or any Markdown block in the row.

#### Scenario: A hostile task keeps the row's shape

- **GIVEN** a task with a 140 character title, two 60 character tags and a Markdown description
  holding a heading, a table, fenced code and an unbroken 150 character URL
- **WHEN** the list is drawn at phone, tablet and laptop widths
- **THEN** its row is no wider than the list, at most twice the median row height, and holds no
  paragraph, list, heading, table or code block

### Requirement: A live refresh keeps the elements still on screen

A live refresh SHALL patch the regions it redraws in place. Every row, card, board column, lane
and scrolling box present before and after SHALL remain the same element, only rows that are new
SHALL be inserted, and a disclosure the reader opened SHALL stay open.

#### Scenario: A burst while the reader scrolls

- **GIVEN** the task list, board or lanes scrolled down and sideways
- **WHEN** a burst of changes refreshes the screen
- **THEN** the scrolling boxes, the rows and the cards on screen are the same elements as before and the reader's focus stays on the element it was on

### Requirement: A live refresh holds the reader to what they were reading

After a live refresh each scrolled box SHALL show the row and the column the reader had at its top
and left edge before it, on both axes. Rows inserted above that row SHALL NOT move it; a pill
SHALL say how many arrived and scroll to them when pressed.

#### Scenario: New tasks above a scrolled list

- **GIVEN** the task list scrolled down
- **WHEN** three tasks that sort above the rows in view are created
- **THEN** the row at the top of the list does not move and a pill reads "3 new, show"

### Requirement: A burst redraws a listing at most twice a second

A listing's live refresh SHALL coalesce events over 500ms, drawing the first at once and the last
of a burst after it. A background refresh SHALL NOT show a busy pointer or mark anything busy, and
SHALL NOT add or remove a scrollbar on a box whose content did not change size.

#### Scenario: No flicker during a burst

- **WHEN** a burst of changes refreshes the task list
- **THEN** no element shows a progress pointer and no scrolling box gains or loses a scrollbar

### Requirement: The task screen filters by project in every view

The filter bar SHALL offer a project control on the list, the board and the lanes. It SHALL
submit `project` with one key or several comma separated, read as `project:` terms of the filter.
The view switch, the pager, the live refresh, CSV export and a saved filter SHALL keep it.

#### Scenario: A board of one project

- **WHEN** the reader picks a project whose workflow has its own states and switches to the board
- **THEN** only that project's cards are shown, in that workflow's columns, and the control still names the project

#### Scenario: Projects that disagree

- **WHEN** the reader asks for two projects whose workflows differ and the board is chosen
- **THEN** the screen explains that they cannot share a board and shows the list

### Requirement: A project-filtered screen listens only to that project

A task screen filtered to projects by their current keys SHALL subscribe to events of those
projects only, so a change elsewhere neither wakes nor appears on it.

#### Scenario: A change in another project

- **GIVEN** the board filtered to project web
- **WHEN** a task is created in project infra
- **THEN** the board's refresh does not draw it

### Requirement: The task screen hands the task to a coding agent

The task screen's header, as a page and in the task dialog, SHALL carry a "Hand to an agent" button
that opens a dialog with a Close control that Escape also closes. The dialog SHALL show a prompt that
names the task's reference, its title and the server's address, and the tix commands that claim,
show, comment on and release the task.

#### Scenario: The dialog names this task on this server

- **GIVEN** task INFRA-12 titled "Fix the build" on a server reached at `https://tix.example.com`
- **WHEN** a reader opens the task and presses "Hand to an agent"
- **THEN** a dialog shows a prompt naming INFRA-12, "Fix the build" and `https://tix.example.com`
- **AND** `tix claim task INFRA-12` is part of it

### Requirement: Every command a hand-off names is real

Every `tix` command and flag the hand-off prompt names SHALL exist in the CLI, and every tool the MCP
variant names SHALL be one `tix mcp` exposes.

#### Scenario: A renamed command fails the build

- **GIVEN** the prompt names `tix claim task`
- **WHEN** that command is renamed in the CLI
- **THEN** the test that resolves the prompt's commands against the CLI fails

### Requirement: Each documented client gets its own way in

The dialog SHALL carry one tab per client documented in `docs/mcp.md`. A client with a command that
starts it with a prompt SHALL show that command; a client without one SHALL show the prompt to paste.
The dialog SHALL also offer the prompt phrased for an agent with the tix MCP tools.

#### Scenario: Claude Code starts on the task

- **WHEN** the reader picks the Claude Code tab
- **THEN** it shows `claude` followed by the prompt as one quoted argument

### Requirement: A hand-off is safe to paste into a shell

A command in the dialog SHALL pass the prompt as one argument that a POSIX shell does not expand,
whatever the title holds: quotes, `$()`, backticks, `$` or line breaks. No hand-off text SHALL carry
an API token.

#### Scenario: A hostile title stays text

- **GIVEN** a task titled `` it's $(rm -rf ~) and `whoami` ``
- **WHEN** its Claude Code command is run by `/bin/sh`
- **THEN** `claude` receives the prompt with the title exactly as written, folded to one line

### Requirement: The dialog works without scripting and remembers the client

Each block in the dialog SHALL have a Copy button. With scripting, the last client tab chosen SHALL
be remembered per browser; without scripting every client's block SHALL be drawn as plain text.

#### Scenario: The last tab comes back

- **GIVEN** a reader who picked the Codex tab
- **WHEN** they open the dialog on another task
- **THEN** the Codex tab is the one shown
