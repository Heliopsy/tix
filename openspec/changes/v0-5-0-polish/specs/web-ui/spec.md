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

### Requirement: Every keyset listing carries the same position control

Each listing paged by a keyset cursor SHALL render one shared control, which SHALL say which page the
reader is on and SHALL offer the next page when one exists and the previous page when one exists.

Six listings each carried a different control: five a bare paragraph holding a link, one a row of
actions, two different words for the same direction, and no styling on any of them. None of them said
where the reader was, and none of them offered a way back.

A listing that fits on one page SHALL render no control at all, rather than a pair of controls that lead
nowhere.

#### Scenario: A listing with more than one page

- **WHEN** a listing has a further page
- **THEN** it offers a next control and says which page is on screen

#### Scenario: A listing that fits on one page

- **WHEN** a listing has no further page and no previous one
- **THEN** it renders no position control

#### Scenario: Every paged listing renders the same control

- **WHEN** any keyset-paged listing is rendered
- **THEN** its position control is the shared one, not a control of its own

### Requirement: A reader can go back to the page they came from

A keyset cursor addresses only the page that follows it, so a listing SHALL carry, in its own URL, the
cursors of the pages already walked, and the previous control SHALL return to the last of them.

The first page has no cursor, so the previous control SHALL be absent there rather than present and
inert.

The record of the walk SHALL be treated as untrusted input: a value carrying anything this deployment
did not issue, or more entries than the bound this deployment sets, SHALL be discarded whole rather than
partly kept, and the reader SHALL be returned to the first page. Keeping part of it would send the
previous control to a page the reader was never on, and a value that grows without bound would lengthen
with every page walked.

Changing the filter or the sort SHALL discard the record. Its cursors address positions in one ordered
result set; under a different filter, or a different ordering, they name rows that were never on the
reader's screen.

The whole position SHALL live in the URL, so a link to a page deep in a listing can be shared, reloaded
and bookmarked, and SHALL require no change to the store, the domain or the API.

#### Scenario: Walking back

- **WHEN** a reader on the third page uses the previous control
- **THEN** they are returned to the second page, and its own previous control returns to the first

#### Scenario: The first page

- **WHEN** the first page of a listing is rendered
- **THEN** it offers no previous control

#### Scenario: A record of the walk that was not issued here

- **WHEN** a listing is requested with a record of the walk carrying a value this deployment did not issue
- **THEN** the screen renders, the value is discarded, and the previous control returns to the first page

#### Scenario: A record of the walk longer than the bound

- **WHEN** a listing is requested with more walked pages than the deployment's bound allows
- **THEN** the value is discarded rather than walked

#### Scenario: Changing the filter

- **WHEN** a reader deep in a listing submits a new filter
- **THEN** they start again at the first page with no record of the previous walk

### Requirement: The task list says who each task is assigned to

The task listing SHALL show the handle of the actor each task is assigned to, and SHALL say when a task
is assigned to nobody.

The handle SHALL be resolved for the whole page at once rather than once per row. The listing showed a
task's status, priority, tags, project, reference and date, and not the one thing a shared list is opened
to answer.

A task assigned to nobody SHALL say so, rather than leaving the column empty, which reads as a value
that failed to load.

#### Scenario: An assigned task

- **WHEN** a task with an assignee is listed
- **THEN** the row names that actor by handle, not by identifier

#### Scenario: An unassigned task

- **WHEN** a task with no assignee is listed
- **THEN** the row says it is unassigned

### Requirement: An abandoned claim is visible after it has been swept

A listing SHALL report that a task's last claim expired, for as long as the domain holds that expiry to be
recent, and SHALL say how long ago it lapsed and which actor held it.

The report SHALL be read from the durable evidence of the expiry rather than from the lease fields, which
a sweeper clears within a minute of the lease lapsing. Reading the lease fields made the report
unreachable in practice: a task an agent took and stopped answering for looked exactly like a task nobody
had ever touched, which is the opposite of what the report is for.

A task claimed again after an expiry SHALL report the live claim rather than the old expiry.

How many times a task has been claimed SHALL be shown beside such an expiry, where repeated claims
followed by a lapse distinguish a holder that keeps failing from work that has simply changed hands.

#### Scenario: The sweeper has cleared the lease

- **WHEN** a task whose claim expired is listed after the sweeper has cleared its lease fields
- **THEN** the row reports the expired claim, how long ago it lapsed, and who held it

#### Scenario: The expiry is no longer recent

- **WHEN** the expiry is older than the window the domain treats as recent
- **THEN** the row reports nothing about it

#### Scenario: Claimed again

