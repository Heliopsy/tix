## ADDED Requirements

### Requirement: The task screen draws and toggles reactions under each comment

Under each comment the task screen SHALL draw one control per reaction given, with its emoji, count, who gave
it as a tooltip and `aria-pressed` for the reader's own, and a button opening a picker of every reaction.
Each SHALL be a form that works without scripting; with htmx it SHALL swap only that comment's reactions.

#### Scenario: Giving a heart

- **WHEN** a reader picks the heart under a comment
- **THEN** that comment's reactions come back with a pressed heart counting 1, and the rest of the thread is untouched

#### Scenario: Without the scope to comment

- **WHEN** a reader without `comment:write` opens the task
- **THEN** the reactions are drawn as text and no reaction form or picker is offered

### Requirement: A reaction given elsewhere reaches an open task screen

The task screen's live subscription SHALL include `comment.reacted` and `comment.unreacted`, so a reaction
given in another tab or by another person redraws the thread.

#### Scenario: Another tab

- **GIVEN** a reader has the task open in two tabs
- **WHEN** they react to a comment in one
- **THEN** the other tab's thread shows the reaction
