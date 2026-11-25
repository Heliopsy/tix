## ADDED Requirements

### Requirement: The project page offers every view of the task screen

`/projects/{key}` SHALL draw the task screen narrowed to that project, with the same List, Board and
Lanes, filter bar, sort, deadline window, saved filters, View panel, bulk bar, quick add, pager and
live refresh, produced by the same handler and template as `/tasks`, under the project's own header.

#### Scenario: A project as a filtered list

- **WHEN** a reader opens `/projects/infra?view=list&q=tag:ops`
- **THEN** the rows are the open infra tasks tagged ops, and no task of another project is drawn

#### Scenario: The board of a project follows the filter

- **WHEN** a reader opens `/projects/infra?view=board&q=tag:ops`
- **THEN** the board has the infra workflow's columns and only the tasks tagged ops on its cards

### Requirement: The project page keeps its own address in every link it writes

The scoped screen SHALL write `/projects/{key}` into its filter form, Clear, pager, show finished
link, row return addresses and view switch, and `/projects/{key}/live` into its live binding,
subscribed to that project alone. It SHALL ignore a `project` parameter and SHALL NOT offer the project
select.

#### Scenario: Paging a project list

- **WHEN** a reader follows Next on a project's list
- **THEN** they stay on `/projects/{key}` with the filter and sort kept

### Requirement: The view travels in the address

A `view` query parameter of `list`, `board` or `lanes` SHALL choose the drawing and SHALL win over
the remembered choice. Without it the task list SHALL draw the remembered view and the project page
its board. A GET of `/tasks` naming a view SHALL remember it for this browser.

#### Scenario: A shared link to a board

- **WHEN** a reader whose remembered view is the list opens `/tasks?q=tag:ops&view=board`
- **THEN** the board is drawn with the filter applied

### Requirement: The view switch keeps the filter

Every position of the view switch, on the task list and on a project page, SHALL be a link to the
same screen carrying the current query with its own `view`, minus the cursor and the flash. A
parameter the target view does not use SHALL stay in the link, so switching back restores it.

#### Scenario: Board and back

- **WHEN** a reader on the list filtered by `q`, `due`, `sort` and `project` follows Board and then List
- **THEN** each address carries the same `q`, `due`, `sort` and `project`, and the list is filtered as before

#### Scenario: Without scripting

- **WHEN** a browser without scripting follows the switch
- **THEN** the new view is drawn with the filter kept, by an ordinary link

### Requirement: An in-place filter redraws the switch

With scripting on, when the filter bar narrows the listing in place, the view switch SHALL be redrawn
from the same response, so its links carry the new query. The live refresh SHALL keep replacing only
the summary and the listing.

#### Scenario: Typing a filter, then switching

- **WHEN** a reader types a filter, waits for the listing to narrow and follows Board
- **THEN** the board is drawn under that filter
