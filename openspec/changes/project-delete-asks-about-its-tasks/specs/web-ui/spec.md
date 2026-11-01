## ADDED Requirements

### Requirement: Deleting a project in the browser is confirmed on a screen that asks about its tasks

The project's delete control SHALL lead to a confirmation screen rather than deleting. For a project holding no
tasks the screen SHALL say so and offer one button that deletes it. For a project holding tasks the screen SHALL
state how many, offer moving them and deleting them as two radio choices, offer a select of the other live projects
as the move target, show the status mapping the selected target implies before anything is confirmed, and state
plainly that deletion is permanent and cannot be undone. The delete path SHALL require the project key to be typed;
a submission whose typed key does not match SHALL be refused on the same screen and SHALL delete nothing.

Choosing another target SHALL redisplay the mapping for it without the CSRF token entering a URL, and SHALL work
without scripting. The screen SHALL carry a help tip explaining the two choices. When no other live project exists
the move choice SHALL be shown as unavailable with the reason.

#### Scenario: The delete button asks first

- **WHEN** the reader presses Delete project in the danger zone
- **THEN** a confirmation screen opens and the project still exists

#### Scenario: The move choice shows the mapping

- **GIVEN** a project with tasks and another project with a different workflow
- **WHEN** the confirmation screen is opened with that project selected
- **THEN** each source status is listed with the status it maps to and its task count

#### Scenario: A mistyped key deletes nothing

- **WHEN** the delete choice is submitted with a key that is not the project's
- **THEN** the screen is shown again with the refusal beside the key field and the project remains

#### Scenario: Moving from the browser

- **WHEN** the move choice is submitted with a target
- **THEN** the project is deleted, its tasks are in the target, and the reader lands on the target's board

### Requirement: The deletion confirmation is drawn in the shared form layout

The project deletion confirmation SHALL be drawn in the layout the create forms use: one card, controls of the
shared control height, refusals under the field they are about, and Cancel beside the action at the bottom
right. Each answer about the tasks SHALL be a card holding the controls that answer needs, and the mapping table
SHALL fit the card at any width without scrolling sideways. Escape SHALL follow Cancel back to the project's
board. On a phone the actions SHALL stay at the bottom edge of the screen while the choices scroll.

#### Scenario: Escape leaves the confirmation

- **WHEN** the reader presses Escape in the confirmation
- **THEN** the browser returns to the project's board and nothing is deleted

#### Scenario: Each answer holds its own controls

- **WHEN** the confirmation is shown for a project with tasks and another live project
- **THEN** the move card holds the target select and the mapping, and the delete card holds the key field
