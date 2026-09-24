## ADDED Requirements

### Requirement: The system reports throughput, ageing and who closed what

The system SHALL report, for a tenant and optionally one project, over a window ending now:

- the number of tasks that reached a terminal state in the window, and the same count per day
- the number of tasks created in the window
- the median and slowest time from creation to terminal state, over tasks that reached one in the window
- the count of tasks currently in each state category
- the actors who moved the most tasks to a terminal state, with their counts
- the oldest tasks not yet in a terminal state, with their age

Every count SHALL be scoped to one tenant, and SHALL NOT include tasks from any other tenant.

#### Scenario: Throughput counts terminal transitions in the window

- **WHEN** a task reaches a terminal state inside the window
- **THEN** it is counted once in the completed total and once on the day it happened

#### Scenario: Work outside the window is excluded

- **WHEN** a task reached a terminal state before the window began
- **THEN** it is not counted in the completed total, and it is not in the ageing figures

#### Scenario: Deleted tasks are excluded

- **WHEN** a task is soft deleted
- **THEN** it appears in no count

#### Scenario: Statistics never cross a tenant

- **WHEN** two tenants each hold tasks and statistics are read for one
- **THEN** no count, actor or task from the other tenant appears

#### Scenario: An empty window reports zeroes rather than failing

- **WHEN** statistics are read for a window in which nothing happened
- **THEN** every count is zero, every list is empty, and the call succeeds

### Requirement: Statistics are reachable from every surface

Statistics SHALL be readable from the command line, over HTTP, in the web interface and in the terminal
interface, and SHALL present the same figures for the same tenant, project and window.

The command line SHALL offer machine-readable output, as every other read does.

#### Scenario: The same window gives the same numbers everywhere

- **WHEN** the same tenant, project and window are read from the command line and from the web interface
- **THEN** the figures are the same

#### Scenario: Statistics are machine readable

- **WHEN** statistics are requested as JSON
- **THEN** the output parses and carries every figure the screen shows

### Requirement: The leaderboard says what it counts

Any presentation of the most active actors SHALL state that the figure counts tasks moved to a terminal
state, so the number is not mistaken for a measure of productivity.

#### Scenario: The wording names the measure

- **WHEN** the most active actors are presented on any surface
- **THEN** the presentation states that the count is of tasks moved to a terminal state
