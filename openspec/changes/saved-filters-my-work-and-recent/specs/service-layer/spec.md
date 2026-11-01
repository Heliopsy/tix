## ADDED Requirements

### Requirement: Saved filter operations

The service SHALL offer `ListSavedFilters`, `GetSavedFilter`, `CreateSavedFilter`, `UpdateSavedFilter`,
`OrderSavedFilters` and `DeleteSavedFilter`. Each SHALL require `task:read`; changing or deleting a shared
filter the caller does not own SHALL require `tenant:admin` and SHALL otherwise be refused as forbidden.
Another actor's private filter SHALL be reported as not found. A filter SHALL be addressed by identifier,
then by the caller's own name, then by a shared name. Names SHALL be unique per owner without regard to
case. A saved expression SHALL be refused as invalid when the filter parser cannot read it. Each change
SHALL write its rows, an audit entry and an event in one transaction, and a change that changes nothing
SHALL write nothing.

#### Scenario: A member cannot change somebody's shared filter

- **GIVEN** a shared filter owned by alice
- **WHEN** bob, a member, renames it
- **THEN** the call is refused as forbidden and nothing is written

#### Scenario: An administrator can

- **GIVEN** a shared filter owned by alice
- **WHEN** an administrator renames it
- **THEN** the new name is stored and an audit entry records it

#### Scenario: Ordering moves only the caller's own filters

- **WHEN** a reader orders filters naming one somebody else shared
- **THEN** the call is refused as not found

### Requirement: me names the caller in a filter

A task listing SHALL read `me` (any case) in the assignee, creator and claimant terms, included or
excluded, as the calling actor, without changing the filter the caller passed.

#### Scenario: assignee:me lists the caller's tasks

- **GIVEN** a task assigned to alice and one assigned to bob
- **WHEN** alice lists tasks with `assignee:me`
- **THEN** only her task is listed
