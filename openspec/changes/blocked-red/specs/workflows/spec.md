## ADDED Requirements

### Requirement: Blocked is drawn in the danger colour

The browser SHALL draw the `blocked` category in the interface's danger colour in every colour scheme, the
red the terminal draws it in, and SHALL NOT draw it in the warning colour. This SHALL hold for the status pill,
the category used by lanes and statistics, the status tint on rows and cards, and the pill marking a task
blocked by an unfinished dependency.

#### Scenario: A blocked task in the browser

- **WHEN** a page draws a task in the `blocked` state, or a task blocked by a dependency
- **THEN** its status pill is drawn in the danger colour
- **AND** not in the warning colour used for notices and high priority

#### Scenario: The statistics breakdown

- **WHEN** the statistics screen draws the `blocked` share of the work
- **THEN** its bar segment and key swatch are drawn in the danger colour
