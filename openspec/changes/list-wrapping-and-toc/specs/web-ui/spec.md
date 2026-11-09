## ADDED Requirements

### Requirement: List and table text breaks at word boundaries

A name, key, description or count in a list row or table cell SHALL wrap only between words. A word SHALL
be broken inside only when it alone is wider than its cell, and a break SHALL never leave a single
character on a line. A cell's content SHALL never spill out of the cell.

#### Scenario: A one-word project name on a crowded row

- **GIVEN** a project named Infrastructureplatform and a viewport 1280px wide
- **WHEN** the projects screen is drawn
- **THEN** the name is on one line

#### Scenario: A short word in a squeezed table column

- **GIVEN** the token list in Bulgarian at a viewport 820px wide
- **WHEN** a token that never expires is listed
- **THEN** the word Никога is on one line

### Requirement: A project row gives its name room before its tools

A project row SHALL keep room for its name and description, and when its Pin, Tasks, Fields and Manage
links do not fit beside them SHALL draw the links on a line of their own. A description clamped to two
lines SHALL carry its full text as a tooltip.

#### Scenario: Links move rather than squeeze the name

- **GIVEN** the projects screen in Bulgarian at a viewport 1280px wide
- **WHEN** it is drawn
- **THEN** every project name keeps its words whole and the row's links sit under it

### Requirement: The settings navigation marks the section in view

With scripting, the settings navigation SHALL mark the link for the section in view with
`aria-current="true"` and no other link, updating as the reader scrolls, and the last section once the
page is scrolled to its end. Without scripting the links SHALL remain plain anchors.

#### Scenario: Scrolling moves the mark

- **WHEN** a reader scrolls the settings screen until Notifications is the section at the top
- **THEN** the Notifications link carries aria-current="true" and no other link does

#### Scenario: The end of the page

- **WHEN** the reader scrolls to the end of the settings screen
- **THEN** the last section's link carries aria-current="true"

### Requirement: A settings navigation link scrolls to its section and updates the address

Activating a navigation link by pointer or keyboard SHALL scroll its section into view, smoothly unless
the reader's system asks for reduced motion, set the address fragment to the section, mark the link and
move focus to the section.

#### Scenario: Reduced motion

- **GIVEN** a browser that prefers reduced motion
- **WHEN** the reader activates the Inbox link
- **THEN** the page jumps to the Inbox section without animating and the address ends in #inbox
