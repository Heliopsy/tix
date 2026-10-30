## ADDED Requirements

### Requirement: The settings screen is navigable by section

The settings screen SHALL group its controls into named sections and SHALL offer a navigation listing
every section as a link to that section on the same page. The navigation SHALL work without
scripting. Where the screen has room beside the sections the navigation SHALL be a column that stays
in view while the sections scroll; where it does not, it SHALL be drawn above the sections and wrap
rather than scroll sideways.

#### Scenario: Every section is linked

- **WHEN** a signed-in reader opens the settings screen
- **THEN** the section navigation carries one link per section, each pointing at that section's id

#### Scenario: A narrow screen does not scroll sideways

- **GIVEN** a viewport 390px wide
- **WHEN** the settings screen is drawn
- **THEN** the page is no wider than the viewport

### Requirement: A setting is a row with its control in one column

On a wide viewport each setting SHALL be drawn as a row with its name and a short description on the
left and its control on the right, and the controls of every row SHALL share one column width so that
they align. At a viewport 900px wide or narrower each row SHALL stack its control under its
description. A setting whose explanation is longer than its description SHALL offer it behind an
information disclosure beside its name. An on/off preference SHALL be drawn as a switch that exposes
its state to assistive technology and remains a plain form submission.

#### Scenario: An on/off preference is a switch

- **WHEN** the settings screen is drawn
- **THEN** each on/off preference's submit button has the switch role and reports whether it is on

#### Scenario: Rows stack on a narrow screen

- **GIVEN** a viewport 900px wide
- **WHEN** the settings screen is drawn
- **THEN** each row's control sits under its description

### Requirement: Saving a setting says so

Submitting a preference from the settings screen SHALL return the reader to the section the
preference belongs to with a message naming what was saved.

#### Scenario: Saving returns to the section

- **WHEN** the reader turns Drag to move off on the settings screen
- **THEN** the browser is returned to the settings screen at the Appearance section
- **AND** a message says the preference was saved

### Requirement: Hover effects are a per-browser preference

Rows, cards, lane cards, navigation items, buttons and links SHALL respond to a pointer hovering them
on devices that can hover. Whether they do SHALL be a per-browser preference, on by default, kept and
controlled like every other display preference. When it is off the root element SHALL carry an
attribute that disables every hover effect the preference governs. A transition used by a hover
effect SHALL NOT run when the reader's system asks for reduced motion.

#### Scenario: On by default

- **GIVEN** a browser that has never chosen
- **WHEN** any signed-in screen is drawn
- **THEN** the root element does not carry the off attribute

#### Scenario: Turned off

- **WHEN** the reader turns Hover effects off on the settings screen
- **THEN** every later screen's root element carries the off attribute

#### Scenario: Reduced motion

- **GIVEN** a system that asks for reduced motion
- **WHEN** the reader hovers a row
- **THEN** no transition runs

### Requirement: Configure screens hold together at every width

The configuration and data screens SHALL draw without the page scrolling sideways at viewport widths
of 1440px, 900px and 390px, with any disclosure inside a table row opened. A table that cannot fit
SHALL scroll inside its own frame, a disclosure's label SHALL NOT break between its letters, and an
information note SHALL stay inside the viewport.

#### Scenario: The token table with Revoke open

- **GIVEN** a viewport 900px wide and a tenant with API tokens
- **WHEN** the reader opens a token's Revoke disclosure
- **THEN** the page is no wider than the viewport
