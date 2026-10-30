## RENAMED Requirements

- FROM: `### Requirement: Display preferences live in one always-visible settings menu`
- TO: `### Requirement: Every display preference has exactly one always-reachable control`

## MODIFIED Requirements

### Requirement: Every display preference has exactly one always-reachable control

The browser interface SHALL gather the preferences that apply to every screen into a settings screen
reachable from the sidebar on every signed-in screen. Every control within it SHALL be a plain form
that works with scripting disabled, and the screen SHALL render under every colour scheme on offer,
including the low contrast one. Preferences scoped to one listing, such as which columns it shows,
SHALL stay on that listing rather than moving into the settings screen.

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
  every other preference not scoped to one listing

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