- **WHEN** a task with an earlier expiry is claimed again and the new lease is live
- **THEN** the row reports the live claim, not the earlier expiry

#### Scenario: A holder that keeps failing

- **WHEN** a task that has been claimed more than once reports an expired claim
- **THEN** the row also says how many times it has been claimed

### Requirement: A column preference records what is hidden

The cookie that carries a reader's column choice SHALL record, for each listing, the columns that listing
leaves OUT, and SHALL show every other column the build declares.

Recording the columns shown cannot distinguish a column the reader turned off from a column that did not
exist when the reader chose, so every column added afterwards read as one the reader had refused. The
readers it silenced were exactly those who had used the picker at all. Project visibility already records
what is hidden, for the same reason, and the two preferences SHALL agree on that.

A listing the reader has never chosen for SHALL keep the declared defaults, which is a different state
from a listing whose hidden set is empty: the first shows the columns declared on by default, the second
shows every declared column. The stored value SHALL distinguish them.

A value written in an earlier form SHALL NOT be read as though it were written in the current one. It
SHALL be recognised and converted, so the reader's recorded choices survive unchanged and no choice is
inverted, and the columns that earlier form could not name SHALL be shown rather than counted as refused.

The value SHALL stay within the size this build will read back for the widest choice the picker can
produce, which is every listing with every column hidden.

#### Scenario: A column is added to a listing

- **WHEN** a build declares a column that did not exist when a reader chose that listing's columns
- **THEN** the column is shown to that reader without the stored choice being touched

#### Scenario: A choice recorded in the earlier form

- **WHEN** a stored value written when the cookie recorded the columns shown is read
- **THEN** every column that reader put away is still put away, and every column they kept is still shown

#### Scenario: Hiding nothing

- **WHEN** a reader ticks every column a listing offers
- **THEN** the listing shows the column that is declared off by default, which an untouched install does not

#### Scenario: The widest choice

- **WHEN** every listing is stored with every column hidden
- **THEN** the value is within the size the build reads back, so the choice is not silently discarded

### Requirement: A refused filter reports on the filter bar

A filter expression the listing cannot answer SHALL be reported beside the filter box, on the listing,
with the expression left in the box, and SHALL NOT replace the screen with the error page.

Every way a term can be refused SHALL be treated alike: an expression that does not parse, and one that
names a project, status, tag or actor this tenant does not have. A rule written per term goes stale the
next time the filter language learns to refuse something.

The screen SHALL be served as a success, because it is the listing that was asked for, and a browser that
swaps a fragment swaps nothing from a response that is not one, which would leave the previous page on
screen carrying no message at all.

A failure that is not the expression's fault SHALL still fail the page.

#### Scenario: A handle nobody has

- **WHEN** a filter names an actor this tenant does not have
- **THEN** the listing renders with the message beside the filter box and the expression still in it

#### Scenario: An expression that does not parse

- **WHEN** a filter expression cannot be parsed
- **THEN** it is reported the same way, on the filter bar

#### Scenario: A failure of the page itself

- **WHEN** a listing fails for a reason that is not the filter expression
- **THEN** the page fails as it did before

### Requirement: The view panel's arrangement and its decoration agree

The panel that holds the column choice and the project choice SHALL size each to what it holds: the
column list is fixed by the build, the project list grows with the tenant.

A rule drawn between the two SHALL follow the arrangement they are actually in, rather than a viewport
width that only guesses at it. A track count chosen by the browser can wrap the sections apart at any
width, leaving a rule that separates nothing.

Each choice's submit SHALL name what it commits, since two controls in one panel both labelled only
"Apply" say nothing about which is which.

#### Scenario: The panel is too narrow for two columns

- **WHEN** the panel renders in one column
- **THEN** the rule between the sections runs across rather than down

#### Scenario: Committing one of the two choices

- **WHEN** the panel is open
- **THEN** each submit names the choice it applies

### Requirement: The task listing is named for what it shows

The task listing SHALL carry a heading, and a navigation entry, that name the set of tasks it actually
renders. The listing is the tenant's queue: it carries every actor's rows, people and agents alike, and
the summary beneath it counts the projects they span and the ones an agent is holding.

The listing SHALL NOT be presented as the reader's own work while it renders everybody's. A reader who
wants only their own rows already has a precise way to ask for them, the `assignee:<handle>` term of the
filter expression, which the assignee column writes into the filter box in one click; a heading is not a
second, weaker way of saying the same thing.

The heading and the navigation entry SHALL agree with each other and with the rows below them, so that
neither can be corrected while the other goes on claiming otherwise.

#### Scenario: The listing carries rows assigned to other actors

