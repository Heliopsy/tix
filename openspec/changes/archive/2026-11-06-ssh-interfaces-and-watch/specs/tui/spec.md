## ADDED Requirements

### Requirement: The start menu lists every interface

The terminal interface SHALL offer a start menu with one row per registered interface: Boards,
which opens the project list, and Watch, which opens the activity view with a project filter
chosen on the row. `~` SHALL open the start menu from any view and the command palette SHALL offer
it. A hosted SSH session with no command SHALL start on it.

#### Scenario: Watch one project from the menu

- **GIVEN** the start menu is open with projects OPS and WEB
- **WHEN** the reader moves to Watch, picks OPS with the right arrow and presses enter
- **THEN** the activity view opens showing only events from OPS

#### Scenario: Back to the menu

- **WHEN** the reader presses `~` on a board
- **THEN** the start menu opens

### Requirement: The activity view can pause

On the activity view `space` SHALL pause the tail, holding new events off screen and saying how
many are held, and pressing it again SHALL resume and draw them.

#### Scenario: Pause and resume

- **GIVEN** the activity view is paused
- **WHEN** two events arrive and the reader presses `space`
- **THEN** both events are drawn and the view follows the newest
