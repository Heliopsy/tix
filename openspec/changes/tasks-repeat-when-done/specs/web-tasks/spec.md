## ADDED Requirements

### Requirement: The browser sets and shows a repeat rule

The task screen's edit form and the add-task form SHALL offer a Repeat field taking the rule syntax, with a
hint naming an example. Saving the edit form with the field emptied SHALL remove the rule; a form that does
not carry the field SHALL leave the rule unchanged. A rule the service refuses SHALL be reported on the
form rather than dropped. The task screen SHALL state the rule and, once the task has spawned, link its
successor.

A listing row and a board card for a task carrying a rule SHALL show a small repeat marker whose
accessible name states the rule. Completing a recurring task from the browser SHALL create the next
occurrence exactly as on every other surface.

#### Scenario: Setting a rule from the task screen

- **WHEN** a reader enters `every week` in the Repeat field and saves
- **THEN** the task carries `every week` and the screen states it

#### Scenario: Marker on a row

- **WHEN** the listing shows a task carrying `every month`
- **THEN** its row shows the repeat marker naming `every month`, and a row of a task with no rule shows none

#### Scenario: Refused rule

- **WHEN** a reader saves the Repeat field as `every fortnight`
- **THEN** the form reports the refusal and the task is unchanged
