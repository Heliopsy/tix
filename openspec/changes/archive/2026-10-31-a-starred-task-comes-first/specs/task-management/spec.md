## ADDED Requirements

### Requirement: A reader can star a task

The system SHALL let any actor who may read a task star it and unstar it. A star SHALL belong to
the actor who set it, within one tenant, and SHALL NOT be visible in another actor's listings.

Starring a task the actor has already starred, and unstarring a task the actor has not starred,
SHALL succeed and SHALL change nothing.

Starring a task the actor cannot read SHALL fail exactly as reading it would.

#### Scenario: Starring marks the task for the reader

- **GIVEN** a task an actor can read
- **WHEN** the actor stars it and then reads it
- **THEN** the task is reported as starred

#### Scenario: A star is personal

- **GIVEN** two actors of one tenant who can both read a task
- **WHEN** the first actor stars it
- **THEN** the task is reported as starred to the first actor and as not starred to the second

#### Scenario: Starring twice is a no-op

- **GIVEN** a task the actor has starred
- **WHEN** the actor stars it again
- **THEN** the call succeeds, the task stays starred, and no further audit entry or event is written

#### Scenario: Unstarring an unstarred task is a no-op

- **WHEN** an actor unstars a task they have not starred
- **THEN** the call succeeds and no audit entry or event is written

#### Scenario: A viewer can star

- **GIVEN** an actor holding only the task read scope
- **WHEN** the actor stars a task
- **THEN** the star is recorded

#### Scenario: Starring outside the caller's reach

- **GIVEN** a token pinned to one project
- **WHEN** it stars a task in another project
- **THEN** the call is refused as reading that task would be

### Requirement: A star is recorded like every other change

Starring and unstarring SHALL each write the star row, an audit entry and an outbox event in one
transaction. The audit actions SHALL be `task.star` and `task.unstar`.

#### Scenario: A star leaves an audit entry

- **WHEN** an actor stars a task
- **THEN** an audit entry with action `task.star` naming the task and the actor is written in the same transaction as the star

### Requirement: A reader's starred tasks lead every task listing

Every task listing SHALL return the reading actor's starred tasks before their other tasks, and
SHALL keep the requested order, including the default urgency order and an explicit sort in
either direction, inside each of the two groups.

A listing read without a reader, such as an export, SHALL be ordered exactly as if nothing were
starred.

#### Scenario: Starred first under the default order

- **GIVEN** three tasks of descending urgency, of which the reader has starred the least urgent
- **WHEN** the reader lists tasks
- **THEN** the starred task is first and the other two follow in urgency order

#### Scenario: Starred first under an explicit sort

- **GIVEN** starred and unstarred tasks with different deadlines
- **WHEN** the reader lists them sorted by due date, ascending or descending
- **THEN** every starred task precedes every unstarred task, and each group is in the requested due-date order

#### Scenario: Another reader's stars do not reorder the listing

- **GIVEN** a task starred by one actor
- **WHEN** a second actor lists tasks
- **THEN** the task appears where the second actor's order puts it, not first

### Requirement: The starred group pages without gaps or repeats

The starred grouping SHALL be the leading key of the keyset ordering and SHALL be carried in the
cursor, so that walking a listing page by page returns every unchanged task exactly once,
including across the page boundary where the starred group ends.

A cursor issued before stars existed SHALL remain valid and SHALL resume among the unstarred
tasks.

A task starred or unstarred while a listing is being walked SHALL move between the groups as a
task whose sort key changed does: the cursor SHALL remain valid, the walk SHALL continue, the
moved task MAY be returned twice or not at all, and no other task SHALL be skipped or repeated.

#### Scenario: Walking across the boundary

- **GIVEN** five starred and seven unstarred tasks
- **WHEN** they are listed two at a time, in either direction and under every sort, following each next cursor
- **THEN** each of the twelve tasks is returned exactly once, the five starred ones first

#### Scenario: Starring during a walk

- **GIVEN** a reader part of the way through the unstarred group
- **WHEN** they star a task they have not reached yet and then request the next page
- **THEN** the next page continues from the cursor, the remaining unchanged tasks are each returned once, and no error is raised

#### Scenario: An old cursor still pages

- **WHEN** a cursor carrying no starred state is presented
- **THEN** the listing resumes among the unstarred tasks after the position the cursor names

### Requirement: Deleting a task removes its stars

Hard deleting a task SHALL remove every actor's star on it, as SHALL anything that removes the
task with it. Soft deleting a task SHALL keep its stars, so that restoring it restores them.
Removing an actor SHALL remove that actor's stars.

#### Scenario: Hard delete clears stars

- **GIVEN** a task two actors have starred
- **WHEN** the task is hard deleted
- **THEN** no star row for it remains

#### Scenario: Restore brings the star back

- **GIVEN** a starred task that is soft deleted and then restored
- **WHEN** the actor who starred it lists tasks
- **THEN** the task is starred and listed first
