## MODIFIED Requirements

### Requirement: Project colour and icon

A project SHALL carry an optional colour and an optional icon so that rows belonging to different
projects can be told apart at a glance. The colour SHALL be a token drawn from a fixed palette
defined by the system rather than a free-form colour value, and a colour outside the palette SHALL
be rejected with a validation error by the service layer, on every access path. The icon SHALL be
exactly one of three kinds, or none: a text icon, which is a short printable token of at most two
characters admitting a single emoji or a brief monogram; a built-in icon, named from a fixed set of
symbols the system ships; or an uploaded image. Setting one kind SHALL replace whichever kind the
project carried, and a write naming two kinds at once SHALL be rejected. Both the colour and the
icon SHALL default to empty, SHALL be settable at creation and afterwards, and SHALL be clearable by
setting them to empty.

#### Scenario: Colour and icon round trip

- **WHEN** a project is created with a palette colour and an icon
- **THEN** reading the project back returns the same colour and icon

#### Scenario: Colour outside the palette is refused

- **WHEN** a project is created or updated with a colour that is not in the palette
- **THEN** the write is rejected with a validation error and nothing is persisted

#### Scenario: Icon longer than the limit is refused

- **WHEN** a project is created or updated with a text icon longer than two characters
- **THEN** the write is rejected with a validation error

#### Scenario: Unknown built-in icon is refused

- **WHEN** a project is created or updated with a built-in icon name the system does not ship
- **THEN** the write is rejected with a validation error naming the available icons

#### Scenario: One kind replaces another

- **WHEN** a project carrying a text icon is updated with a built-in icon
- **THEN** the project carries the built-in icon and no text icon

#### Scenario: Two kinds at once are refused

- **WHEN** a project is created with both a text icon and a built-in icon
- **THEN** the write is rejected with a validation error and nothing is persisted

#### Scenario: Both are clearable

- **WHEN** a project that has a colour and an icon is updated with an empty colour and an empty icon
- **THEN** the project keeps its other fields and carries neither a colour nor an icon of any kind

#### Scenario: Existing projects are unaffected by the migration

- **WHEN** the migration adding the colour and icon columns, or the one adding built-in and uploaded icons, is applied to a database holding projects
- **THEN** every existing project keeps the colour and text icon it had, carries no built-in icon and no image, and every other field is unchanged

#### Scenario: The project listing shows both

- **WHEN** projects are listed in the human-readable table format
- **THEN** the table carries a colour column and an icon column, the colour is drawn as a sample in
  that palette colour when colour is enabled and as the bare colour name when it is not, a built-in
  icon is shown as its fallback glyph and its name, an uploaded image as a fixed mark, and a project
  with neither renders both columns empty

#### Scenario: Appearance survives a snapshot round trip

- **WHEN** a project carrying a colour and a text or built-in icon is exported and the snapshot is imported elsewhere
- **THEN** the imported project carries the same colour and the same icon

#### Scenario: Appearance travels with a project template

- **WHEN** a project carrying a colour and a text or built-in icon is exported as a bundle component and the bundle is applied
- **THEN** the resulting project carries the same colour and the same icon, and a bundle naming a
  colour outside the palette, an over-long icon or an unknown built-in icon is rejected before
  anything is written

## ADDED Requirements

### Requirement: Uploaded project icon images

An uploaded project icon SHALL be a PNG, JPEG, WebP or SVG image of at most 64 KiB, recognised from
its bytes rather than from a type the caller declares. An SVG SHALL be refused when it carries a
script element, a `foreignObject` element, an event handler attribute, a `javascript:` URL, or a
reference to anything other than a fragment of itself or an embedded `data:` image. The image bytes
SHALL be stored in the database on both engines, in a tenant-owned table subject to the same tenant
scoping and, on PostgreSQL, the same row-level isolation policy as every other tenant-owned table,
and SHALL be removed with their project. The project SHALL carry the image's content digest, and
reading the image SHALL require the same permission as reading the project. An image SHALL be
written in the same transaction as the project change, its audit entry and its event.

#### Scenario: An image round trips

- **WHEN** a project is updated with a 1 KiB PNG and its icon is read back
- **THEN** the read returns the same bytes with content type `image/png`, and the project carries the image's digest and no text or built-in icon

#### Scenario: A declared type is not trusted

- **WHEN** a file of plain text is uploaded declaring itself `image/png`
- **THEN** the write is rejected with a validation error

#### Scenario: Oversized images are refused

- **WHEN** an image larger than 64 KiB is uploaded
- **THEN** the write is rejected with a validation error and the project is unchanged

#### Scenario: Scripted SVG is refused

- **WHEN** an SVG carrying a script element or an `onload` attribute is uploaded
- **THEN** the write is rejected with a validation error

#### Scenario: Another tenant cannot read the image

- **GIVEN** a project with an uploaded icon in one tenant
- **WHEN** an actor of another tenant asks for that project's icon
- **THEN** the request fails as not found and no bytes are returned

#### Scenario: Replacing the image removes the old bytes

- **WHEN** a project with an uploaded icon is given a text icon
- **THEN** reading its icon image fails as not found

#### Scenario: Imports do not carry images

- **WHEN** a snapshot holding a project with an uploaded icon is imported into a store where that project already carries an image
- **THEN** the project keeps the image it had, and an import into a store without the project creates it with no icon
