## ADDED Requirements

### Requirement: Reactions in the terminal thread

The detail view SHALL show each comment's reactions under its body as each emoji with its count, and the
`&` key SHALL ask for an emoji and toggle it on the selected comment: add it when the reader has not left
it, remove it when they have.

#### Scenario: Reacting from the detail view

- **GIVEN** a task open in the detail view with a comment selected
- **WHEN** the reader presses `&`, types 👍 and applies
- **THEN** the reader's 👍 is added to that comment

#### Scenario: Taking a reaction back

- **GIVEN** the selected comment carries the reader's 👍
- **WHEN** the reader presses `&`, types 👍 and applies
- **THEN** the reader's 👍 is removed
