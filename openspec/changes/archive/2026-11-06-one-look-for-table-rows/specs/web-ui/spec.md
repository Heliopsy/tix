## ADDED Requirements

### Requirement: Closing a dialog leaves no mark behind

A dialog's opener SHALL name an id no other element on the screen carries, so the button opens the
dialog rather than following its link. Closing the dialog SHALL take both its `dialog` parameter and a
fragment naming it out of the address. No row SHALL stay outlined or tinted after the dialog closes.

#### Scenario: Manage on the user listing

- **WHEN** the reader clicks Manage on a user's row and then closes the dialog
- **THEN** the address carries neither the dialog nor a fragment, and the row looks as it did before

### Requirement: A row a link lands on flashes once

The row a fragment, a deep link or a toast names SHALL be lit with a brief tint that fades by itself,
with or without scripting. It SHALL NOT keep an outline or a tint after the flash ends.

#### Scenario: A link from the activity feed

- **WHEN** the reader follows a link to `/admin/users#user-ID`
- **THEN** that row flashes and then returns to its resting look

### Requirement: Every table row takes the same states

Every table and every list drawn as one SHALL carry the class `rows`. Its rows SHALL take one tint
under the pointer and while a control in them has keyboard focus, one colour when picked or focused by
the keyboard cursor, and no other hover border, bar, shadow, movement or cursor, in every theme.

#### Scenario: Hovering rows on different screens

- **WHEN** the reader points at a row of the user listing, the members table, the task list and the project list
- **THEN** each takes the same background tint and nothing else about it changes
