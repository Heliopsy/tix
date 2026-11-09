## ADDED Requirements

### Requirement: A project carries a personal star toggle

Each row of the projects screen and the project page's header SHALL draw an always-visible star toggle
for the reader's own star on that project, with `aria-pressed` and a tooltip of Star this project or
Unstar. It SHALL be a form that works without scripting and, with htmx, SHALL replace only itself.

#### Scenario: Starring from the projects screen

- **WHEN** a reader presses the outline star on a project's row
- **THEN** the toggle comes back filled with `aria-pressed="true"` and the project is listed under Projects in the sidebar

#### Scenario: Without scripting

- **WHEN** the toggle's form is submitted without htmx
- **THEN** the reader is redirected back to the page the toggle was on

### Requirement: Stars are personal

A project star SHALL belong to the reader who set it within one tenant. Nobody else SHALL see it on any
screen, and it SHALL be the same mark as the star on that project's lane.

#### Scenario: Another person does not see it

- **GIVEN** Ada starred INFRA
- **WHEN** Bob opens the projects screen
- **THEN** INFRA is not in a Starred section and its toggle is not pressed

#### Scenario: A pin made before stars

- **GIVEN** a lane star on INFRA saved before this change
- **WHEN** its owner opens the projects screen
- **THEN** INFRA is listed under Starred

### Requirement: The projects screen lists starred projects first

The first page of the projects screen SHALL draw a Starred section holding the reader's starred projects
before the rest, and SHALL not repeat them below. `?starred=1` SHALL draw only the Starred section.

#### Scenario: Starred only

- **WHEN** a reader opens `/projects?starred=1`
- **THEN** only the projects they starred are listed

### Requirement: The sidebar lists starred projects

The sidebar SHALL list the reader's starred projects under Projects, at most twelve, in the lanes' order.
When there are more it SHALL end the list with Show all, linking to `/projects?starred=1`. The
interface SHALL call the mark Star and Starred, never Pin.

#### Scenario: More than twelve

- **GIVEN** a reader starred thirteen projects
- **WHEN** any screen is drawn
- **THEN** the sidebar lists twelve and a Show all link
