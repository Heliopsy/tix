## ADDED Requirements

### Requirement: Saved filters in the sidebar

The filter bar SHALL offer "Save this filter" with a name and a share choice whenever it holds an
expression or a deadline window, saving the expression, the deadline window, the sort and the view in
force. The sidebar SHALL list the reader's saved filters under Tasks, own first, then shared, each a link
that opens the task screen on the filter and sets the view it names, marked current when the screen shows
exactly that filter. A filter the reader may change SHALL carry a three dots menu that renames, shares or
stops sharing, moves up or down among the reader's own, and deletes; its forms work without scripting and
redraw the sidebar in place with the heading menus' focus return and keyboard rules.

#### Scenario: Save from the filter bar

- **GIVEN** the task screen filtered with `tag:ops` and sorted by priority on the board
- **WHEN** the reader saves it as "Ops"
- **THEN** "Ops" is listed in the sidebar, and opening it later shows the board filtered and sorted the same way

### Requirement: My work

The sidebar SHALL carry a My work entry showing the reader's open tasks assigned to them as the list,
grouped under Overdue, Today, This week, Later and No date read in the reader's time zone, leaving out an
empty group. A switch SHALL add the tasks the reader holds a claim on. The groups SHALL be arranged like
any list group, per reader, under the view `mine`.

#### Scenario: Grouped by deadline

- **GIVEN** a task of mine due yesterday, one due next month and one with no deadline
- **WHEN** I open My work
- **THEN** they appear under Overdue, Later and No date, in that order

### Requirement: Recently viewed

The sidebar SHALL list the last ten tasks this browser opened for the signed-in reader, newest first, in a
collapsible section, kept in the browser's own storage and drawn by script, so with scripting off it is
not offered. `tix.recentTasks()` SHALL answer the same list as `{ref, title, href, at}` entries.

#### Scenario: A task opened again moves to the top

- **GIVEN** a reader opened tasks A then B
- **WHEN** they open A again
- **THEN** the list reads A, B

### Requirement: The rail draws the reader's entries as icons

On the collapsed sidebar rail, My work and each saved filter SHALL be drawn as an icon keeping its name
for assistive technology, the saved filter menus SHALL be put away, and the recent section SHALL fold to
an icon that opens the sidebar.

#### Scenario: Collapsed rail

- **WHEN** the reader collapses the sidebar
- **THEN** My work and every saved filter are icons with their names as accessible names
