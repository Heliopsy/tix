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

The control SHALL show the window currently selected. Its selection SHALL be carried across pages of the
listing. A window the filter language does not accept SHALL be reported beside the filter box, with the
listing withheld, like any other refused expression.

#### Scenario: Selecting overdue narrows the listing

- **GIVEN** one task whose deadline has passed, one whose deadline is far off, and one with no deadline
- **WHEN** the reader selects the overdue window
- **THEN** only the task whose deadline has passed is listed

#### Scenario: The control shows its own state

- **WHEN** the listing is rendered for a selected window
- **THEN** that window is the selected option of the control

#### Scenario: The control and the box ask the same question

- **WHEN** the same deadline term is typed into the filter box instead
- **THEN** the same tasks are listed

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

Starred tasks SHALL be drawn first in the list, first within each board column and first within each
lane.

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

#### Scenario: A starred task leads its lane

- **GIVEN** the lanes view with two tasks in one project, the later one unstarred
- **WHEN** the reader stars the later one from its lane card
- **THEN** that card is drawn first in its lane with its button pressed

### Requirement: The browser sets and shows a repeat rule

The task screen's edit form and the add-task form SHALL offer a Repeat field taking the rule syntax, with a
hint naming an example. Saving the edit form with the field emptied SHALL remove the rule; a form that does
not carry the field SHALL leave the rule unchanged. A rule the service refuses SHALL be reported on the
form rather than dropped. The task screen SHALL state the rule and, once the task has spawned, link its
successor.

A listing row and a board card for a task carrying a rule SHALL show a small repeat marker whose
accessible name states the rule. Completing a recurring task from the browser SHALL create the next
occurrence exactly as on every other surface.

#### Scenario: Setting a rule from the task screen

- **WHEN** a reader enters `every week` in the Repeat field and saves
- **THEN** the task carries `every week` and the screen states it

#### Scenario: Marker on a row

- **WHEN** the listing shows a task carrying `every month`
- **THEN** its row shows the repeat marker naming `every month`, and a row of a task with no rule shows none

#### Scenario: Refused rule

- **WHEN** a reader saves the Repeat field as `every fortnight`
- **THEN** the form reports the refusal and the task is unchanged

### Requirement: The task screen states the star, the parent and the next occurrence

The task screen SHALL state in words whether the reader has starred the task. A subtask's screen
SHALL link the task it belongs to. A repeating task's screen SHALL state its rule and, when the next
occurrence exists, link it with its due date; when it does not exist yet, SHALL state the date
finishing the task now would give it, computed by the same rule completion applies; and when the
rule has run out, SHALL say that this is the last occurrence. When the task has started or finished,
the screen SHALL say when.

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
- **THEN** it says the task is starred and links its parent

### Requirement: The task screen can draw one lane per project

The task screen SHALL offer a lanes view that draws the selected page of tasks as one lane per
project. The lanes SHALL hold exactly the tasks the list would draw for the same filter, deadline
window, sort, page, page size and project visibility choice, except that the open-work default the
list applies SHALL NOT narrow the lanes, so finished tasks are drawn in their lanes.

Lanes SHALL be ordered by the order the screen lists projects in, and the tasks within a lane SHALL
keep the listing's order. A lane SHALL be drawn for every project with a task on the page, and for
every project a `project:` term in the filter names, even when it holds no task. No other empty lane
SHALL be drawn. A task on the page whose project the screen cannot name SHALL be reported on the
screen rather than omitted.

#### Scenario: Tasks are grouped into their own project's lane

- **GIVEN** tasks in two projects
- **WHEN** the reader draws the lanes
- **THEN** each project has one lane holding exactly its own tasks
- **AND** the lanes follow the order the screen lists projects in

#### Scenario: A project put away has no lane

- **GIVEN** a reader who has put one project away
- **WHEN** they draw the lanes
- **THEN** no lane and no card of that project is drawn

#### Scenario: A project named by the filter has a lane even when empty

- **GIVEN** a filter naming a project with `project:` that holds no matching task
- **WHEN** the lanes are drawn
- **THEN** that project's lane is drawn and says it holds no tasks

#### Scenario: An empty project not named by the filter has no lane

- **GIVEN** a project with no task on the page and no `project:` term naming it
- **WHEN** the lanes are drawn
- **THEN** no lane is drawn for it

#### Scenario: Finished tasks are in their lanes

- **GIVEN** a project with one open and one finished task and no filter
- **WHEN** the lanes are drawn
- **THEN** both tasks are in the project's lane

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

The stored choice SHALL be applied before the lanes are painted and SHALL survive a live refresh of
the listing. Storage that cannot be read SHALL read as the default. Without scripting the toggle
SHALL NOT be shown and finished cards SHALL be shown.

#### Scenario: Hiding finished cards is remembered

- **GIVEN** a reader on the lanes view
- **WHEN** they turn Show completed off
- **THEN** finished cards are hidden
- **AND** local storage holds `false` under `tix.lanes.showCompleted` and nothing else for lanes

#### Scenario: Turning it back on stores nothing

- **GIVEN** a browser that hid finished cards
- **WHEN** the reader turns Show completed on
- **THEN** the key is removed and finished cards are shown

#### Scenario: Storage that throws reads as shown

- **GIVEN** a browser whose local storage refuses access
- **WHEN** the lanes are drawn
- **THEN** finished cards are shown

### Requirement: Lanes stay current and fit the screen

