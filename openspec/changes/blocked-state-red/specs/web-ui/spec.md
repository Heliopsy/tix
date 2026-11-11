## ADDED Requirements

### Requirement: Blocked is drawn in red

The web interface SHALL draw a state in the `blocked` category in the danger red on every surface that
colours a state: its status chip, its badge, its status tint, the statistics breakdown and the heading of
its board column. A board column heading SHALL take the colour of its state's category.

#### Scenario: The default workflow's board

- **GIVEN** a project on the default workflow
- **WHEN** its board is shown
- **THEN** the Blocked column heading carries the blocked category class, which the stylesheet draws in the danger red
