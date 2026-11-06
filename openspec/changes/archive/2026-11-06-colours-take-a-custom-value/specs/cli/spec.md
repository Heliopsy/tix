## ADDED Requirements

### Requirement: A custom project colour on a terminal

Where the CLI or the terminal interface draws a project colour that is a custom value, it SHALL draw
that colour itself on a terminal that declares truecolor, and the nearest colour the terminal can
show otherwise. With colour disabled it SHALL print the hexadecimal value as text.

#### Scenario: Truecolor terminal

- **WHEN** a project with a custom colour is listed with colour enabled and `COLORTERM=truecolor`
- **THEN** its swatch is drawn with a 24-bit foreground of that colour

#### Scenario: Sixteen colour terminal

- **WHEN** a project with a custom colour is listed with colour enabled and no truecolor declared
- **THEN** its swatch is drawn in the palette colour nearest to it

#### Scenario: No colour

- **WHEN** a project with a custom colour is listed with colour disabled
- **THEN** the colour column holds the hexadecimal value and no escape sequence
