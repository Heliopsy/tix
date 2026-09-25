## ADDED Requirements

### Requirement: Completing a task animates only the task that was completed

The web interface SHALL animate the completion mark only on the row that was acted on.

A checkbox that is already complete SHALL NOT replay its completion animation when the page is rendered
for any other reason: a load, a filter change, a column change or a navigation. Motion on a screen means
something just happened, and a page that animates fifteen checkboxes in sequence on load says that fifteen
things happened when nothing did.

Everything static about the completed state SHALL remain outside the animation, so that a page rendered
without JavaScript still paints completion marks correctly.

#### Scenario: Ticking a task

- **WHEN** a task is completed from the list
- **THEN** exactly one completion mark animates, on the row that was completed

#### Scenario: Loading a list that already contains completed tasks

- **WHEN** a list containing completed tasks is rendered
- **THEN** no completion mark animates

#### Scenario: Completion marks render without JavaScript

- **WHEN** the page is rendered and no script runs
- **THEN** completed tasks still show their completion mark

### Requirement: Assets carry a cache validator

Every static asset response SHALL carry a validator derived from the bytes served, and SHALL answer a
conditional request for an unchanged asset without sending the body again.

An asset URL that does not change between releases and carries no validator lets a browser keep serving
the previous release's stylesheet against the current release's markup, with nothing to tell either side
that it has happened. This was observed during review: a reader saw new markup styled by an old sheet and
reported the interface as broken.

#### Scenario: An asset is revalidated rather than refetched

- **WHEN** a browser requests an asset it already holds, quoting the validator it was given
- **THEN** the response says it is unchanged and carries no body

#### Scenario: An upgrade replaces a cached asset

- **WHEN** an asset's bytes change and a browser revalidates with the validator from before
- **THEN** it is sent the new bytes rather than told to reuse what it has

#### Scenario: Every asset validates independently

- **WHEN** two assets are served
- **THEN** their validators differ, so changing one does not invalidate the other

### Requirement: A control that opens over a list is not clipped by it

A control that opens a panel over a listing SHALL render that panel outside the listing's clipping
context, and the panel SHALL be dismissable without submitting it.

A panel positioned inside a container that hides its overflow is clipped out of the scrollable area: the
hidden part takes no pointer events and cannot be scrolled to. A reader on a lower row therefore sees a
truncated panel, clicks a choice, and nothing happens at any scroll position, which reads as the page
having frozen.

#### Scenario: A panel opened on a low row is fully usable

- **WHEN** the control is opened on a row near the end of a long listing
- **THEN** every choice in the panel is visible and can be clicked

#### Scenario: A panel can be dismissed

- **WHEN** a panel is open and the reader presses escape or clicks away from it
- **THEN** it closes without applying anything

#### Scenario: One panel at a time

- **WHEN** a panel is open and another row's control is opened
- **THEN** only the newly opened panel remains open

### Requirement: A move through intermediate states is shown before it is applied

A listing offering a state that is reachable only through other states SHALL show the whole route before
it is applied, and SHALL distinguish it from a move of a single step.

Each step SHALL be applied as an ordinary transition, so each writes its own audit entry and emits its own
event. That is the intended behaviour: a task that passed through a state really did pass through it, and
the history says so.

#### Scenario: A multi-step move names its route

- **WHEN** a state is reachable only through one or more intermediate states
- **THEN** the choice spells out the route and how many steps it takes

#### Scenario: Each step is recorded

- **WHEN** a multi-step move is applied
- **THEN** the audit trail carries one transition entry per step, in order

#### Scenario: A move that stops part way says so

- **WHEN** a step is refused after earlier steps have been applied
- **THEN** the report names the state reached and says it stopped, rather than reporting the whole move

### Requirement: A reader chooses how instants are shown to them

The web interface SHALL let a reader choose the date format and timezone their own browser renders
instants in, and SHALL treat the deployment's configuration as the default for a reader who has not
chosen.

A preference SHALL NOT reach any other reader's page. Two people sharing a tenant are frequently in
different timezones, for the same reason they may want different keyboard schemes.

#### Scenario: A reader's zone is their own

- **WHEN** two browsers holding different timezone preferences request the same screen at the same time
- **THEN** each is served its own zone and neither carries the other's

#### Scenario: An unusable preference falls back

- **WHEN** a timezone or format that does not resolve is submitted
- **THEN** it is not stored and the deployment's default is used
