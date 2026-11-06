## ADDED Requirements

### Requirement: A history row's raw entry opens in a dialog

The Raw control on an activity row and on a task's History row SHALL open a modal dialog titled Raw entry,
naming the entry's action, subject and time, with a close control. Escape SHALL close it and return focus to
the control. It SHALL NOT open a disclosure or popover inside the row.

#### Scenario: Raw opens a dialog

- **WHEN** a reader presses Raw on an activity row
- **THEN** a modal Raw entry dialog opens at least as wide as the window allows up to 56rem
- **AND** Escape closes it and focus is back on Raw

### Requirement: The raw entry dialog shows the recorded entry readably

The dialog SHALL show each entry's actor, action, subject, full timestamp, source and sequence, and each
recorded snapshot as indented JSON with markup shown as text, each with a Copy button and one Copy all as
JSON button. Long values SHALL wrap inside the dialog.

#### Scenario: A hostile title reads as text

- **WHEN** a task titled with a script tag and a long unbroken word is retitled and its raw entry opened
- **THEN** the before snapshot shows the tag as text, indented, with nothing scrolling sideways

### Requirement: The raw entry dialog sets changed fields side by side

When an entry has both a before and an after snapshot, the dialog SHALL list the top-level fields that were
added, removed or changed, each marked with which, with its value on each side.

#### Scenario: A retitle lists the title

- **WHEN** a task's title and priority are changed and the entry's raw view opened
- **THEN** title and priority are listed as changed with the old and new values

### Requirement: The raw entry dialog works without scripting

Without scripting the Raw control SHALL be a link to the same screen with the dialog drawn open in place,
whose close control leads back to the screen without it.

#### Scenario: The fallback address draws the dialog

- **WHEN** the activity feed is read at `?dialog=raw-SEQ` for a row on it
- **THEN** that row's Raw entry dialog is drawn open, with a Close link back to `/activity`
