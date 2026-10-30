## ADDED Requirements

### Requirement: The sign-in screen presents the tenant in the tenant's colour

The sign-in screen SHALL present the tenant's brand lockup above the sign-in card, and SHALL carry a
soft accent glow behind that card. Every colour on the screen SHALL resolve a design token rather
than a literal value, so that a tenant accent configured with `tix theme` and each of the light,
dark and dim schemes reach it without a further edit.

The glow SHALL be softened by its own gradient stops rather than by a blur filter, and SHALL be
painted behind the card rather than over it.

#### Scenario: The tenant accent reaches the glow

- **WHEN** a tenant configures a non-default accent and its sign-in screen is served
- **THEN** the glow behind the card is drawn in that accent

#### Scenario: The screen renders in every scheme

- **WHEN** the sign-in screen is viewed in the light, dark and dim schemes
- **THEN** each scheme renders its own background, surface, ink and accent, and no element keeps a
  value belonging to another scheme

#### Scenario: No colour is hard-coded

- **WHEN** the stylesheet rules that paint the sign-in screen are read
- **THEN** each colour resolves a custom property rather than a literal value

### Requirement: The sign-in screen does not restyle the screens it shares a layout with

The shell-less layout SHALL serve both the sign-in screen and the refusal screen. Styling added for
the sign-in screen SHALL apply only to the sign-in card, and SHALL NOT change the measure, heading
size or spacing of any other screen rendered in that layout.

#### Scenario: The refusal screen keeps its own shape

- **WHEN** a refusal is rendered in the shell-less layout
- **THEN** its heading and measure are unchanged by the sign-in screen's rules

## MODIFIED Requirements

### Requirement: Usable at phone width

Every web UI screen SHALL be usable on a narrow viewport typical of a phone, with no horizontal page
scrolling and with all controls reachable.

Decorative elements SHALL be bounded by the viewport. An absolutely positioned decoration that
overhangs the viewport adds to the page's scrollable width, and a `overflow-x: hidden` on the body
does not prevent it.

#### Scenario: Board at phone width

- **WHEN** the project board is viewed at a phone-width viewport
- **THEN** the columns remain navigable and the page does not scroll horizontally

#### Scenario: Forms at phone width

- **WHEN** a task detail form is viewed at a phone-width viewport
- **THEN** every field and the submit control are reachable and operable

#### Scenario: A decoration wider than the viewport

- **WHEN** a screen carries a decorative element whose natural size exceeds a phone-width viewport
- **THEN** the element is bounded by the viewport and the page does not scroll horizontally
