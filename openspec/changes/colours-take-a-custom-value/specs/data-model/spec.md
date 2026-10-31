## MODIFIED Requirements

### Requirement: Project colour and icon

A project SHALL carry an optional colour and an optional icon so that rows belonging to different
projects can be told apart at a glance. The colour SHALL be either a token drawn from a fixed palette
defined by the system, or a custom colour written as `#` followed by six hexadecimal digits. The
system SHALL store a palette token or a custom colour in lowercase, and SHALL reject any other value,
including short hexadecimal forms, colour names outside the palette and functional notations, with a
validation error raised by the service layer on every access path. The icon SHALL be a short
printable token, at most two characters, which admits a single emoji or a brief monogram and requires
no icon font or asset pipeline. Both SHALL default to empty, SHALL be settable at creation and
afterwards, and SHALL be clearable by setting them to empty.

#### Scenario: Colour and icon round trip

- **WHEN** a project is created with a palette colour and an icon
- **THEN** reading the project back returns the same colour and icon

#### Scenario: A custom colour round trips in lowercase

- **WHEN** a project is created or updated with the colour `#1A7F5A`
- **THEN** reading the project back returns the colour `#1a7f5a`

#### Scenario: Colour outside the palette is refused

- **WHEN** a project is created or updated with a colour that is neither in the palette nor `#` followed by six hexadecimal digits
- **THEN** the write is rejected with a validation error and nothing is persisted

#### Scenario: Icon longer than the limit is refused

- **WHEN** a project is created or updated with an icon longer than two characters
- **THEN** the write is rejected with a validation error

#### Scenario: Both are clearable

- **WHEN** a project that has a colour and an icon is updated with an empty colour and an empty icon
- **THEN** the project keeps its other fields and carries neither a colour nor an icon

#### Scenario: Existing projects are unaffected by the migration

- **WHEN** the migration adding the colour and icon columns is applied to a database holding projects
- **THEN** every existing project carries an empty colour and an empty icon and every other field is unchanged

#### Scenario: The project listing shows both

- **WHEN** projects are listed in the human-readable table format
- **THEN** the table carries a colour column and an icon column, the colour is drawn as a sample in
  that colour when colour is enabled and as the bare colour name or hexadecimal value when it is
  not, and a project with neither renders both columns empty

#### Scenario: Appearance survives a snapshot round trip

- **WHEN** a project carrying a colour and an icon is exported and the snapshot is imported elsewhere
- **THEN** the imported project carries the same colour and the same icon

#### Scenario: Appearance travels with a project template

- **WHEN** a project carrying a colour and an icon is exported as a bundle component and the bundle is applied
- **THEN** the resulting project carries the same colour and the same icon, and a bundle naming a
  colour that is neither in the palette nor a custom colour, or an over-long icon, is rejected before
  anything is written
