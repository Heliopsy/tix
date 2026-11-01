## MODIFIED Requirements

### Requirement: Every display preference has exactly one always-reachable control

The browser interface SHALL gather the preferences that apply to every screen into a settings screen
reachable from the sidebar on every signed-in screen. Every control within it SHALL be a plain form
that works with scripting disabled, and the screen SHALL render under every colour scheme on offer,
including the low contrast one. A preference scoped to one listing, such as which columns it shows,
SHALL be offered beside that listing as well as on the settings screen, and both controls SHALL
write the same cookie, so the two can never disagree.

The colour scheme SHALL instead be set by a single control in the sidebar, present on every
signed-in screen, and SHALL NOT also be settable from the settings screen, so that one cookie has
one control. That control SHALL be a plain form that works with scripting disabled, SHALL move to
the next scheme on offer each time it is submitted, and SHALL return the reader to the screen they
submitted it from. It SHALL be reachable by keyboard, and its accessible name SHALL state both the
scheme in force and the scheme submitting it moves to, rather than leaving either to a hover-only
attribute. The settings screen SHALL name the scheme in force and say where the control is.

#### Scenario: The menu is on every screen

- **WHEN** any signed-in screen is rendered
- **THEN** the sidebar carries a link to the settings screen, which gathers the advanced toggle and
  every other preference but the colour scheme

#### Scenario: The scheme switch is in the sidebar and nowhere else

- **WHEN** any signed-in screen is rendered, the settings screen among them
- **THEN** the sidebar carries one control that submits the colour scheme
- **AND** the settings screen's own content carries no second control for it

#### Scenario: Submitting the switch moves to the next scheme

- **GIVEN** a browser reading under any scheme on offer
- **WHEN** the sidebar's scheme control is submitted
- **THEN** the next scheme on offer is recorded and the reader is returned to the screen they
  submitted it from
- **AND** submitting it once for each scheme on offer returns the browser to the scheme it started in

#### Scenario: The switch says what it is and what it does

- **WHEN** the sidebar's scheme control is rendered
- **THEN** its accessible name states the scheme in force and the scheme submitting it moves to

#### Scenario: Every scheme renders it

- **WHEN** the interface is rendered under the system, light, dark, and low contrast schemes
- **THEN** the settings screen renders in each, and the sidebar's control names the scheme the page
  is drawn in

#### Scenario: The menu needs no scripting

- **WHEN** a preference is changed with JavaScript disabled
- **THEN** the change is recorded and the reader is returned to the page they made it on

#### Scenario: A listing's columns are chosen in either place

- **WHEN** a reader hides a column from the picker beside the task list
- **THEN** the settings screen's picker for the task list shows that column hidden too

### Requirement: The settings screen owns every per-browser display preference

The settings screen SHALL carry a control for every per-browser display preference this interface
keeps in a cookie, with no preference editable only from elsewhere, except the colour scheme, whose
one control is the sidebar's.

Layout state a script keeps in the browser's own storage rather than in a cookie SHALL NOT count as
a display preference: the sidebar's width and whether it is collapsed, and whether the lanes view
shows completed tasks. Such state SHALL be changed where it is drawn, SHALL NOT be sent to the
server, and SHALL be named in the web interface documentation.

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

#### Scenario: Layout state stays in the browser

- **WHEN** a reader collapses the sidebar or turns on completed tasks in the lanes view
- **THEN** the choice is kept in the browser's own storage, no request carries it, and the settings
  screen offers no control for it
