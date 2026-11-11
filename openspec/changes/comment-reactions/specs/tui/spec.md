## ADDED Requirements

### Requirement: The terminal's thread shows and toggles reactions

The task view SHALL draw each comment's reactions under its body, the reader's own marked. The React key SHALL
open a numbered picker of every reaction for the selected comment, saying which the reader gave, and picking
one SHALL give it or, when the reader gave it, take it back. On the board the key SHALL say to open the task.

#### Scenario: Taking back one's own

- **GIVEN** the reader gave thumbs_up to the selected comment
- **WHEN** they press the React key and pick thumbs_up
- **THEN** the reaction is taken back rather than given again