- **WHEN** the task listing renders rows belonging to several actors
- **THEN** its heading names the listing rather than claiming it is the reader's own work

#### Scenario: The entry that leads to it

- **WHEN** the navigation is rendered
- **THEN** the entry leading to the task listing names it the same way its heading does

### Requirement: A retention window is written the way it is shown

A retention window SHALL be rendered in the duration vocabulary the product prints elsewhere, so that a
thirty day window reads as `30d` rather than as `720h0m0s`.

The field SHALL accept every value it renders. A form that prints a value its own handler refuses is a
form that rejects its own contents on save, so the rendering and the parsing are one change and not two:
the screen SHALL read a submitted window through the same grammar that produced the value in the box,
which knows the day unit that Go's own duration syntax does not.

A value that grammar does not accept SHALL still be refused, with a message naming the field and the
value and saying what a window looks like.

#### Scenario: Saving the value the screen rendered

- **WHEN** the value a retention field renders is submitted back unchanged
- **THEN** it is accepted, and the field renders the same value again

#### Scenario: A window that is not a duration

- **WHEN** a retention field is submitted with a value the duration grammar does not accept
- **THEN** the submission is refused and the message says what a window looks like

### Requirement: Every row of the tenant tree carries a figure or says why it has none

Each row of the diagram showing what sits under a tenant SHALL render either its live count or, in the
count's place, a short statement of why it carries none.

A blank beside a column of figures reads as a count that failed rather than one deliberately not made,
and a count that really did fail read exactly like one nobody attempted. A row whose figure the service
cannot produce SHALL name where that figure does live instead: a task listing is paginated by cursor and
carries no total, and a custom field is declared on one project at a time.

A row whose listing errored SHALL say that its count is unavailable rather than rendering nothing.

#### Scenario: A kind the service can count

- **WHEN** the tenant tree renders a kind whose listing returns
- **THEN** that row carries the figure

#### Scenario: A kind that has no tenant-wide figure

- **WHEN** the tenant tree renders a kind the service cannot total
- **THEN** that row says where its figure lives, in the figure's place

#### Scenario: A listing that failed

- **WHEN** a count cannot be made because its listing errored
- **THEN** the row says the count is unavailable rather than leaving a gap

### Requirement: A field notice sits on the line of the control it explains

The small disclosure that carries a field's explanation SHALL be rendered on the same line as the label
or control it belongs to.

The disclosure shows only a one-character marker until it is opened, so one left to fall onto a line of
its own reads as an icon whose text failed to render rather than as an affordance. Placement is therefore
part of the affordance and not decoration.

#### Scenario: A notice beside a checkbox

- **WHEN** a notice explains a checkbox rather than a labelled input
- **THEN** it renders on that checkbox's line, as it does beside every other control that carries one

### Requirement: A value a screen shows but cannot change is still a field

A value rendered on a form that no control on that form can change SHALL carry a label and, where the
value's purpose is not evident from the label, the same notice idiom the fields beside it carry. It SHALL
be rendered as read-only rather than editable, and SHALL NOT be submitted with the form.

The tenant key was a bare run of body text between a field's help and the form's submit, reading as
output left behind rather than as a value anybody was meant to use. A label and a notice are what say
which value it is, that a shell addresses this tenant by it, and that this form cannot change it; the
read-only rendering is what stops a reader typing into a box whose Save was never going to carry it.

The value SHALL remain selectable, because the reason it is on the screen is that it has to be copied
somewhere else.

#### Scenario: A value no handler saves

- **WHEN** a form renders a value that none of its handlers accept
- **THEN** the value is labelled, rendered read-only, and not submitted with the form

#### Scenario: A value whose purpose the label alone does not give

- **WHEN** such a value is one used outside the browser
- **THEN** its notice says where it is used and that this form cannot change it

### Requirement: Arriving at a listing leaves the reader on the listing

A listing SHALL NOT move focus away from the page when it loads. The reader arrived to read the rows, and
a form at the top of the screen taking focus makes the form the subject: it draws a focus ring onto the
loudest position on the screen, starts a screen reader at a text box rather than at the page's heading,
and on a small screen raises the keyboard over the rows.

Where a listing carries a control worth reaching without a pointer, the keyboard scheme SHALL bind a key
to it and that binding SHALL appear in the shortcut help, so reaching the control costs one keystroke
rather than a page that has already decided for everybody.

#### Scenario: Loading the task list

- **WHEN** the task list is rendered, with or without a message from a previous action
- **THEN** no control on it takes focus, and the reader's starting point is the page itself

#### Scenario: Reaching the quick-add field

- **WHEN** a reader presses the key their scheme binds to the new-task action
- **THEN** focus moves to the quick-add field
