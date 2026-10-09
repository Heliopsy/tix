## ADDED Requirements

### Requirement: The settings screen owns every per-browser display preference

The settings screen SHALL carry a control for every per-browser display preference this interface
keeps, with no preference editable only from elsewhere.

Every such control SHALL be a form the browser submits on its own, with a method, an action and a
submit button, and SHALL carry no scripted handler, so that it stores its value with scripting
turned off.

Every such control SHALL be accompanied by a description of what the preference does, associated
with the control so that assistive technology announces it, and written so that a reader who does not
already know the preference's name can tell what it changes.

The set of preferences SHALL be declared in the code, and both the settings screen's coverage of it
and the documentation table that names it SHALL be decided against that declaration rather than
maintained by hand.

#### Scenario: Every preference has a control on the settings screen

- **GIVEN** the set of per-browser preferences this build declares
- **WHEN** a signed-in reader opens the settings screen
- **THEN** the screen carries a posting form for each one

#### Scenario: A preference added without a control fails

- **GIVEN** a per-browser preference declared in the code
- **WHEN** the settings screen carries no control that submits to its route
- **THEN** the guard over the declared set fails

#### Scenario: Every preference is documented

- **GIVEN** the set of per-browser preferences this build declares
- **WHEN** the preferences table in the web interface documentation is read
- **THEN** every declared preference has a row, and every row names a declared preference

#### Scenario: Saving a preference needs no scripting

- **GIVEN** a browser with scripting unavailable
- **WHEN** it submits any preference form on the settings screen as a plain form post
- **THEN** the value is stored
- **AND** the browser is returned to the screen the form was submitted from

### Requirement: A preference editable in two places is one control over one value

A preference whose control appears both on the settings screen and on the screen that uses it SHALL
be rendered from one template against state built by one constructor, and SHALL be resolved by one
reader and stored by one handler.

Changing such a preference on either screen SHALL be reflected in the control on the other, as that
control renders, and not merely in the stored value.

#### Scenario: A change made on settings reaches the screen that uses it

- **GIVEN** a reader who switches the task view to the board on the settings screen
- **WHEN** they open the task screen
- **THEN** the board is drawn
- **AND** the task screen's own switch shows the board as the current position

#### Scenario: A change made in context is shown on settings

- **GIVEN** a reader who puts one project away using the control on the task screen
- **WHEN** they open the settings screen
- **THEN** that project's checkbox in the settings control is unticked
- **AND** every project they kept is ticked

#### Scenario: A column change made on a listing is shown on settings

- **GIVEN** a reader who hides one optional column using the picker beside a listing
- **WHEN** they open the settings screen
- **THEN** that column's checkbox in that listing's section is unticked

#### Scenario: A column change made on settings reaches the listing

- **GIVEN** a reader who hides one optional column of a listing from the settings screen
- **WHEN** they open that listing
- **THEN** the hidden column is absent from the table's header row
- **AND** every column they kept is present

### Requirement: Column preferences are offered per listing

The settings screen SHALL offer a column picker for every listing whose columns can be chosen, and
for no listing that has none, each picker naming the listing it governs.

Each picker SHALL offer the same columns, in the same order, with the same current state, as the
picker that listing carries beside itself, and SHALL offer a reset that returns that listing to its
declared defaults.

#### Scenario: Each configurable listing has its own section

- **GIVEN** the listings this build declares columns for
- **WHEN** a reader opens the settings screen
- **THEN** each listing has exactly one picker, named after that listing

#### Scenario: A listing with no configurable columns is not offered one

- **GIVEN** a screen that declares no optional columns
- **WHEN** a reader opens the settings screen
- **THEN** no picker names it

### Requirement: The project visibility control names its projects and accounts for keys that name none

The project visibility control SHALL name each project as well as identify it by key, so that it can
be used on a screen that carries no project listing.

Where the stored preference holds a key that no project on the control's own listing answers to, the
control SHALL name that key and SHALL say that no project answers to it, and SHALL not offer a
checkbox for it. Submitting the control SHALL discard such keys.

A stored key that names no project SHALL exclude nothing from the task listing, and SHALL not prevent
any screen from rendering.

#### Scenario: A project is named, not only keyed

- **GIVEN** a tenant with a project whose key and name differ
- **WHEN** a reader opens the visibility control on either screen
- **THEN** the project's name and its key are both shown

#### Scenario: A key naming a deleted project does not break the screen

- **GIVEN** a browser whose stored preference hides a key no project answers to
- **WHEN** the reader opens the task screen or the settings screen
- **THEN** the screen renders
- **AND** the key is named as one no project answers to
- **AND** no checkbox is offered for it

#### Scenario: A key naming a deleted project is forgotten on the next submission

- **GIVEN** a browser whose stored preference hides a key no project answers to
- **WHEN** the reader applies the visibility control
- **THEN** the key is no longer held
- **AND** the choice they applied is

#### Scenario: A key naming a deleted project excludes nothing

- **GIVEN** a browser whose stored preference hides a key no project answers to
- **WHEN** the task listing is answered
- **THEN** every task of every project the reader kept is on it

### Requirement: The task view preference has two states

The task view preference SHALL have exactly two positions on every control that offers it: the list
and the board. No control SHALL offer a third position meaning that no choice has been made.

The list SHALL be what an absent, empty or unrecognised stored value means, and choosing the list
SHALL store nothing.

#### Scenario: The switch on settings has the same two positions

- **GIVEN** a reader on the settings screen
- **WHEN** they read the view switch
- **THEN** it offers the list and the board and nothing else
- **AND** exactly one of them is marked as current

#### Scenario: An untouched browser reads as the list on both screens

- **GIVEN** a browser that has never used either switch
- **WHEN** the reader opens the settings screen and the task screen
- **THEN** both show the list as the current position
