## ADDED Requirements

### Requirement: Arranging a view emits its own event

Every change to a reader's group preferences SHALL write an audit entry and emit a `view.arranged`
event in the same transaction, carrying the view and the groups changed, and SHALL write neither
when it changed nothing. `view.arranged` SHALL be nameable in a subscription filter.

#### Scenario: Hiding a column emits view.arranged

- **WHEN** an actor hides the `done` column of the board
- **THEN** a `view.arranged` event naming the view `board` and the group `done` is emitted

#### Scenario: A change that changes nothing is silent

- **WHEN** an actor hides a column that is already hidden
- **THEN** no event and no audit entry are written
