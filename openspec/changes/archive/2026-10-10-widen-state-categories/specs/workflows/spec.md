# Workflows

## ADDED Requirements

### Requirement: A state category comes from a fixed vocabulary

A workflow state SHALL carry its category from a closed vocabulary of exactly six words: `todo`,
`in_progress`, `blocked`, `waiting`, `done` and `cancelled`. A state MAY carry no category at all.

The vocabulary SHALL be fixed rather than extensible, and a state SHALL NOT carry a colour of its own,
so that a category means the same thing in every deployment and on every surface.

#### Scenario: Each of the six is accepted

- **WHEN** a workflow declares a state whose category is any of the six
- **THEN** the workflow validates

#### Scenario: A state may decline to name a category

- **WHEN** a workflow declares a state carrying no category
- **THEN** the workflow validates
- **AND** the state's category is derived from what else it carries: a terminal state reads as `done`,
  the initial state as `todo`, and any other state as `in_progress`

#### Scenario: A word outside the vocabulary is refused

- **WHEN** a workflow declares a state whose category is a word the vocabulary does not contain
- **THEN** the workflow is refused as invalid
- **AND** the refusal names the state and the word it refused
- **AND** the refusal lists all six accepted categories

#### Scenario: The vocabulary cannot be edited by a caller

- **WHEN** a caller obtains the vocabulary and modifies the value it was handed
- **THEN** the next caller still receives the whole unmodified vocabulary

### Requirement: Cancelled work is categorised apart from completed work

A state for work that will not be done SHALL carry the category `cancelled`, which SHALL be a distinct
value from `done`. A state for work stalled on this board SHALL carry `blocked`, distinct from both
`todo` and `waiting`.

#### Scenario: The shipped workflow categorises its own states

- **WHEN** the shipped workflow is read
- **THEN** its `cancelled` state carries the category `cancelled`
- **AND** its `blocked` state carries the category `blocked`

### Requirement: A state's category does not decide whether it is terminal

Terminal-ness SHALL be decided by the state's terminal flag alone. No judgement about whether a task is
finished — the completion timestamp, the release of its dependents, what the claim path will hand out, or
which states satisfy a dependency — SHALL read the state's category.

#### Scenario: A cancelled state stays terminal

- **WHEN** a state carries the category `cancelled` and is declared terminal
- **THEN** it is reported among the workflow's terminal states
- **AND** a task moved into it is given a completion timestamp
- **AND** its dependents are released

#### Scenario: Recategorising does not change what a claim will hand out

- **WHEN** a state's category changes and its terminal flag does not
- **THEN** the claim path's judgement about that state is unchanged

### Requirement: A stored workflow keeps the categories it declared

The system SHALL NOT rewrite or re-derive the categories of a workflow that is already stored. A
workflow that declares a category SHALL be reported under that category, whatever the shipped workflow
declares for a similarly named state.

#### Scenario: A hand-authored workflow calling its cancelled state done is honoured

- **GIVEN** a stored workflow whose cancelled state declares the category `done`
- **WHEN** a task is moved into that state and the statistics are read
- **THEN** the task is counted in the `done` row
- **AND** it is not counted in the `cancelled` row

#### Scenario: An existing installation is not migrated

- **WHEN** an installation that already holds the shipped workflow starts
- **THEN** the stored workflow is left exactly as it is
- **AND** only a fresh installation receives the widened categories

### Requirement: Every category is drawn distinctly and degrades to its word

Every surface that colours a workflow state SHALL derive that colour from the state's category and from
nothing else. No two categories SHALL be drawn identically on a surface that draws colour at all. A
category the build does not recognise SHALL be drawn plainly rather than given a guessed colour.

Where a surface writes no colour, every category SHALL be drawn with no styling and SHALL remain
identifiable by its word.

#### Scenario: The six are mutually distinct on a colour terminal

- **WHEN** a board, a command-line listing or a browser page draws all six categories in colour
- **THEN** each is drawn in a style no other category is drawn in
- **AND** `cancelled` is not drawn in the style `done` is drawn in
- **AND** `blocked` is not drawn in the style `todo` is drawn in

#### Scenario: The terminal and the board agree

- **WHEN** a category is drawn on the board and on the command line
- **THEN** both write the same terminal colour for it

#### Scenario: A colourless terminal writes no attribute

- **WHEN** a board is drawn for a terminal that writes no colour
- **THEN** no category writes any escape sequence
- **AND** each column keeps the label naming its state

#### Scenario: An unrecognised category is not guessed at

- **WHEN** a state carries a category this build does not recognise
- **THEN** it is drawn in the ordinary text colour on every surface
