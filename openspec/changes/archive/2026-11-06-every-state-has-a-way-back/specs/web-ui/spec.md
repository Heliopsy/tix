## ADDED Requirements

### Requirement: Every screen state has a way back

Every state a screen can be drawn in, including a result, a preview and a refusal, SHALL show a way
back to where the reader came from, and that way back SHALL lead to an address that draws a screen.

#### Scenario: A sync run's result

- **WHEN** a reader runs a sync source from the sync screen
- **THEN** the result is drawn at the top of the sync screen with a link back to the sources

#### Scenario: A deletion preview

- **WHEN** a reader previews where a deleted project's tasks would move
- **THEN** the confirmation still offers Cancel and the back link to the board

### Requirement: A failed action with no screen leads back to the screen it was posted from

When a mutation whose address draws no screen fails on the error screen, the way back SHALL lead to
the same-origin screen that posted it, or to the task list when there is none, and never to the
action's own address.

#### Scenario: A sync run refused for its mapping file

- **GIVEN** a sync source whose mapping file does not exist
- **WHEN** the reader runs it
- **THEN** the sync screen shows why the run was refused with a link back to the sources
- **AND** no way back leads to `/sync/run`

### Requirement: A result posted to an action is shown at its screen's address

A result or a refused dialog drawn in answer to a POST to an action SHALL ask the browser to show the
address of the screen it is drawn on, so reloading it or going back to it does not post again.

#### Scenario: Reloading a sync result

- **WHEN** a reader with scripting enabled reloads the page after a sync run
- **THEN** the browser loads `/sync` and the run is not repeated

### Requirement: The deletion preview describes the target the picker shows

The project deletion screen SHALL select, in its target picker, the project its status mapping
describes, whatever case the target was named in.

#### Scenario: A target named in capitals

- **WHEN** the deletion screen is opened with `move_to` naming a project key in capitals
- **THEN** the picker selects that project and the mapping describes it
