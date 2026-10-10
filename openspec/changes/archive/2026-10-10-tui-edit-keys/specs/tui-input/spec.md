## ADDED Requirements

### Requirement: One key edits the selected thing

The interface SHALL offer exactly one edit binding, dispatched by the view it is pressed in: the
whole-task form on a view where a task is selected, the project form on the project screen, and the
tenant form on the tenant screen. It SHALL NOT offer a second binding that edits a task's title alone,
and no free-text input gathering only a title SHALL remain registered.

Each screen that borrows the binding SHALL describe it in its own words, because the description that
fits a task does not fit a screen holding none.

#### Scenario: The edit key opens the whole-task form

- **WHEN** the edit key is pressed with a task selected, on the board or in the detail view
- **THEN** the whole-task form opens over that task, showing title, body, priority and assignee
- **AND** no single-field title input is opened

#### Scenario: The edit key still edits a project

- **WHEN** the edit key is pressed on the project screen
- **THEN** the project form opens on the open project's attributes

#### Scenario: The edit key still edits a tenant

- **WHEN** the edit key is pressed on the tenant screen
- **THEN** the tenant form opens on the tenant in force

#### Scenario: A reader who may not edit is not offered it

- **WHEN** the reader's authority does not reach the update the binding calls
- **THEN** the footer and the help overlay leave the binding out, and pressing it does nothing

### Requirement: A key cycles the selected task's priority

The interface SHALL offer a binding that moves the selected task one place down the priority scale and
applies it immediately, without opening a picker or any other input. From the lowest priority it SHALL
wrap to the highest, so every priority is reachable from that binding alone.

The binding SHALL be offered wherever a task is selected, gated on the same update operation the rest of
the task actions are gated on, and SHALL NOT be offered on a deleted task.

The binding that opens the priority picker SHALL continue to set any priority in one trip.

#### Scenario: One press moves one place

- **WHEN** the cycle key is pressed on a task at a priority above the lowest
- **THEN** the task is updated to the next priority down, with nothing left open to answer

#### Scenario: The lowest priority wraps

- **WHEN** the cycle key is pressed on a task at the lowest priority
- **THEN** the task is updated to the highest priority

#### Scenario: The cycle reaches the open task too

- **WHEN** the cycle key is pressed in the detail view
- **THEN** the open task's priority moves one place, the same as on the board

#### Scenario: The picker is still one trip

- **WHEN** the picker key is pressed and a priority is chosen
- **THEN** that priority is applied, whatever the task held before

### Requirement: No key means two things in one view

A keybinding scheme SHALL NOT bind one key to two actions reachable from the same view, counting the
cross-view bindings as reachable from every view. A scheme or override that would SHALL be refused
rather than applied.

Because the cross-view bindings are matched before a view's own, a key wanted for a view-level action
SHALL be released by the cross-view action that held it rather than shadowed by it.

#### Scenario: Every shipped scheme validates

- **WHEN** each shipped scheme's key map is validated across every view
- **THEN** no collision is reported

#### Scenario: A key taken by a view action is not also a cross-view key

- **WHEN** a view-level action takes a key a cross-view action held
- **THEN** the cross-view action is bound to a different key, and both keys work
