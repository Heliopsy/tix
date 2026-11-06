## ADDED Requirements

### Requirement: The board follows the reader's arrangement

The terminal board of a project SHALL draw its columns by the reader's arrangement of
`project:KEY`: hidden columns SHALL not be drawn, starred columns SHALL lead and positioned ones
follow in their order, and a collapsed column SHALL be drawn as a narrow strip with its name and
count. The command palette SHALL offer to hide, star, collapse and move the selected column, and to
show every hidden column again.

#### Scenario: Hiding the selected column from the palette

- **WHEN** the reader runs "Hide column" from the palette with `done` selected
- **THEN** the board no longer draws `done`, and the web board of the same project hides it too
