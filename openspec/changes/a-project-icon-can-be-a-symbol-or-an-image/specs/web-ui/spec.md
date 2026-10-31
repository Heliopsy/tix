## MODIFIED Requirements

### Requirement: Project colour and icon distinguish rows

The task list SHALL mark each row with its project's colour as an accent along the row's leading
edge, and SHALL show the project's icon beside it when one is set, drawing a text icon as text, a
built-in icon from the interface's own symbol sprite and an uploaded image through an image element.
The accent SHALL be visually distinct from the status badges, so that a project stripe is not read
as a status. A row whose project carries neither SHALL render as it did before, without an accent.
The project screens SHALL allow a project's colour and icon to be set from the palette and cleared
again.

#### Scenario: Row carries the project accent

- **WHEN** the task list renders a task belonging to a project that has a colour
- **THEN** the row carries that project's colour as a leading-edge accent

#### Scenario: Row carries the project icon

- **WHEN** the task list renders a task belonging to a project that has an icon
- **THEN** the icon is shown on the row

#### Scenario: A built-in icon is drawn from the sprite

- **WHEN** the task list renders a task belonging to a project whose icon is the built-in rocket
- **THEN** the row draws the sprite's rocket symbol and no emoji

#### Scenario: An uploaded icon is drawn as an image

- **WHEN** the task list renders a task belonging to a project whose icon is an uploaded image
- **THEN** the row draws an image element whose address names the project and the image's digest

#### Scenario: A project without either renders plainly

- **WHEN** the task list renders a task belonging to a project with no colour and no icon
- **THEN** the row renders without an accent or icon and the list is otherwise unchanged

#### Scenario: Colour remains legible in every scheme

- **WHEN** a project colour is rendered under the light, dark, and low contrast schemes
- **THEN** each scheme renders a shade of that palette token that remains distinguishable against its own background

#### Scenario: Clearing from the browser

- **WHEN** a project's colour is set to none and its icon emptied on the project screen
- **THEN** the project is saved with neither and its rows lose the accent

## ADDED Requirements

### Requirement: Project icon picker

The project creation form and the project settings form SHALL offer the icon as a choice of kind,
Emoji, Icon or Upload, together with a choice of no icon. The Emoji kind SHALL take the text icon,
the Icon kind SHALL offer every built-in symbol as a grid of choices each drawn from the sprite and
labelled in the reader's language, and the Upload kind SHALL take a file. The form SHALL show a
preview of the icon the project has, SHALL carry an explanatory tip like the other settings, and
SHALL keep an uploaded image when the form is saved without a new file.

#### Scenario: Choosing a built-in icon

- **WHEN** a reader picks Icon, selects the rocket and saves the project
- **THEN** the project carries the built-in rocket icon and its board header draws it

#### Scenario: Uploading an image

- **WHEN** a reader picks Upload, chooses a PNG file and saves the project
- **THEN** the project carries that image and the settings form previews it

#### Scenario: Saving other settings keeps the image

- **WHEN** a project with an uploaded icon has its name changed on the settings form and no new file chosen
- **THEN** the project keeps its image

#### Scenario: Removing the icon

- **WHEN** a reader picks no icon and saves the project
- **THEN** the project carries no icon of any kind

### Requirement: Project icon images are served safely

The browser interface SHALL serve a project's uploaded icon at `/projects/{key}/icon` with the
image's own content type, `X-Content-Type-Options: nosniff`, a content security policy that forbids
script and sandboxes the document, and a strong entity tag derived from the image's digest, and
SHALL answer a request whose `If-None-Match` matches with `304 Not Modified`. A project without an
uploaded icon SHALL answer not found.

#### Scenario: Headers on an icon

- **WHEN** a signed-in reader requests a project's icon
- **THEN** the response carries the image bytes, its content type, `nosniff`, a `sandbox` policy and an ETag

#### Scenario: Revalidation

- **WHEN** the same request is repeated with the ETag in `If-None-Match`
- **THEN** the response is 304 with no body
