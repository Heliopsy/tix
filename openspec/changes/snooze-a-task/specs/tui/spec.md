## ADDED Requirements

### Requirement: Snooze from the terminal interface

On the board, `z` SHALL open a prompt for when to snooze the selected task, taking the same values
as the CLI, and SHALL unsnooze the task instead when it is snoozed. A snoozed card shown under a
snooze filter SHALL say so.

#### Scenario: Snooze the selected card

- **WHEN** the reader presses `z` on OPS-3, types `tomorrow` and confirms
- **THEN** OPS-3 is snoozed and leaves the board
