## ADDED Requirements

### Requirement: The error screen's ways out lead where they say

Every link the error screen offers SHALL be labelled with where it leads, and no link SHALL point at
an address that redirects somewhere other than its label. A failed mutation SHALL offer the screen
it was submitted from. A failed page SHALL offer the same-origin page that linked to it, and SHALL
offer the task list when there is none or when that page is the failing one. The task list SHALL
always be one of the ways out.

#### Scenario: A missing page with no referrer

- **WHEN** a signed-in reader requests a page that does not exist, with no referring page
- **THEN** the error screen offers "Back to tasks" leading to the task list
- **AND** it offers no link to the root address

#### Scenario: A missing page reached from a filtered listing

- **GIVEN** a reader on the task list filtered to one tag
- **WHEN** they follow a link to a task that no longer exists
- **THEN** the error screen leads back to that filtered listing and also offers the task list

### Requirement: The task screen's back link returns to the listing it was opened from

The task screen SHALL offer a link back to the listing the reader opened it from, filter, sort, view
and page intact, and SHALL label it with that listing's name. The listing SHALL be taken from the
`next` the link carried, else from the referring page when that is a task listing or a project board,
else from the `next` of the referring page when that page is the same task screen, else the task
list. Every redirect that returns the reader to the task screen after a change made on it SHALL keep
the listing, so the link survives any number of saves. Deleting the task SHALL return the reader to
that listing. A `next` that is not a relative path on this origin SHALL be replaced by the task list.

#### Scenario: Opened from a filtered list

- **GIVEN** the task list filtered to `tag:ops`
- **WHEN** the reader opens a task from it
- **THEN** the back link is labelled "Back to tasks" and leads to the list filtered to `tag:ops`

#### Scenario: The listing survives a save

- **GIVEN** a task opened from the task list filtered to `tag:ops`
- **WHEN** the reader adds a tag and then a comment on the task screen
- **THEN** the back link still leads to the list filtered to `tag:ops`

#### Scenario: Opened from a project board

- **WHEN** the reader opens a task from a card on the `infra` project's board
- **THEN** the back link is labelled "Back to the infra board" and leads to that board

#### Scenario: Opened from nowhere in particular

- **WHEN** the reader opens a task from a link that carries no listing and no listing referred them
- **THEN** the back link leads to the task list

### Requirement: Adding a task stays on the listing by default

Creating a task from the task list, by the one-line box or by Add with details, SHALL return the
reader to the listing they added from, filter, sort, view and page intact, unless this browser has
turned the "Open a new task" preference on. The listing SHALL then say that the task was created,
SHALL link the new task, and SHALL mark its row or card when it is on the page. A browser that has
never chosen SHALL be treated as having chosen to stay. A browser whose stored preference says it
turned opening on SHALL keep opening the new task.

#### Scenario: Quick add with no preference stored

- **GIVEN** a browser that has never set the preference
- **WHEN** the reader adds a task with the one-line box
- **THEN** they are returned to the listing, the toast says the task was created and links it, and its row is marked

#### Scenario: Add with details

- **GIVEN** a browser that has never set the preference
- **WHEN** the reader creates a task through Add with details
- **THEN** they are returned to the listing rather than to the task

#### Scenario: A reader who chose to open keeps it

- **GIVEN** a browser that turned "Open a new task" on, under this build or an earlier one
- **WHEN** the reader adds a task
- **THEN** the new task's screen is opened

#### Scenario: Adding twice names the second task

- **GIVEN** the reader has just added one task and is on the listing with its toast
- **WHEN** they add another
- **THEN** the toast links and the marked row is the second task, not the first

### Requirement: The task fields chosen in View apply to every drawing of the tasks

The task listing's View menu SHALL offer a Star field and a Repeats field among the task fields, both
shown by default. The Repeats field SHALL draw the repeat glyph together with the task's rule. Hiding
Star SHALL remove the star toggle from rows and cards without changing the order starred tasks are
drawn in, and the star SHALL remain on the task's own screen. Board cards and lane cards SHALL draw a
field the cards carry -- star, status on a lane card, priority, tags, repeat rule and reference --
only when it is chosen.

#### Scenario: A repeating task shows its rule

- **GIVEN** a task repeating `every week` and a browser with no column choice
- **WHEN** the list is drawn
- **THEN** its row shows the repeat glyph and the words `every week`

#### Scenario: Hiding a field hides it on cards

- **GIVEN** the reader has hidden Tags and Repeats under View
- **WHEN** the tasks are drawn as lanes or as a board
- **THEN** no card draws a tag or a repeat rule

#### Scenario: Hiding the star keeps the order

- **GIVEN** a starred task and an unstarred one, and Star hidden under View
- **WHEN** the list is drawn
- **THEN** no star toggle is drawn and the starred task is still first
