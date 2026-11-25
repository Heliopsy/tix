## ADDED Requirements

### Requirement: The task screens share one compact filter toolbar

The task list and the project page SHALL draw one toolbar above the list from one partial: a filter
box, a Filters button, a Sort button, the view switch, the Select toggle where the screen has one, and
a More button. The filter box and every filter field SHALL belong to one GET form whose action is the
screen's own route. The project page SHALL NOT offer a project picker.

#### Scenario: The toolbar on the task list

- **WHEN** a reader opens `/tasks`
- **THEN** the page holds one form with class `filterbar` carrying the `q` box, and buttons opening the Filters, Sort and More popovers

#### Scenario: The toolbar on a project page

- **WHEN** a reader opens `/projects/infra`
- **THEN** the same toolbar is drawn, its form submits to `/projects/infra`, and the Filters popover has no project select

### Requirement: The toolbar's menus are popovers that work without scripting

Filters, Sort and More SHALL be elements with the `popover` attribute opened by buttons carrying
`popovertarget`, never `<details>` folds. Every field they hold SHALL submit with the filter form by
GET, or with its own form where it is a preference, so the toolbar works with scripting off.

#### Scenario: Filters opens with scripting off

- **GIVEN** a browser with scripting off
- **WHEN** the reader presses Filters, picks a status and presses Apply
- **THEN** the form is submitted by GET and the listing is answered under the picked status

### Requirement: The Filters popover holds a picker per common term

The Filters popover SHALL offer the project (on the task list only), assignee, tag, status,
priority, deadline window and a control to include finished work. The status choices SHALL be the
states of the tenant's workflows, and each picker SHALL show the term the expression holds when it
holds exactly one, and "as typed" when it holds several.

#### Scenario: A picker shows the term in force

- **GIVEN** the address `/tasks?q=status:doing`
- **WHEN** the reader opens Filters
- **THEN** the status picker shows `doing` as selected

### Requirement: A picked term is folded into the expression

A request carrying a changed status, priority or assignee picker SHALL replace the expression's
selecting terms of that key with the picked one, or remove them when the empty choice is picked; a tag
picker SHALL add a `tag:` term. The screen SHALL answer with a redirect to its route carrying the new
`q` and no picker fields or page position.

#### Scenario: Picking a priority

- **GIVEN** the address `/tasks?q=tag:ops+priority:low`
- **WHEN** the reader picks high in Filters and applies
- **THEN** they are redirected to `/tasks?q=tag:ops+priority:high`

#### Scenario: An untouched picker leaves a typed term alone

- **GIVEN** the Filters popover drawn for `q=status:todo`
- **WHEN** the reader types `status:done` into the box and presses Enter without touching the status picker
- **THEN** the listing is answered under `status:done`

### Requirement: Active filter terms are shown as removable chips

Under the toolbar, the screen SHALL draw one chip per active term: the deadline window, each project
the project control names, each term of the expression in the order typed, and the include finished
choice, followed by Clear all. A chip's remove control SHALL be a plain link to the same screen
without that one term, keeping every other term.

#### Scenario: Removing one chip

- **GIVEN** the address `/tasks?q=tag:ops+-tag:chore+@raj&sort=title`
- **WHEN** the reader follows the remove link of the `-tag:chore` chip
- **THEN** the screen is `/tasks?q=tag:ops+@raj&sort=title`

#### Scenario: No filter, no chips

- **WHEN** a reader opens `/tasks` with no filter
- **THEN** no chip row and no Clear all are drawn

### Requirement: The toolbar's links keep the address rules

A chip link, Clear all and the view switch SHALL keep the sort, its direction, the view, the lanes
grouping, the page size and select mode. A chip link, Clear all and a submitted filter SHALL drop the
cursor, its trail and the page number, so a changed filter starts at the first page.

#### Scenario: A chip keeps the view and resets the page

- **GIVEN** the address `/tasks?q=tag:ops&view=board&page=3`
- **WHEN** the reader removes the `tag:ops` chip
- **THEN** the screen is `/tasks?view=board` at its first page

### Requirement: The Sort popover sets the order and the grouping

The Sort popover SHALL offer the sort field and a direction, submitted as `sort` and `order=desc`,
with the list grouping when the list is drawn and Group lanes by when the lanes are. The pager, the
view switch and Clear all SHALL keep the direction.

#### Scenario: Sorting by title, descending

- **GIVEN** tasks titled alpha and beta
- **WHEN** the reader opens `/tasks?sort=title&order=desc`
- **THEN** beta is listed before alpha

### Requirement: The More popover holds the rest of the screen's settings

The More popover SHALL hold the column choices with Export CSV, the page size, saving the current
filter, the reader's saved filters as links, the hidden groups with their reset, and project
visibility. Saving SHALL be offered only while a filter is in force.

#### Scenario: Saved filters are one press away

- **GIVEN** a reader with a saved filter named Late ops
- **WHEN** they open More on `/tasks`
- **THEN** a link to Late ops is listed

### Requirement: The filter box suggests terms from the tenant

The filter box SHALL suggest values for `project:`, `tag:`, `@`, `assignee:`, `status:`, `priority:`
and `due:` from the tenant's projects, tags, actors and workflows, and the term keys for a bare
word, from data embedded in the page. Arrows SHALL move through the suggestions, Enter or Tab SHALL
take the active one, and Escape SHALL close the list.

#### Scenario: Enter applies when nothing is chosen

- **GIVEN** the suggestions open under `tag:o`
- **WHEN** the reader presses Enter without pressing an arrow
- **THEN** the filter is applied as typed

#### Scenario: Taking a suggestion

- **GIVEN** a tenant with a project whose key is fidaro
- **WHEN** the reader types `pro`, presses Down to reach `project:` and Tab, then types `fi` and takes `project:fidaro`
- **THEN** the box reads `project:fidaro` followed by a space, with the caret after it

### Requirement: The toolbar stays compact

With no filter in force, the toolbar SHALL be one row on a desktop window and at most two rows at
phone width, its controls SHALL share one height, and the task screens SHALL NOT scroll sideways at
375px in either language or any theme.

#### Scenario: A desktop window

- **GIVEN** a 1280 by 800 window and no filter in force
- **WHEN** the reader opens `/tasks`
- **THEN** the toolbar is no taller than an eighth of the window
