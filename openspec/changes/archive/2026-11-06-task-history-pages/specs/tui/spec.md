## ADDED Requirements

### Requirement: The terminal history reads the newest entries first

The terminal interface's history view SHALL read the newest entries of the subject's log first, so a log
longer than the view's page draws its most recent changes and points at `tix audit ls` for the older ones.

#### Scenario: A log longer than one page

- **GIVEN** a task whose log holds more entries than the history view reads
- **WHEN** the reader opens its history
- **THEN** the view is read in descending sequence and draws the newest entries
