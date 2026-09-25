## ADDED Requirements

### Requirement: A command seeds a demonstrable backlog

The command line SHALL provide a command that fills a database with a backlog spanning a configurable
window, so that the screens can be shown doing the thing they exist for rather than empty.

The seeded data SHALL be written through the ordinary service calls on a clock the command advances, not
by writing rows or importing a snapshot. Statistics attribute a completion by matching the transition
entry written in the same instant as the completion, so data written behind the service leaves every
leaderboard and lead-time figure empty, which is the opposite of the point.

The seeded backlog SHALL carry descriptions, tags, priorities, assignees, due dates and custom fields,
completions spread across many days and performed by several actors, and lead times that vary.

#### Scenario: Seeding a scratch database

- **WHEN** the command is run against a database holding no tasks or users
- **THEN** projects, actors, custom field definitions, tasks, comments and dependencies are written across
  the requested window, and the command reports what it wrote

#### Scenario: The statistics screens have something to show

- **WHEN** statistics are read over the seeded window
- **THEN** completions fall on many different days, the leaderboard names more than one actor, and the
  median lead time is not zero

#### Scenario: A database that already holds work is refused

- **WHEN** the command is run against a database that already holds tasks or users
- **THEN** nothing is written, and the refusal says to re-run with the reset flag or to point at a scratch
  database

#### Scenario: Replacing seeded data

- **WHEN** the command is run with the reset flag
- **THEN** the tasks, projects and users already there are removed before the new backlog is written

### Requirement: Durations are rendered for a reader

Every surface SHALL render a duration it shows to a person in a form that drops precision as the
magnitude rises, and SHALL NOT show the machine rendering of a duration on a screen meant to be read.

The rendering SHALL belong to the duration type, so the command line, the terminal interface and the
browser cannot drift apart on the same figure. The machine-readable output formats SHALL keep the full
precision, because a snapshot is built from them.

#### Scenario: A lead time counted in weeks

- **WHEN** a statistics screen shows a lead time of 384 hours
- **THEN** it reads as a count of days rather than as hours, minutes and fractional seconds

#### Scenario: The machine formats are unchanged

- **WHEN** the same figure is written as JSON or YAML
- **THEN** it carries the full precision
