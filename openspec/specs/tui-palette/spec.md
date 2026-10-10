# tui-palette Specification

## Purpose

Provides the terminal command palette, so every operation is reachable by name without memorising a binding.

## Requirements

### Requirement: One key opens a palette of every action

The terminal interface SHALL bind a command palette to a key that works in every view, and SHALL name that
key in the help overlay.

The palette SHALL list the actions the interface performs, each under a human-readable name and each beside
the key that performs it.

A keybinding scheme that has already spent the palette's default key on another action SHALL bind the
palette to a key that scheme leaves free, and no scheme SHALL leave the palette unbound.

#### Scenario: The palette opens from any view

- **WHEN** the palette key is pressed on the board, the project list or the task detail
- **THEN** the palette opens over the view it was pressed from
- **AND** lists actions by name with each action's own key beside it

#### Scenario: Every shipped scheme can open the palette

- **WHEN** each shipped keybinding scheme is loaded
- **THEN** the palette carries at least one key
- **AND** no key in that scheme is bound both to the palette and to another action of the same view

### Requirement: The palette, the help overlay and the key press read one list

The bindings, names and requirements of the interface's actions SHALL be held in one list. The help
overlay, the palette and the resolution of a key press SHALL all be derived from that list.

Performing an action SHALL have exactly one implementation, reached both by its key press and by its
palette entry.

#### Scenario: The palette offers what the overlay documents

- **WHEN** the palette and the help overlay are rendered for the same reader with a task selected
- **THEN** every action the overlay documents for that reader is offered by the palette
- **AND** the palette offers no action the overlay does not document

#### Scenario: Every listed action has an implementation

- **WHEN** each action in the list is performed
- **THEN** the interface reports the action as handled

#### Scenario: Every listed action is reached by its own key

- **WHEN** an action's first key is pressed
- **THEN** the press resolves to that action and to no other

#### Scenario: A palette entry takes the key's own path

- **WHEN** an action is chosen from the palette
- **THEN** the resulting state is the state that pressing the action's key produces

### Requirement: The palette offers only what the reader may do

The palette SHALL offer an action that opens a view only when the reader is offered that view, decided by
the same predicate the help overlay filters with.

The palette SHALL offer an action on the selected task only when the reader holds the authority the
action's operation needs, decided by the same predicate the footer filters with.

An action that acts on the selected task SHALL NOT be listed while no task is selected, and an action that
needs an open project SHALL NOT be listed while no project is open. Such an entry SHALL be omitted rather
than listed in a disabled form.

The palette SHALL mark the entry for the view the reader is currently in.

#### Scenario: A reader refused a view is not offered its entry

- **WHEN** the palette is opened by a reader who may not subscribe to events
- **THEN** no entry opens the activity view
- **AND** the entries for the views that reader may enter are still listed

#### Scenario: A reader refused an operation is not offered its entry

- **WHEN** the palette is opened by a reader who may not claim a task
- **THEN** no entry claims the selected task

#### Scenario: An action on the selection is hidden with nothing selected

- **WHEN** the palette is opened with no task selected
- **THEN** no entry acts on a selected task

#### Scenario: The open view is marked

- **WHEN** the palette is opened from the activity view
- **THEN** the entry that opens the activity view is marked as the current one

### Requirement: The palette is narrowed by a query over the action names

The palette SHALL narrow its entries by a case-insensitive match of a typed query against the action's
name, where every whitespace-separated word of the query must appear in the name and the words may appear
in any order.

An empty query SHALL list every entry the reader is offered.

A query that matches no entry SHALL render a line stating that nothing matches, rather than an empty list.

Narrowing SHALL NOT reorder the entries that remain.

#### Scenario: A query narrows to the matching entries

- **WHEN** the reader types a word that appears in some action names
- **THEN** only the entries whose names contain that word are listed

#### Scenario: Words may be given in any order

- **WHEN** the reader types two words that both appear in one action's name in the other order
- **THEN** that action is listed

#### Scenario: An empty query lists everything

- **WHEN** the palette is opened and nothing is typed
- **THEN** every entry the reader is offered is listed

#### Scenario: A query matching nothing says so

- **WHEN** the reader types a query no action name contains
- **THEN** the palette states that no action matches the query

### Requirement: The palette is driven by the interface's own keys

The palette SHALL run the highlighted entry on the accept key, cancel on the cancel key, and move the
highlight on the up and down keys, taking each from the interface's key map rather than from a literal.

Where a movement key is also a printable character, that character SHALL be typed into the query rather
than moving the highlight.

Cancelling the palette SHALL leave the view and the selection exactly as they were.

#### Scenario: Enter runs the highlighted entry

- **WHEN** an entry is highlighted and the accept key is pressed
- **THEN** the palette closes and that action is performed

#### Scenario: Escape leaves nothing behind

- **WHEN** the palette is open and the cancel key is pressed
- **THEN** the palette closes
- **AND** the view is the view it was opened from

#### Scenario: A letter bound to a movement key is typed

- **WHEN** the palette is open and a letter that the up key is also bound to is pressed
- **THEN** the letter is added to the query and the highlight does not move

### Requirement: The footer names the palette key

The footer SHALL name the palette's key in every view, whenever no input mode is open.

The hint SHALL be placed so that a terminal too narrow for the whole footer drops the view's own bindings
before it drops the palette hint.

A terminal too narrow to carry the whole hint SHALL omit it rather than render a truncated key.

#### Scenario: Every view advertises the palette

- **WHEN** a view's footer is rendered with no input mode open
- **THEN** it names the palette's key and what it opens

#### Scenario: The hint survives a narrow terminal

- **WHEN** the footer is rendered at the narrowest width the interface draws at
- **THEN** the palette hint is present in the rendered footer

#### Scenario: A hint that cannot fit is dropped whole

- **WHEN** the footer is rendered at a width narrower than the hint's own text
- **THEN** the hint is absent rather than truncated