The lanes view SHALL refresh from the event stream through the same listing fragment as the list and
the board, keeping the view, filter, sort, page and visibility choice. From 640px wide, lanes SHALL
scroll sideways inside their own frame rather than scrolling the page. Below 640px each lane SHALL
take most of the width of the screen, the lanes SHALL snap sideways one lane at a time, and each lane
SHALL be its natural height so that the page, not the lane, scrolls vertically. On a coarse pointer
the tick, status and priority controls of a lane card SHALL be at least 40px in both dimensions.

#### Scenario: A refresh keeps the lanes

- **GIVEN** a reader on the lanes view
- **WHEN** the listing fragment is fetched for a live refresh
- **THEN** it draws lanes holding the tasks on screen

#### Scenario: A phone scrolls the page, not the lane

- **GIVEN** a viewport 390px wide
- **WHEN** the lanes are drawn
- **THEN** a lane does not scroll vertically inside itself and the lanes snap sideways

### Requirement: A lane card's tick completes and reopens along its workflow

A lane card's tick SHALL complete an unfinished task and reopen a finished one, following the task's
own project's workflow. For an unfinished task the candidate routes SHALL be the shortest routes from
its state to each terminal state of the done category. For a task in a terminal state the candidate
routes SHALL be the routes to non-terminal states that have the fewest steps.

A candidate SHALL be usable only when no step of it requires a comment and the reader holds the
scope to transition tasks and every scope a step requires. When there is exactly one candidate and
it is usable, the tick SHALL be a box that applies it. When there is more than one candidate, the
tick SHALL open a menu offering every usable candidate and naming the states each passes through.
When no candidate is usable, or there is none, or the reader cannot transition tasks, the tick SHALL
open a panel that says why in visible text and links to the task's own screen.

Every tick SHALL submit its route to the task's transition route as a form post carrying the CSRF
token and the listing to return to, and SHALL work without scripting. Where a route passes through
other states the control SHALL name them in its accessible name before it is applied. A finished
card SHALL render ticked whichever control it offers.

#### Scenario: Ticking completes the task along its only route

- **GIVEN** a task in `todo` in the built-in workflow
- **WHEN** the reader ticks its box in the lanes view
- **THEN** the task moves through `doing` to `done`
- **AND** the reader returns to the same listing

#### Scenario: Unticking reopens the task along its nearest route

- **GIVEN** a task in `done` in the built-in workflow
- **WHEN** the reader ticks its box again
- **THEN** the task moves to `todo`

#### Scenario: Two equally near open states offer the choice

- **GIVEN** a workflow in which two non-terminal states are each one step from a terminal state
- **WHEN** the lane card of a task in that terminal state is drawn
- **THEN** the tick opens a menu offering both routes and applies none by itself

#### Scenario: Two done states offer the choice

- **GIVEN** a workflow in which two terminal done-category states are reachable from the task's state
- **WHEN** the lane card is drawn
- **THEN** the tick opens a menu offering both routes

#### Scenario: A route that needs a comment says so

- **GIVEN** a workflow whose only route to done requires a comment
- **WHEN** the lane card is drawn
- **THEN** no route is submitted by the tick
- **AND** its panel says a comment is needed and links to the task

#### Scenario: A workflow with no way back says so

- **GIVEN** a workflow with no transition out of its done state
- **WHEN** the lane card of a finished task is drawn
- **THEN** the card renders ticked and its panel says no route leads back to an open state

#### Scenario: A reader without the scope is not offered a route

- **GIVEN** a reader who cannot transition tasks
- **WHEN** the lane card is drawn
- **THEN** no route is submitted by the tick and its panel says they cannot move tasks

### Requirement: A lane card's status and priority are menus

A lane card SHALL draw its status badge as a control that opens a menu of the routes the reader may
take from the task's state in its own project's workflow, omitting any route that needs a comment or
a scope the reader lacks, each submitted to the task's transition route. A reader who cannot
transition tasks, or a card with no route to offer, SHALL see the badge as a label.

A lane card SHALL draw its priority as a control that opens a menu of every priority, marking the
current one, each submitted to the task's priority route, for a reader who may write tasks. A reader
who may not SHALL see the priority as a label, and a normal priority SHALL then not be drawn.

Both menus SHALL work as plain forms without scripting and SHALL be operable from the keyboard.

#### Scenario: The status menu offers the workflow's moves

- **GIVEN** a task in `todo` in the built-in workflow and a reader who can transition tasks
- **WHEN** the reader opens the card's status menu
- **THEN** it offers `doing`, `blocked` and `cancelled` and the route through `doing` to `done`
- **AND** choosing `blocked` moves the task to `blocked`

#### Scenario: A move needing a comment is left out

- **GIVEN** a workflow whose transition into `blocked` requires a comment
- **WHEN** the card's status menu is drawn
- **THEN** it does not offer `blocked`

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
show the refusal inside it. A live refresh SHALL NOT replace the listing while a lane action is in
flight or a menu in it is open, and SHALL return focus to the element that held it when it lands.

#### Scenario: A tick swaps one card

- **GIVEN** a reader on the lanes view with scripting
- **WHEN** they tick a card
- **THEN** only that card is replaced and focus is on its tick

#### Scenario: A refusal is shown on the card

- **GIVEN** a card whose task another reader has just moved where the route no longer applies
- **WHEN** the reader ticks it
- **THEN** the card stays and says why the move was refused

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
