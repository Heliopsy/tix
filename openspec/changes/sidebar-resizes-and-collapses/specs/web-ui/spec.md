## ADDED Requirements

### Requirement: The page content starts the same distance from the sidebar on every screen

On a viewport wide enough to show the sidebar as a column, the horizontal distance between the
sidebar's edge and the start of the page content SHALL be the same on every signed-in screen at a
given viewport width, and SHALL come from a single rule. A screen MAY cap how wide its content grows,
but that cap SHALL NOT move where the content starts.

#### Scenario: A form screen and a listing start in the same place

- **GIVEN** a viewport 1920px wide
- **WHEN** the reader opens the task screen and then the projects screen
- **THEN** the content of both begins the same distance from the sidebar

#### Scenario: A screen's measure does not centre it

- **GIVEN** a screen whose content is capped narrower than the space beside the sidebar
- **WHEN** it is drawn
- **THEN** its content begins at the gutter beside the sidebar and the spare width is on the right

### Requirement: The sidebar can be resized by dragging its edge

On a viewport wide enough to show the sidebar as a column, the sidebar's right edge SHALL be a
control that changes the sidebar's width when dragged. The width SHALL be clamped between a minimum
and a maximum, SHALL be kept per browser, and SHALL be applied before the page paints, so a reload or
a navigation never draws the default width first. Restoring the default width SHALL remove what was
kept.

The control SHALL be reachable by keyboard as a vertical separator that exposes its current value,
minimum and maximum, and the arrow keys SHALL change the width. Without scripting the control SHALL
be absent and the sidebar SHALL keep its default width.

#### Scenario: A dragged width survives a reload

- **GIVEN** a reader who drags the sidebar's edge to 320px
- **WHEN** they reload the page
- **THEN** the sidebar is 320px wide from the first paint

#### Scenario: A drag past the maximum stops at the maximum

- **WHEN** the reader drags the edge further right than the maximum width
- **THEN** the sidebar is drawn at the maximum width

#### Scenario: The edge is moved from the keyboard

- **GIVEN** the separator has focus
- **WHEN** the reader presses the right arrow
- **THEN** the sidebar widens and the separator's value reports the new width

#### Scenario: No script, no handle

- **GIVEN** a browser with scripting off
- **WHEN** any signed-in screen is drawn
- **THEN** no resize control is offered and the sidebar is its default width

### Requirement: The sidebar collapses to a rail

On a viewport wide enough to show the sidebar as a column, the sidebar SHALL offer a button that
collapses it to a narrow rail and expands it again, and dragging its edge below a threshold SHALL
collapse it. The rail SHALL keep the product logo and every navigation entry as an icon, and every
entry in the rail SHALL keep its accessible name. The collapsed state SHALL be kept per browser and
applied before the page paints, and expanding SHALL return the sidebar to the width it had.

On a viewport too narrow for the sidebar column, the collapsed state SHALL have no effect, and the
sidebar SHALL be the bar and menu it is without it.

#### Scenario: Dragging it small collapses it

- **WHEN** the reader drags the sidebar's edge below the collapse threshold and lets go
- **THEN** the sidebar is drawn as a rail holding the logo and the navigation icons
- **AND** a button to expand it is visible

#### Scenario: The button works on its own

- **GIVEN** an expanded sidebar
- **WHEN** the reader presses the collapse button and then presses it again
- **THEN** the sidebar is a rail after the first press and back at its previous width after the second

#### Scenario: A rail entry still has its name

- **GIVEN** a collapsed sidebar
- **WHEN** assistive technology reads a navigation entry in the rail
- **THEN** the entry is announced by the same name it has when the sidebar is expanded

#### Scenario: The phone layout ignores the collapsed state

- **GIVEN** a browser that collapsed the sidebar on a desktop
- **WHEN** a signed-in screen is drawn at phone width
- **THEN** the brand bar and the menu button are drawn as they are for a browser that never collapsed it
