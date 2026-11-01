## MODIFIED Requirements

### Requirement: Project colour and icon distinguish rows

The task list SHALL mark each row with its project's colour as an accent along the row's leading
edge, and SHALL show the project's icon beside it when one is set, drawing a text icon as text, a
built-in icon from the interface's own symbol sprite and an uploaded image through an image element.
The accent SHALL be visually distinct from the status badges, so that a project stripe is not read
as a status. A row whose project carries neither SHALL render as it did before, without an accent.
The project screens SHALL allow a project's colour and icon to be set from the palette or to a
custom colour, and cleared again.

A custom colour SHALL be drawn wherever a palette colour is drawn: the row stripe, the board card,
the lane, the project list and the project key. In each colour scheme it SHALL be drawn in a shade
that has a contrast ratio of at least 4.5:1 against that scheme's page and surface backgrounds, and a
colour that already meets that ratio SHALL be drawn exactly as chosen. Only a colour the server
parsed and formatted itself SHALL reach a `style` attribute.

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

#### Scenario: A custom colour is legible in every scheme

- **WHEN** a project carries a custom colour and is rendered under the light, low contrast and dark schemes
- **THEN** each scheme draws a shade of it with a contrast ratio of at least 4.5:1 against that scheme's page and surface

#### Scenario: A custom colour that already reads is not changed

- **WHEN** a project carries a custom colour that meets 4.5:1 against the light scheme's page and surface
- **THEN** the light scheme draws exactly that colour

#### Scenario: Clearing from the browser

- **WHEN** a project's colour is set to none and its icon emptied on the project screen
- **THEN** the project is saved with neither and its rows lose the accent

## ADDED Requirements

### Requirement: Every colour control offers the predefined colours and a custom one

Every control in the web interface that chooses a colour SHALL be the same shared control. It SHALL
offer the predefined choices as swatches, SHALL offer a custom colour through a native colour input
and a text field accepting `#` followed by six hexadecimal digits, SHALL show a live preview of the
custom colour, and SHALL carry an information tip explaining the two. Where a colour is optional it
SHALL offer none as a choice. The control SHALL submit a custom colour without scripting through the
text field. The new project form, the project settings form and the tenant theme form SHALL use it.

#### Scenario: The project form offers both

- **WHEN** a reader opens the new project form
- **THEN** it offers every palette colour, none, and a custom colour with a colour input, a hex field and a preview

#### Scenario: A custom colour is saved from the browser

- **WHEN** a reader chooses custom and enters `#1a7f5a` on the project settings form and saves
- **THEN** the project carries `#1a7f5a` and its rows are drawn in it

#### Scenario: The current custom colour is shown as chosen

- **WHEN** the project settings form is opened for a project with a custom colour
- **THEN** the custom choice is selected and the hex field and colour input hold that colour

#### Scenario: The tenant theme form offers a custom accent

- **WHEN** an administrator opens the tenant form
- **THEN** the theme is chosen with the same control, offering every resolvable theme and a custom accent
