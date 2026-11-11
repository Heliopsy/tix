## ADDED Requirements

### Requirement: Reaction chips under each comment

The task screen SHALL draw a comment's reactions under it as chips showing the emoji and count. A chip the
reader left SHALL be highlighted and marked pressed, and each chip SHALL name who reacted in its tooltip.
Pressing a chip SHALL add that emoji for the reader, or remove it when it is already theirs.

#### Scenario: Toggling a chip

- **GIVEN** a comment with a 👍 the reader left
- **WHEN** the reader presses the 👍 chip
- **THEN** the reader's 👍 is removed and the screen is drawn again

### Requirement: Adding a reaction from the task screen

A reader with `comment:write` SHALL have an add-reaction button after the chips that opens a picker with
👍 👎 ❤️ 🎉 😄 😕 👀 🚀 and a field for any other emoji. Every control SHALL be a button or field reachable by
keyboard, and the screen SHALL update live when anyone reacts.

#### Scenario: Picking from the quick list

- **WHEN** the reader opens the picker and presses 🎉
- **THEN** the comment carries the reader's 🎉

#### Scenario: Somebody else reacts

- **GIVEN** the task screen is open
- **WHEN** another actor reacts to a comment on it
- **THEN** the chips update without a reload
