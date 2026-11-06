## ADDED Requirements

### Requirement: Group preferences are per actor and per view

The store SHALL keep group preferences keyed by tenant, actor, view and group, holding hidden,
collapsed, starred and a position. A preference that says nothing SHALL not be stored. A project's
view and a project group SHALL be stored by the project's identifier, so a project whose key changes
keeps its arrangement. Removing an actor or a tenant SHALL remove its preferences. Preferences SHALL
not travel in snapshots.

#### Scenario: A preference reset to plain leaves no row

- **GIVEN** a reader who hid a column
- **WHEN** they show it again
- **THEN** no preference row remains for that column

#### Scenario: Readers are apart

- **WHEN** two actors of one tenant each star a different lane
- **THEN** each actor's listing of the lanes view holds only their own star
