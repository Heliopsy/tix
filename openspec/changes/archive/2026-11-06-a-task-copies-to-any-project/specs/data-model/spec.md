## ADDED Requirements

### Requirement: Copied-from link

A task SHALL carry an optional `copied_from_id` naming the task it was copied from. The column
SHALL NOT be a foreign key: a link to a purged task SHALL resolve to nothing rather than block
the purge or clear the link. Snapshots SHALL carry the field.

#### Scenario: Link survives the source

- **WHEN** a linked copy's source is hard deleted
- **THEN** the copy still exists and its `copied_from_id` is unchanged

#### Scenario: Unlinked by default

- **WHEN** a task is created rather than copied
- **THEN** its `copied_from_id` is empty
