## ADDED Requirements

### Requirement: The projects view stars projects

The terminal's projects view SHALL mark a starred project with a star, toggle the reader's own star on
the selected project with the Star key, and with the Filter key SHALL switch between every project and
only the starred ones.

#### Scenario: Filtering to starred

- **GIVEN** the reader starred one of three projects
- **WHEN** they press the Filter key in the projects view
- **THEN** the view lists that one project
