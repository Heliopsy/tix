## MODIFIED Requirements

### Requirement: Workflow and field definition editors

The web UI SHALL provide an editor for per-project workflows, including states and allowed transitions, and an editor for typed custom field definitions.

Editing part of a workflow SHALL preserve the rest. A save from the workflow editor SHALL change only what its form edits -- the initial state, the state keys with their labels and terminality, the permitted transitions, and the state migrations -- and SHALL carry every other field of a state, a transition or the definition through from what is stored. A state or transition the form no longer names SHALL be removed.

A state key the form renames SHALL keep the fields the editor does not show when the save declares the rename in its migration lines. A rename the save does not declare SHALL be treated as removing one state and adding another, which the service refuses outright while any task still sits in the removed state.

The third field of a state line SHALL accept `terminal` or `open` and `true` or `false`, SHALL read an empty field as open, and SHALL refuse a value that names neither rather than reading it as open. The help text beside the field SHALL name the values it accepts.

#### Scenario: Workflow is edited

- **WHEN** an authorized user adds a state and a transition to a project's workflow
- **THEN** the board shows the new column and the new transition is permitted

#### Scenario: Saving without changing anything changes nothing

- **WHEN** an authorized user opens the workflow editor and saves it with no edits
- **THEN** every state keeps its reporting category and its lease-expiry revert, every transition keeps the scope and the comment it requires, and the workflow keeps its default lease

#### Scenario: A restricted transition stays restricted

- **WHEN** a transition requires a scope and the workflow is saved from the editor with no edits
- **THEN** an actor holding the plain transition scope but not the required one is still refused that move, and an actor holding the required scope may still make it

#### Scenario: A renamed state keeps what the editor does not show

- **WHEN** an authorized user changes a state's key and gives the rename as a migration line
- **THEN** the state under its new key keeps the category and the lease-expiry revert the old key carried

#### Scenario: A removed state is removed

- **WHEN** an authorized user deletes a state line and the transitions naming it
- **THEN** that state and those transitions are gone from the stored workflow

#### Scenario: Invalid workflow is refused

- **WHEN** a workflow edit would leave tasks in a state the workflow no longer defines
- **THEN** the change is refused with an explanation and the workflow is unchanged

#### Scenario: The documented terminal words are read as written

- **WHEN** a state line ends in `true`, having followed the help text beside the field
- **THEN** the state is stored as terminal

#### Scenario: An unreadable terminal field is refused

- **WHEN** a state line ends in a word that is neither a yes nor a no
- **THEN** the save is refused with an explanation and no workflow is stored

#### Scenario: Field definition is created

- **WHEN** an authorized user defines a typed custom field
- **THEN** the field appears on task detail screens for that project and values are validated against its type
