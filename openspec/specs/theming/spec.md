# theming Specification

## Purpose

Provides named palettes and applies the chosen one consistently across the terminal and the browser.

## Requirements

### Requirement: A theme is a named palette resolvable everywhere

The system SHALL provide themes as named palettes carrying an accent colour and a soft accent, and SHALL
resolve a theme by name from one registry holding both built-in themes and themes defined in configuration.
Where a name is defined in both, configuration SHALL win.

Every colour SHALL be a six-digit hexadecimal value prefixed with `#`. The system SHALL reject any other
form wherever a theme is defined or named.

#### Scenario: A built-in theme resolves by name

- **WHEN** a caller resolves a name that a built-in theme carries
- **THEN** that theme is returned with its accent and soft accent

#### Scenario: Configuration redefines a built-in name

- **WHEN** configuration defines a theme whose name matches a built-in one
- **THEN** resolving that name returns the configured palette, not the built-in one

#### Scenario: A malformed colour is refused

- **WHEN** a theme is defined with a colour that is not `#` followed by six hexadecimal digits
- **THEN** the definition is rejected and the reason names the offending colour

### Requirement: Every surface uses the tenant's resolved theme

The web interface and the terminal interface SHALL both render a tenant's accent from the same resolved
theme, so that one tenant presents one accent on every surface.

The terminal interface SHALL continue to suppress all colour when the environment asks for none, whatever
theme is resolved. Colours that carry meaning rather than brand, being state category and priority, SHALL
NOT be replaced by the theme.

#### Scenario: One tenant, one accent

- **WHEN** a tenant names a theme and is viewed in the web interface and in the terminal interface
- **THEN** both render that theme's accent

#### Scenario: A theme does not defeat NO_COLOR

- **WHEN** the environment disables colour and the tenant names a theme
- **THEN** the terminal interface emits no colour escapes

#### Scenario: Status colours survive theming

- **WHEN** a tenant names a theme
- **THEN** state category and priority keep their own colours

### Requirement: Themes are listable

The system SHALL list every resolvable theme with its name, its palette, and whether it is built in or
configured.

#### Scenario: Listing shows both sources

- **WHEN** configuration defines a theme and the caller lists themes
- **THEN** both the built-in themes and the configured one are listed, each marked with its source

### Requirement: A tenant that names no theme uses the product default

A tenant with no theme name SHALL resolve to the `default` theme.

This replaces the earlier behaviour, where an unnamed tenant's accent was derived by hashing its key and
id into a small fixed palette. That kept an upgrade from repainting an existing deployment, at the cost of
a fresh install opening in a colour nobody chose, two installs of the same software looking unrelated, and
neither matching the logo.

#### Scenario: A tenant that names nothing

- **WHEN** a tenant's theme is empty or whitespace
- **THEN** its accent is the default theme's accent, whatever its key and id are

#### Scenario: Two tenants that name nothing look the same

- **WHEN** two tenants with different keys and ids both name no theme
- **THEN** they resolve to the same accent

### Requirement: The default theme is the product green

The default SHALL be the product's own green, so the colour is recognisable rather than arbitrary.

#### Scenario: The default is the product green

- **WHEN** the default theme is resolved
- **THEN** its accent is the green the logo uses

### Requirement: A tenant records a theme name, and naming none means the default

A tenant SHALL record the name of a theme. Setting a name that does not resolve SHALL be refused at the
point of writing.

When a tenant names no theme, it SHALL resolve to the default theme, as the requirement on tenants that
name no theme describes.

When a tenant names a theme that no longer resolves, reading SHALL fall back to the default theme rather
than fail.

#### Scenario: Setting an unknown theme is refused

- **WHEN** a tenant is set to a theme name that does not resolve
- **THEN** the write is refused with a not-found error naming the theme

#### Scenario: An unthemed tenant uses the default theme

- **WHEN** a tenant names no theme
- **THEN** its accent is the default theme's accent

#### Scenario: A theme that stopped being defined does not break the page

- **WHEN** a tenant names a theme and configuration no longer defines it
- **THEN** the surfaces render with the default theme and the tenant record is unchanged

### Requirement: A tenant may name a custom accent

A tenant SHALL be able to name a custom accent, written as `#` followed by six hexadecimal digits,
in place of a theme name. The system SHALL store it in lowercase and SHALL resolve it to a theme
whose light accent is the chosen colour moved just far enough to reach 4.5:1 against white, whose
dark accent is the chosen colour moved just far enough to reach 4.5:1 against the dark page, and
whose soft accents are tints of those. Every surface SHALL render it as it renders any resolved
theme.

#### Scenario: A custom accent is accepted

- **WHEN** a tenant's theme is set to `#7A3EF0`
- **THEN** the write succeeds and the tenant records `#7a3ef0`

#### Scenario: A custom accent resolves to a legible theme

- **WHEN** a tenant names a custom accent
- **THEN** the resolved light accent reaches 4.5:1 against white and the resolved dark accent reaches 4.5:1 against the dark page

#### Scenario: A malformed accent is still refused

- **WHEN** a tenant's theme is set to `#7a3e`
- **THEN** the write is refused
