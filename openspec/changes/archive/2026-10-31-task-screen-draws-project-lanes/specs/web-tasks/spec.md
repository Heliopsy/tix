## ADDED Requirements

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

### Requirement: A lane card offers a tick box only for one unambiguous completion

A lane card for an unfinished task SHALL offer a tick box that completes the task only when the
task's own project's workflow offers exactly one shortest route from the task's current state to a
terminal state of the done category, when no step of that route requires a comment, and when the
reader holds the scope to transition tasks and every scope a step of the route requires.

Ticking the box SHALL submit that route to the task's transition route as a form post carrying the
CSRF token and the listing to return to, and SHALL work without scripting. Where the route passes
through other states the box SHALL name them in its accessible name before it is applied.

Where the box is not offered, the card SHALL instead offer a link to the task's own screen whose
accessible name says why it cannot be completed from the lane. A task already in a terminal state
SHALL render ticked and SHALL NOT offer a control that reopens it.

#### Scenario: Ticking completes the task along its only route

- **GIVEN** a task in `todo` in the built-in workflow
- **WHEN** the reader ticks its box in the lanes view
- **THEN** the task moves through `doing` to `done`
- **AND** the reader returns to the same listing

#### Scenario: Two done states withhold the box

- **GIVEN** a workflow in which two terminal done-category states are reachable from the task's state
- **WHEN** the lane card is drawn
- **THEN** no tick box is offered and the card links to the task instead

#### Scenario: A route that needs a comment withholds the box

- **GIVEN** a workflow whose only route to done requires a comment
- **WHEN** the lane card is drawn
- **THEN** no tick box is offered and the link says a comment is needed

#### Scenario: A reader without the scope is not offered the box

- **GIVEN** a reader who cannot transition tasks
- **WHEN** the lane card is drawn
- **THEN** no tick box is offered

#### Scenario: A finished task renders ticked

- **GIVEN** a task in a terminal state
- **WHEN** its lane card is drawn
- **THEN** it is marked ticked and offers no form

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
the board, keeping the view, filter, sort, page and visibility choice. Lanes SHALL scroll sideways
inside their own frame rather than scrolling the page, and at phone width each lane SHALL take the
width of the screen.

#### Scenario: A refresh keeps the lanes

- **GIVEN** a reader on the lanes view
- **WHEN** the listing fragment is fetched for a live refresh
- **THEN** it draws lanes holding the tasks on screen
