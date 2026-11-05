## MODIFIED Requirements

### Requirement: A setting is a row with its control in one column

On a wide viewport each setting SHALL be drawn as a row with its name and a short description on the
left and its control on the right, and the controls of every row SHALL share one column width so that
they align. At a viewport 900px wide or narrower each row SHALL stack its control under its
description. A setting whose explanation is longer than its description SHALL offer it behind an
information disclosure beside its name. An on/off preference SHALL be drawn as a switch that exposes
its state to assistive technology, names its state in words such as On and Off or Shown and Hidden,
and remains a plain form submission. A preference choosing between two or more named positions SHALL
be drawn as a segmented control naming every position, the current one marked with `aria-current`,
each position a submit button that works without scripting, and SHALL NOT be drawn as a switch.

#### Scenario: An on/off preference is a switch

- **WHEN** the settings screen is drawn
- **THEN** each on/off preference's submit button has the switch role and reports whether it is on

#### Scenario: Two named positions are a segmented control

- **WHEN** the settings screen is drawn
- **THEN** the Task opens in row offers Page and Dialog as two submit buttons and no switch
- **AND** the Clicking a title row offers Open the task and Edit the title as two submit buttons and
  no switch

#### Scenario: Rows stack on a narrow screen

- **GIVEN** a viewport 900px wide
- **WHEN** the settings screen is drawn
- **THEN** each row's control sits under its description

### Requirement: A browser chooses whether a task opens as a page or in a dialog

The web interface SHALL offer a per-browser preference, Task opens in, stored in the `tix_task_open`
cookie, whose positions are Page and Dialog. Absent, or any value other than `dialog`, SHALL mean
Page, which SHALL behave exactly as the interface did before the preference existed. The control
SHALL be a segmented control in the Listings section of Settings that posts without scripting, names
both positions and marks the one in force.

#### Scenario: An untouched browser opens tasks as pages

- **GIVEN** a browser that has never set Task opens in
- **WHEN** the reader opens the task list
- **THEN** the page carries no task dialog
- **AND** the Settings control for Task opens in marks Page as current

#### Scenario: Choosing Dialog

- **WHEN** the reader presses Dialog in the Task opens in control on Settings
- **THEN** the browser stores `tix_task_open=dialog`
- **AND** the reader is returned to the Listings section with a toast naming the setting
- **AND** the task list carries the task dialog

### Requirement: A browser chooses whether clicking a title edits it

The web interface SHALL offer a per-browser preference, Clicking a title, stored in the
`tix_title_click` cookie, whose positions are Open the task and Edit the title. Absent, or any value
other than `edit`, SHALL mean Open the task. The control SHALL be a segmented control in the Listings
section of Settings that posts without scripting, names both positions and marks the one in force. A
post naming a position SHALL store it; a post naming none SHALL move to the other position.

#### Scenario: Choosing Edits the title

- **WHEN** the reader presses Edit the title in the Clicking a title control on Settings
- **THEN** the browser stores `tix_title_click=edit`
- **AND** every task title in the list, on board cards and on lane cards is marked as editable and
  has an open link beside it

## ADDED Requirements

### Requirement: A post to Task opens in stores the position it names

A post naming a position SHALL store it; a post naming none SHALL move to the other position.

#### Scenario: Pressing the current position keeps it

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader presses Dialog again
- **THEN** the browser still stores `tix_task_open=dialog`

### Requirement: The database target is for administrators

The Settings screen SHALL draw its Database section, and the section navigation's link to it, only for
a reader holding `tenant:admin`. The command palette SHALL offer the Database entry to the same
readers only. The target SHALL be set at body size and SHALL break anywhere rather than widen the page.

#### Scenario: A member does not see the target

- **GIVEN** a server started with a target description
- **WHEN** a reader without `tenant:admin` opens Settings
- **THEN** the page carries no Database section, no link to it and no part of the target

#### Scenario: An administrator does

- **GIVEN** a server started with a target description
- **WHEN** a tenant administrator opens Settings
- **THEN** the Database section shows the target and the section navigation links to it

### Requirement: The calendar feed says where its address went

When the reader has a calendar feed, the Calendar feed section's first row SHALL state that the feed is
Active once, SHALL say when it was created, and SHALL say that its address is shown only once, when the
feed is created or rotated, so a new address comes from Rotate.

#### Scenario: An active feed explains its address

- **GIVEN** a reader with a calendar feed
- **WHEN** they open Settings
- **THEN** the Calendar feed section's first row says Active once, names the creation date and says
  the address is shown once and Rotate issues a new one

### Requirement: Settings sections share one shape

Every Settings section SHALL be a heading followed by its card of rows. A section with a description
SHALL draw it as the card's first row. No section heading SHALL carry an information disclosure; each
row SHALL carry its own. Existing notification channels SHALL be rows in a card, and the form adding a
channel SHALL be folded behind an Add disclosure that opens by itself when the form was refused.

#### Scenario: Descriptions sit in the card

- **WHEN** the settings screen is drawn
- **THEN** the Inbox and Notifications descriptions are inside their sections' cards
- **AND** no section heading has an information disclosure

#### Scenario: Add a channel is folded

- **WHEN** a reader with one channel opens Settings
- **THEN** the channel is a row in a card and Add a channel is a closed disclosure

### Requirement: An information note pushes the page down

An information disclosure beside a setting's name, a field's label or a heading SHALL, when opened,
draw its note in the flow under that line, moving what follows down, rather than over the content
below it.

#### Scenario: Opening a note on a phone

- **GIVEN** a viewport 390px wide
- **WHEN** the reader opens the (i) beside Colour scheme on Settings
- **THEN** the note is drawn under the name and the Colour rows by row starts below the note

### Requirement: Placeholders read as hints

A placeholder SHALL be drawn in the muted colour, in italics and in the body font, including in a
monospace field, so it cannot be mistaken for a value. A placeholder giving an example value SHALL say
it is an example.

#### Scenario: The feed filter's placeholder is an example

- **WHEN** a reader opens the Calendar feed section
- **THEN** the filter field's placeholder begins "For example"

### Requirement: Touch-only devices are not offered hover and keyboard settings

On a device whose primary input cannot hover and is a coarse pointer, the Settings screen SHALL NOT draw the
Hover effects row or the Keyboard section and its navigation link, and the account menu's Keyboard
shortcuts entry, the command palette's key hints and the create forms' key line SHALL NOT be drawn. A
device whose primary input is a fine, hovering pointer SHALL keep them.

#### Scenario: A phone drops them

- **GIVEN** a touch-only phone
- **WHEN** the reader opens Settings
- **THEN** neither Hover effects nor the Keyboard section is drawn

#### Scenario: A touch laptop keeps them

- **GIVEN** a laptop with a touch screen, whose primary input is its trackpad
- **WHEN** the reader opens Settings
- **THEN** Hover effects and the Keyboard section are drawn
