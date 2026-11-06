## ADDED Requirements

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
