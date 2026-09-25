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
- **THEN** the tasks and projects already there are removed before the new backlog is written, and the
  accounts are kept so that the seed can put its credentials back on them

#### Scenario: A dropped claim is visible in the seeded backlog

- **WHEN** the seeded backlog is read
- **THEN** a small number of tasks report a claim that expired within the evidence window, at least one of
  them having been claimed more than once, so the state a lease expiry leaves behind can be seen without
  arranging it by hand

#### Scenario: The seeded tenant shows only the lists it wrote

- **WHEN** the command is run against a database carrying the starter lists a fresh installation is given
- **THEN** the starter lists that hold no task are removed, so the projects screen and the project filters
  name only the lists the backlog works out of

### Requirement: The seeded demonstration can be signed in to

The seeding command SHALL leave at least one credentialed account that can sign in to the browser
interface, created through the same service call the user-creation command makes, so that a seeded
demonstration is one identity per person rather than an account beside the actor that wrote the history.

The accounts SHALL be identities the fixture already writes history as, at least one of them an
administrator and at least one holding a different role, so the screens can be shown as more than one
kind of user. An account that the fixture drives as an agent SHALL NOT be given a password, because an
agent authenticates with the token minted beside it.

The password SHALL be a documented default that the caller may replace with a flag, because an account
nobody can guess the password of leaves the demonstration as unreachable as no account at all. The seeded
data is demonstration data by construction, and a database that already holds work or accounts is
refused before any of it is written.

The command SHALL report the credentials it left in every output format, so that the person who ran it is
told how to open what it wrote without reading the documentation.

#### Scenario: Signing in to a freshly seeded database

- **WHEN** a database is seeded and the reported email and password are presented to the sign-in path
- **THEN** a session is issued, and a password that is not the reported one is rejected

#### Scenario: Re-seeding leaves a working sign-in

- **WHEN** the command is run with the reset flag against a database seeded before
- **THEN** the accounts are re-credentialed with the password the run reports, and signing in with it
  succeeds

#### Scenario: The credentials are reported

- **WHEN** the seed finishes
- **THEN** the table output names the email, password and role of every account that can sign in, and the
  machine-readable formats carry the same fields

#### Scenario: Agents hold tokens rather than passwords

- **WHEN** the seeded agents are examined
- **THEN** none of them is reported as a sign-in, and none of them can sign in with the seeded password

#### Scenario: A handle held by an actor with no account

- **WHEN** the command is run against a database where a seeded handle belongs to an actor no account
  stands behind
- **THEN** the seed is refused, naming the handle, rather than writing a history whose authors nobody can
  sign in as

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

### Requirement: A duration the product prints can be typed back

The product SHALL read durations in one vocabulary everywhere a person supplies one, and that vocabulary
SHALL include everything it renders. A duration rendered for a reader SHALL parse, and parsing it and
rendering it again SHALL produce the identical text.

The vocabulary SHALL be Go's duration syntax extended with a day unit of exactly twenty-four hours, and
SHALL accept whitespace between terms, because that is the form the reader's rendering takes. It SHALL NOT
add a unit nothing renders, since a unit accepted but never printed is the same asymmetry facing the other
way. The grammar SHALL belong to the duration type, so the command line, the HTTP interface, the browser
and the configuration reader cannot drift apart on what a duration is.

The machine rendering SHALL stay Go's syntax, because snapshots and JSON round-trip through it.

#### Scenario: A retention window as the screen shows it

- **WHEN** a retention window is given as `30d`
- **THEN** it is accepted as seven hundred and twenty hours, on the command line, over HTTP and in
  configuration

#### Scenario: A statistics window in days

- **WHEN** `--window 30d` is passed to the statistics command
- **THEN** the window is thirty days, and the refusal for an unreadable value names a form that works

#### Scenario: The reader's rendering is valid input

- **WHEN** a duration is rendered for a reader as `16d 1h`
- **THEN** that text parses, and rendering the result again gives `16d 1h`

#### Scenario: The snapshot format is unchanged

- **WHEN** a duration is written to a snapshot or to JSON
- **THEN** it carries Go's syntax, which Go's own parser still reads

### Requirement: A failed listing writes no answer to standard output

A listing command that fails SHALL NOT write a complete document to standard output. A document that
parses is an answer, and a consumer reading only standard output, which is the ordinary shape of a
pipeline, cannot tell an empty answer from a failure it never saw.

Where records were already streamed before the failure, the command SHALL NOT complete the document: a
bracketed format is left unterminated, so a consumer parsing standard output gets a syntax error rather
than a short listing it would believe. A record-oriented format has no terminator to withhold, and the
records already written stand, each true on its own.

The failure SHALL still be reported on standard error with the exit status of its kind.

#### Scenario: A listing that fails before it writes

- **WHEN** a listing is filtered by an assignee that does not resolve, in any output format
- **THEN** standard output is empty, the failure is on standard error, and the exit status is the one for
  the failure's kind

#### Scenario: A listing that fails part way through

- **WHEN** a bracketed listing fails after records have been written
- **THEN** the document is left unterminated and cannot be parsed as a complete listing

### Requirement: The binary carries its own zone database

The binary SHALL embed the zone database rather than depend on the host providing one, so that a reader's
chosen timezone is honoured on any base image.

#### Scenario: A base image with no tzdata

- **WHEN** the binary runs where the host has neither a zone database nor Go's bundled archive
- **THEN** a named timezone still resolves, rather than falling back to the deployment default without
  telling anyone
