# web-ui Specification

## Purpose

Provides the browser interface, at functional parity with the other access paths except where an exemption is recorded.

## Requirements

### Requirement: Functional parity with the other access paths

The web UI SHALL provide every operation available through the CLI, the TUI, and the HTTP API, except operations carrying a recorded exemption. An operation reachable from any other access path SHALL be reachable from the web UI.

#### Scenario: A CLI operation is reachable in the browser

- **WHEN** an operator performs an operation from the CLI
- **THEN** the same operation can be performed from the web UI with the same effect

#### Scenario: A new operation must reach the web

- **WHEN** a new operation is added to the service and wired to the CLI and the API but not to the web UI
- **THEN** the build fails

### Requirement: The web binding is declared in the capability registry

Every browser screen that exposes an operation SHALL be declared as that operation's web binding in the capability registry, naming the route and the template it renders. The registry SHALL be the single source of truth for which operations the browser reaches, and a browser route SHALL NOT expose an operation the registry does not declare.

#### Scenario: Web binding names its route and template

- **WHEN** a registry entry declaring a web binding is read
- **THEN** it names the browser route and the embedded template that renders it

#### Scenario: Every operation reaches the browser or says why not

- **WHEN** the registry is checked for web bindings
- **THEN** every operation either names a browser route or carries a web exemption with a written reason

### Requirement: Parity is mechanically verified

Parity SHALL be verified by tests that fail the build, not asserted in documentation. The tests SHALL check that every registered operation has a web binding and that every declared binding resolves against the real HTTP mux, the real command tree, and the real embedded templates.

#### Scenario: Missing web binding fails the build

- **WHEN** a registered operation has no web binding and no exemption
- **THEN** the parity test fails

#### Scenario: Dangling binding fails the build

- **WHEN** a registry entry names a web route that the served mux does not have
- **THEN** the parity test fails

#### Scenario: Missing template fails the build

- **WHEN** a web binding references a template that is not present in the embedded assets
- **THEN** the parity test fails

### Requirement: Exemptions are explicit and justified

An operation genuinely inapplicable to a browser SHALL carry an explicit exemption recorded in the registry with a written justification. An operation SHALL NOT be absent from the web UI without such an exemption.

#### Scenario: Exemption permits absence

- **WHEN** an operation carries a recorded exemption with a justification
- **THEN** the parity test accepts its absence from the web UI

#### Scenario: Exemption without justification is rejected

- **WHEN** an exemption is recorded with an empty justification
- **THEN** the parity test fails

#### Scenario: Exemptions are enumerable

- **WHEN** the exemption list is requested
- **THEN** every exempted operation is listed with its justification, so exemptions can be reviewed

### Requirement: Project board grouped by workflow state

The web UI SHALL present a project board with one column per state of the project's workflow, showing tasks in their current state, and SHALL allow a task to be moved between states subject to the workflow's transition rules.

#### Scenario: Columns follow the workflow

- **WHEN** a project uses a workflow with four states
- **THEN** the board shows four columns in the workflow's declared order

#### Scenario: Legal move succeeds

- **WHEN** a user moves a task to a state the workflow permits from its current state
- **THEN** the task's state changes and the board reflects the new position

#### Scenario: Illegal move is refused

- **WHEN** a user attempts to move a task to a state the workflow does not permit from its current state
- **THEN** the move is refused with an explanatory message and the task remains in its original state

### Requirement: Filterable task list sharing the CLI filter grammar

The web UI SHALL provide a task list whose filter expressions use the same grammar as the CLI filter, so an expression valid in one is valid and equivalent in the other.

#### Scenario: CLI expression works in the browser

- **WHEN** a filter expression that selects a set of tasks in the CLI is entered in the web task list
- **THEN** the same set of tasks is shown

#### Scenario: Invalid expression is explained

- **WHEN** a malformed filter expression is entered
- **THEN** the UI reports the parse error rather than silently returning all tasks

#### Scenario: Listing is paginated by cursor

- **WHEN** a filtered list exceeds one page and the next page is requested
- **THEN** the next page continues without skipping or repeating a task

### Requirement: Task detail screen

The web UI SHALL provide a task detail screen showing the task's built-in fields, its custom fields, its subtasks, its dependencies, its comments, its artifacts, and its audit history, and SHALL allow each of these to be edited where the caller is authorized.

#### Scenario: All sections are present

- **WHEN** a task with subtasks, dependencies, comments, artifacts, and prior edits is opened
- **THEN** each of those sections is shown on the detail screen

#### Scenario: History is shown

- **WHEN** the history section of a task is viewed by a caller with audit read scope
- **THEN** each recorded change is listed with its actor, time, source, and the fields that changed

#### Scenario: Unauthorized editing is prevented

- **WHEN** a caller without write authorization opens a task
- **THEN** the detail screen presents the task read-only and edit submissions are refused

### Requirement: Workflow and field definition editors

The web UI SHALL provide an editor for per-project workflows, including states and allowed transitions, and an editor for typed custom field definitions.

#### Scenario: Workflow is edited

- **WHEN** an authorized user adds a state and a transition to a project's workflow
- **THEN** the board shows the new column and the new transition is permitted

#### Scenario: Field definition is created

- **WHEN** an authorized user defines a typed custom field
- **THEN** the field appears on task detail screens for that project and values are validated against its type

### Requirement: A workflow editor save changes only what its form edits

Editing part of a workflow SHALL preserve the rest. A save from the workflow editor SHALL change only what its form edits -- the initial state, the state keys with their labels and terminality, the permitted transitions, and the state migrations -- and SHALL carry every other field of a state, a transition or the definition through from what is stored. A state or transition the form no longer names SHALL be removed.

#### Scenario: Saving without changing anything changes nothing

- **WHEN** an authorized user opens the workflow editor and saves it with no edits
- **THEN** every state keeps its reporting category and its lease-expiry revert, every transition keeps the scope and the comment it requires, and the workflow keeps its default lease

#### Scenario: A restricted transition stays restricted

- **WHEN** a transition requires a scope and the workflow is saved from the editor with no edits
- **THEN** an actor holding the plain transition scope but not the required one is still refused that move, and an actor holding the required scope may still make it

#### Scenario: A removed state is removed

- **WHEN** an authorized user deletes a state line and the transitions naming it
- **THEN** that state and those transitions are gone from the stored workflow

### Requirement: A workflow editor rename is kept only when declared as a migration

A state key the form renames SHALL keep the fields the editor does not show when the save declares the rename in its migration lines. A rename the save does not declare SHALL be treated as removing one state and adding another, which the service refuses outright while any task still sits in the removed state.

#### Scenario: A renamed state keeps what the editor does not show

- **WHEN** an authorized user changes a state's key and gives the rename as a migration line
- **THEN** the state under its new key keeps the category and the lease-expiry revert the old key carried

#### Scenario: Invalid workflow is refused

- **WHEN** a workflow edit would leave tasks in a state the workflow no longer defines
- **THEN** the change is refused with an explanation and the workflow is unchanged

### Requirement: The workflow editor's terminal field accepts only the words its help names

The third field of a state line SHALL accept `terminal` or `open` and `true` or `false`, SHALL read an empty field as open, and SHALL refuse a value that names neither rather than reading it as open. The help text beside the field SHALL name the values it accepts.

#### Scenario: The documented terminal words are read as written

- **WHEN** a state line ends in `true`, having followed the help text beside the field
- **THEN** the state is stored as terminal

#### Scenario: An unreadable terminal field is refused

- **WHEN** a state line ends in a word that is neither a yes nor a no
- **THEN** the save is refused with an explanation and no workflow is stored

### Requirement: Tenant, domain, user, and token administration

The web UI SHALL provide administration screens for tenants and their memberships, for domain mappings, for
users, and for API tokens, restricted to callers holding the corresponding administrative scopes.

#### Scenario: Administrative screens require scope

- **WHEN** a caller without administrative scope requests an administration screen
- **THEN** access is refused and no administrative data is rendered

#### Scenario: Domain mapping is manageable

- **WHEN** an authorized administrator adds a domain mapping for a tenant
- **THEN** the mapping appears in the domain list and its verification state is shown

### Requirement: The token issue form sets every token property, expiry included

The token screen's issue form SHALL be able to set every property of a token the service accepts and the
listing displays, which SHALL include its scopes and its expiry. The expiry control SHALL offer a set of
durations and an explicit "never expires" choice, and SHALL propose an expiry rather than proposing no
expiry. A value the control does not offer SHALL be refused rather than read as no expiry.

#### Scenario: An expiry set on the form reaches the token

- **WHEN** an administrator issues a token choosing an expiry of seven days
- **THEN** the token's stored expiry is seven days ahead
- **AND** the listing's expires column shows it

#### Scenario: A token issued with no expiry says so

- **WHEN** an administrator issues a token choosing never expires
- **THEN** the listing's expires column says the token does not expire

### Requirement: The token listing shows each token's dates and whether it still authenticates

The token listing SHALL show, as optional columns behaving like every other optional column, when each
token was created and when it expires, and SHALL show whether each token still authenticates: active,
expired, or revoked with the time it was revoked.

#### Scenario: The listing says when a token was created

- **WHEN** the token listing is read
- **THEN** each row shows when that token was created

### Requirement: Revoking a token is confirmed first and never offered for a revoked one

A revoked token SHALL NOT offer a revocation control. Revocation, being irreversible and ending a
credential something may be holding, SHALL be behind a confirm dialog that names the token it ends
and says what revoking does.

#### Scenario: A revoked token is shown as revoked and offers no revocation

- **GIVEN** a revoked token
- **WHEN** the token listing is read
- **THEN** the row is marked revoked
- **AND** the row offers no control to revoke it

#### Scenario: Revocation is confirmed before it happens

- **GIVEN** a live token
- **WHEN** the token listing is read
- **THEN** the row's Revoke button opens a dialog that names the token and says that whatever holds it
  stops authenticating

### Requirement: A newly issued token's value is shown whole in a region of its own

A newly issued token's value SHALL be presented in a region of its own, distinct from the interface's
transient confirmation message, readable in full without being cropped, selectable, and accompanied by a
copy control. The value SHALL remain readable in full when no scripting is available.

#### Scenario: Token is issued once

- **WHEN** an authorized administrator creates an API token
- **THEN** the token value is displayed once at creation and is not retrievable afterwards

#### Scenario: The issued value is not presented as a confirmation message

- **WHEN** a token has just been issued
- **THEN** its value is in its own region rather than in the transient confirmation message
- **AND** a copy control points at it

### Requirement: Webhook administration with delivery log and redelivery

The web UI SHALL allow webhook endpoints to be configured, SHALL show a delivery log with the state of each delivery attempt, and SHALL allow a delivery to be redelivered.

#### Scenario: Delivery log shows attempt state

- **WHEN** a webhook delivery has failed and been retried
- **THEN** the delivery log shows the attempts and the current state

#### Scenario: Redelivery is available

- **WHEN** an authorized user triggers redelivery of a failed delivery
- **THEN** a new delivery attempt is queued and appears in the log

#### Scenario: Secrets are not displayed

- **WHEN** a configured webhook endpoint is viewed
- **THEN** its signing secret is not shown

### Requirement: Import, export, and sync screens

The web UI SHALL provide screens for exporting a snapshot, importing a snapshot with mode selection and dry run, and running or refreshing an external import with its dry run and its full refresh.

#### Scenario: Export is downloadable

- **WHEN** an authorized user exports a tenant from the web UI
- **THEN** a snapshot document is produced in the selected format

#### Scenario: Dry run is shown before import

- **WHEN** a user uploads a snapshot and requests a dry run
- **THEN** the planned creations, updates, and skips are displayed and nothing is written

#### Scenario: External import reports its result

- **WHEN** an external import is run from the web UI
- **THEN** the created, updated, skipped, and lossy results are displayed

#### Scenario: A full refresh is available in the browser

- **WHEN** an operator runs an external import from the web UI and asks for a full refresh
- **THEN** the run ignores the stored cursor and reconsiders every source record

### Requirement: The sync screen explains what a source is before asking for one

The external sync screen SHALL state, on the screen itself, what an import reads from, what each available adapter is for, where a source's settings are supplied, what the mapping file governs, what a dry run does, and what a second run does. It SHALL NOT collect any value that a sync source record does not keep.

#### Scenario: An operator learns what generic means without leaving the page

- **WHEN** an operator opens the external sync screen
- **THEN** the screen states that the generic adapter reads a CSV or JSON file and is the choice for a system with no adapter of its own

#### Scenario: The screen names where settings are supplied

- **WHEN** a source is configured
- **THEN** the screen names the exact environment variables that source reads, derived from its name, and only those its system reads

#### Scenario: No field is collected that is not stored

- **WHEN** an operator registers a source from the web UI
- **THEN** the form collects only the values a sync source record keeps, and no field whose value would be discarded

#### Scenario: Idempotent refresh is explained

- **WHEN** an operator reads the external sync screen
- **THEN** the screen states that a re-run updates entities carrying an external reference rather than duplicating them, and that a matching external version is skipped

### Requirement: A view of what imports have run

The web UI SHALL provide a view of completed external imports, listing for each the time it ran, the source and system it read, and the entities it created, updated, and skipped, together with the current state of every configured source. The view SHALL state which runs it cannot show.

#### Scenario: A completed run is listed

- **WHEN** an external import completes
- **THEN** it appears in the run history with its time, its source, its system, and its created, updated, and skipped counts

#### Scenario: A dry run leaves no row

- **WHEN** an external import is run as a dry run
- **THEN** no row is added to the run history, and the view states that a dry run does not appear

#### Scenario: A failed run is reported against its source

- **WHEN** an external import fails
- **THEN** the run history states that failed runs do not appear in it, and the source's current state reports the failure

#### Scenario: Run history is scoped to the tenant

- **WHEN** an operator opens the run history
- **THEN** no import belonging to another tenant is shown

### Requirement: Live activity feed

The web UI SHALL provide an activity feed showing recent events for the tenant, updating as new events arrive when JavaScript is available.

The feed SHALL use the same connection mechanism as every other live screen: the same reconnect, the same resume, the same fallback and the same notice when it is not live.

#### Scenario: Feed shows recent activity

- **WHEN** a user opens the activity feed
- **THEN** recent events for the tenant are listed in reverse chronological order

#### Scenario: Feed updates live

- **WHEN** another user changes a task while the feed is open with JavaScript enabled
- **THEN** the change appears in the feed without the user reloading the page

#### Scenario: Feed is readable without JavaScript

- **WHEN** the feed is opened with JavaScript disabled
- **THEN** recent events are still listed, refreshed on page load

#### Scenario: A feed that has stopped receiving says so

- **WHEN** the feed's connection drops and cannot immediately be reopened
- **THEN** the feed says the view may be out of date, and reconnects

### Requirement: Served from the single binary with no build step and no CDN

All web UI assets SHALL be embedded in the binary and served from it. The build SHALL NOT require a JavaScript toolchain, and the running UI SHALL NOT fetch scripts, stylesheets, or fonts from any external network location.

#### Scenario: Offline host serves the UI

- **WHEN** the server runs on a host with no outbound internet access
- **THEN** every UI screen renders fully with all styling and scripts

#### Scenario: Build needs no JavaScript toolchain

- **WHEN** the project is built on a machine with no Node or npm installed
- **THEN** the build succeeds and the resulting binary serves the UI

#### Scenario: No external requests are made

- **WHEN** a UI page is loaded and its outbound requests are observed
- **THEN** every request targets the serving origin

### Requirement: Every form works without JavaScript

Every form in the web UI SHALL submit and function with JavaScript disabled. JavaScript SHALL provide progressive enhancement, including live updates from the event stream, and SHALL NOT be required for any operation.

#### Scenario: Task is created without JavaScript

- **WHEN** a user with JavaScript disabled submits the task creation form
- **THEN** the task is created and the resulting page reflects it

#### Scenario: Board move works without JavaScript

- **WHEN** a user with JavaScript disabled changes a task's state through the form control rather than by dragging
- **THEN** the transition is applied

#### Scenario: Enhancement adds liveness only

- **WHEN** JavaScript is enabled
- **THEN** the same operations are available and additionally update in place as events arrive

### Requirement: Usable at phone width

Every web UI screen SHALL be usable on a narrow viewport typical of a phone, with no horizontal page
scrolling and with all controls reachable.

Decorative elements SHALL be bounded by the viewport. An absolutely positioned decoration that
overhangs the viewport adds to the page's scrollable width, and a `overflow-x: hidden` on the body
does not prevent it.

#### Scenario: Board at phone width

- **WHEN** the project board is viewed at a phone-width viewport
- **THEN** the columns remain navigable and the page does not scroll horizontally

#### Scenario: Forms at phone width

- **WHEN** a task detail form is viewed at a phone-width viewport
- **THEN** every field and the submit control are reachable and operable

#### Scenario: A decoration wider than the viewport

- **WHEN** a screen carries a decorative element whose natural size exceeds a phone-width viewport
- **THEN** the element is bounded by the viewport and the page does not scroll horizontally

### Requirement: Per-tenant branding

The web UI SHALL apply per-tenant branding, at minimum a display name, a logo, and accent colouring, to pages served in that tenant's context.

#### Scenario: Branding follows the tenant

- **WHEN** a tenant configures a display name and logo and its pages are served
- **THEN** those pages show that tenant's name and logo

#### Scenario: Default branding applies when unset

- **WHEN** a tenant has configured no branding
- **THEN** pages render with the default branding rather than failing

#### Scenario: Branding does not leak between tenants

- **WHEN** pages for two tenants are served by the same process
- **THEN** each page shows only its own tenant's branding

### Requirement: The web UI never exposes another tenant's data

Every web response SHALL contain only data belonging to the tenant resolved for that request. Supplying an identifier belonging to another tenant SHALL yield a not-found result rather than that tenant's data.

#### Scenario: Foreign identifier is not found

- **WHEN** a user requests a task detail page using an identifier belonging to another tenant
- **THEN** a not-found result is returned and no field of that task is disclosed

#### Scenario: Listings are scoped

- **WHEN** any listing screen is rendered
- **THEN** every row belongs to the resolved tenant

#### Scenario: Filters cannot widen scope

- **WHEN** a user supplies filter parameters naming another tenant
- **THEN** the response contains no data from that tenant

### Requirement: Project colour and icon distinguish rows

The task list SHALL mark each row with its project's colour as an accent along the row's leading
edge, and SHALL show the project's icon beside it when one is set. The accent SHALL be visually
distinct from the status badges, so that a project stripe is not read as a status. A row whose
project carries neither SHALL render as it did before, without an accent. The project screens SHALL
allow a project's colour and icon to be set from the palette or to a custom colour, and cleared
again.

#### Scenario: Row carries the project accent

- **WHEN** the task list renders a task belonging to a project that has a colour
- **THEN** the row carries that project's colour as a leading-edge accent

#### Scenario: Row carries the project icon

- **WHEN** the task list renders a task belonging to a project that has an icon
- **THEN** the icon is shown on the row

#### Scenario: A project without either renders plainly

- **WHEN** the task list renders a task belonging to a project with no colour and no icon
- **THEN** the row renders without an accent or icon and the list is otherwise unchanged

#### Scenario: Colour remains legible in every scheme

- **WHEN** a project colour is rendered under the light, dark, and low contrast schemes
- **THEN** each scheme renders a shade of that palette token that remains distinguishable against its own background

#### Scenario: Clearing from the browser

- **WHEN** a project's colour is set to none and its icon emptied on the project screen
- **THEN** the project is saved with neither and its rows lose the accent

### Requirement: Listings offer a chosen column set

Every listing screen whose columns can be chosen SHALL offer a control selecting which of its
optional columns render. The available columns and the set an untouched installation shows SHALL be
declared in one place rather than repeated per screen, and a listing with no recorded choice SHALL
render that declared default. A recorded choice that this build does not recognise SHALL be
discarded in favour of the declared default rather than failing the page.

#### Scenario: An untouched installation is unchanged

- **WHEN** a listing is rendered for a browser that has chosen no columns
- **THEN** it renders exactly the declared default set, in the declared order

#### Scenario: A chosen set changes what renders

- **WHEN** a reader chooses a column set for a listing
- **THEN** that listing renders the chosen columns and omits the rest

#### Scenario: An unrecognised choice falls back

- **WHEN** a listing is rendered for a browser whose recorded preference names an unknown listing or
  column, or is malformed or oversized
- **THEN** the declared default is rendered and no error is raised

#### Scenario: The control needs no scripting

- **WHEN** the column control is submitted with JavaScript disabled
- **THEN** the choice is recorded and the reader is returned to the listing they chose it from,
  filter and position intact

#### Scenario: Resetting restores the default

- **WHEN** a reader resets a listing's columns
- **THEN** the listing renders the declared default set again

### Requirement: Choosing listing columns never puts a row or a value out of reach

Each listing SHALL always render the column naming its rows, whatever is chosen, so no choice can
produce a listing with nothing in it or put a record out of reach. Hiding a column is a display
choice and not a permission: every hidden value SHALL remain reachable on the record's own screen.

#### Scenario: Nothing is put out of reach

- **WHEN** a column is hidden from a listing
- **THEN** the listing still names each row, and the hidden value is still shown on the record's own
  screen

### Requirement: A listing's column choice is kept per browser, not per tenant

The column choice SHALL be recorded per browser rather than in tenant data, so two people working in
one tenant may read a listing differently, and SHALL survive later requests from that browser.

#### Scenario: The choice survives the next request

- **WHEN** the same browser requests the listing again
- **THEN** it is rendered with the chosen set rather than with the default

#### Scenario: The choice belongs to the browser

- **WHEN** a second browser requests the same listing
- **THEN** it is rendered with the default set, unaffected by the first browser's choice

### Requirement: An actor is shown by name rather than by identifier

The browser interface SHALL name the actor behind a record -- a creator, an assignee, a comment
byline, an audit row -- by that actor's handle when the directory holds one. Where no handle can be
resolved, it SHALL show a two-word name derived from the identifier itself, so that one identifier
always yields the same name in every process and on every installation.

#### Scenario: A handle is used where one exists

- **WHEN** a screen names an actor that has a handle
- **THEN** the handle is shown rather than the identifier or a generated name

#### Scenario: A name is generated where no handle exists

- **WHEN** a screen names an actor whose handle cannot be resolved
- **THEN** a two-word name derived from the identifier is shown in its place

#### Scenario: The same identifier always reads the same

- **WHEN** one identifier is rendered in two processes or on two installations
- **THEN** both produce the same generated name

#### Scenario: A membership names the person it grants a role to

- **WHEN** the tenant administration screen lists this tenant's members
- **THEN** each row names the actor rather than showing its bare identifier

### Requirement: A generated actor name is a display aid and is resolved within the tenant

A generated name SHALL be a display aid only: the identifier SHALL remain reachable on the element
that carries the name, and machine-readable output SHALL be unchanged, carrying the identifier and
no generated name. Resolving a name SHALL be scoped to the caller's own tenant.

#### Scenario: The identifier stays reachable

- **WHEN** a generated name or a handle is shown for an actor
- **THEN** the identifier it stands for is available on the element, and the record's machine-readable
  form still carries the identifier and gains no name field

#### Scenario: Names do not cross the tenant boundary

- **WHEN** a record names an actor belonging to another tenant
- **THEN** that actor's handle is not disclosed and the generated name is shown instead

### Requirement: Every display preference has exactly one always-reachable control

The browser interface SHALL gather the preferences that apply to every screen into a settings screen
reachable from the sidebar on every signed-in screen. Every control within it SHALL be a plain form
that works with scripting disabled, and the screen SHALL render under every colour scheme on offer,
including the low contrast one.

#### Scenario: The menu is on every screen

- **WHEN** any signed-in screen is rendered
- **THEN** the sidebar carries a link to the settings screen, which gathers the advanced toggle and
  every other preference but the colour scheme

#### Scenario: Every scheme renders it

- **WHEN** the interface is rendered under the system, light, dark, and low contrast schemes
- **THEN** the settings screen renders in each, and the sidebar's control names the scheme the page
  is drawn in

#### Scenario: The menu needs no scripting

- **WHEN** a preference is changed with JavaScript disabled
- **THEN** the change is recorded and the reader is returned to the page they made it on

### Requirement: A listing's own preference is offered beside it and on the settings screen through one cookie

A preference scoped to one listing, such as which columns it shows, SHALL be offered beside that
listing as well as on the settings screen, and both controls SHALL write the same cookie, so the two
can never disagree.

#### Scenario: A listing's columns are chosen in either place

- **WHEN** a reader hides a column from the picker beside the task list
- **THEN** the settings screen's picker for the task list shows that column hidden too

### Requirement: The colour scheme is set only from the sidebar

The colour scheme SHALL be set by a single control in the sidebar, present on every signed-in
screen, and SHALL NOT also be settable from the settings screen, so that one cookie has one control.
That control SHALL be a plain form that works with scripting disabled, SHALL move to the next scheme
on offer each time it is submitted, and SHALL return the reader to the screen they submitted it
from.

#### Scenario: The scheme switch is in the sidebar and nowhere else

- **WHEN** any signed-in screen is rendered, the settings screen among them
- **THEN** the sidebar carries one control that submits the colour scheme
- **AND** the settings screen's own content carries no second control for it

#### Scenario: Submitting the switch moves to the next scheme

- **GIVEN** a browser reading under any scheme on offer
- **WHEN** the sidebar's scheme control is submitted
- **THEN** the next scheme on offer is recorded and the reader is returned to the screen they
  submitted it from
- **AND** submitting it once for each scheme on offer returns the browser to the scheme it started in

### Requirement: The sidebar's colour scheme control states the scheme in force and the next one

The sidebar's colour scheme control SHALL be reachable by keyboard, and its accessible name SHALL
state both the scheme in force and the scheme submitting it moves to, rather than leaving either to
a hover-only attribute. The settings screen SHALL name the scheme in force and say where the control
is.

#### Scenario: The switch says what it is and what it does

- **WHEN** the sidebar's scheme control is rendered
- **THEN** its accessible name states the scheme in force and the scheme submitting it moves to

### Requirement: The project listing offers every project control

The project listing SHALL make every operation on a project reachable from the listing itself: its
name, workflow, colour, icon and description, and the controls to archive and to delete it, in
addition to creating a new one. A reader on the listing SHALL be able to tell that a project can be
changed without first opening it.

#### Scenario: Controls are reachable from the listing

- **WHEN** the project listing is rendered
- **THEN** each row offers the project's settings and the controls to archive and delete it

#### Scenario: Editing from the listing preserves what it did not show

- **WHEN** a project is saved from the listing
- **THEN** every field the form carried is stored and no unshown field is blanked

### Requirement: The project listing's controls are plain forms in an optional column

The project listing's controls SHALL be plain forms, SHALL be governed by the listing's column
preference like any other optional column, and the listing SHALL always name each row whatever is
chosen.

#### Scenario: The controls are an optional column

- **WHEN** the column carrying the controls is switched off
- **THEN** the listing still names each row and links to the project

### Requirement: The task list shows a chosen set of lists

The task list SHALL offer a control selecting which projects it draws from, placed with the listing's
own controls rather than in the settings menu. An installation that has made no choice SHALL show
every list, and a project created after a choice was made SHALL be visible without the choice being
revisited. The choice SHALL be recorded per browser and SHALL survive later requests.

#### Scenario: An untouched installation is unchanged

- **WHEN** the task list is rendered for a browser that has chosen no lists
- **THEN** every project's tasks are shown and nothing is reported as hidden

#### Scenario: A hidden list disappears and stays hidden

- **WHEN** a reader hides a list
- **THEN** its tasks are absent from the task list, the screen says how many lists are hidden, and the
  choice applies to later requests from that browser

#### Scenario: A new list is visible without being chosen

- **WHEN** a project is created after a visibility choice was made
- **THEN** its tasks appear on the task list without the choice being revisited

### Requirement: An explicit project filter overrides the chosen lists and the task list says so

A filter naming a project explicitly SHALL override the chosen set of lists, and the screen SHALL say
which rule produced what it shows rather than presenting a shorter or empty list without explanation.
A recorded value this build does not recognise SHALL fall back to showing every list.

#### Scenario: An explicit filter wins

- **WHEN** the filter names a project that the visibility choice hides
- **THEN** that project's tasks are shown and the screen says the filter overrode the choice

#### Scenario: An empty result is explained

- **WHEN** every list is hidden
- **THEN** the screen says so and offers the way to bring one back

#### Scenario: An unrecognised choice falls back

- **WHEN** the recorded choice is malformed or oversized
- **THEN** every list is shown and no error is raised

### Requirement: The activity feed reads as records, not identifiers

The activity feed SHALL render each entry with the kind of change it records distinguished from the
others, the actor named as any other screen names one, and the subject shown by the reference a
reader recognises where one exists rather than by its identifier. Entries arriving over the event
stream SHALL be rendered in the same shape as entries rendered by the server, so a live entry is
indistinguishable from a reloaded one.

#### Scenario: A change is distinguishable by kind

- **WHEN** the feed renders a creation, an update and a deletion
- **THEN** each is marked so the three do not read identically

#### Scenario: A task is named by its reference

- **WHEN** the feed renders an entry whose subject is a task
- **THEN** the task's reference is shown, with the identifier still reachable

#### Scenario: A live entry matches a rendered one

- **WHEN** an entry arrives over the event stream
- **THEN** it is appended in the same shape the server renders

### Requirement: Every activity row leads to what it is about

Each row of the activity feed SHALL link both the actor that made the change and the record the
change was made to. The actor's name SHALL link to the feed narrowed to that actor, so one person's
trail can be followed from any row.

#### Scenario: An actor leads to that actor's own trail

- **WHEN** a reader follows the actor named on a row
- **THEN** the feed is shown narrowed to that actor, and says so

### Requirement: An activity row's subject leads to the record itself or names its kind

An activity row's subject SHALL link to the record itself where the reader may reach one: a task to
its own screen, a project or workflow to its screen, and a comment or an artifact to its place on
the task it hangs off, addressed so the browser lands on that record rather than on the top of the
page. A subject the feed cannot resolve SHALL name its kind in words and carry no link, never a bare
identifier presented as a destination.

#### Scenario: A comment is reachable from the row about it

- **WHEN** the feed renders an entry whose subject is a comment
- **THEN** the row links to that comment's own place on the task it was left on

#### Scenario: A record inside a closed disclosure is still reached

- **WHEN** a link from the feed addresses a record that the target screen keeps behind a disclosure
  or on a tab not shown
- **THEN** that disclosure is opened or that tab shown, and the record is marked, so the link lands
  somewhere visible

#### Scenario: An unresolvable subject is not a dead link

- **WHEN** the feed renders an entry whose subject it cannot name
- **THEN** the row says what kind of record changed and offers no link

### Requirement: The activity feed is searchable and filterable from its own address

The activity feed SHALL offer a filter over free text, the kind of record changed, the surface the
change arrived from, and the actor that made it. The whole filter SHALL live in the query string, so
a narrowed feed is a link that can be shared, bookmarked and restored, and the control SHALL be a
plain form that submits without scripting.

#### Scenario: A filter is addressable

- **WHEN** a filter is applied to the feed
- **THEN** the address bar carries it, and loading that address again restores the same feed

#### Scenario: The live update respects the filter

- **WHEN** a change arrives over the event stream while the feed is filtered
- **THEN** the re-rendered rows are still narrowed by that filter

### Requirement: Each active activity filter term is shown and an empty match says what was searched

Every term that is narrowing the feed SHALL be shown, each removable on its own without retyping the
rest. A filter that matches nothing SHALL say so and say how much was searched, rather than reading
as an empty tenant.

#### Scenario: An active term can be dropped on its own

- **WHEN** a feed is narrowed by more than one term
- **THEN** each term is shown separately and can be removed without disturbing the others

#### Scenario: Nothing matching is distinguished from nothing recorded

- **WHEN** a search matches no record
- **THEN** the feed says the filter matched nothing and how many records were searched

### Requirement: Primary task attributes are edited without a disclosure

The task detail screen SHALL present a task's primary attributes -- its title, description, priority
and assignee -- in the form that is visible on arrival, and SHALL reserve a disclosure for the
attributes a project defines for itself. Where a screen carries more than one form over the same
task, each SHALL carry the values it does not show, so submitting one never blanks a field held by
another, and a submission refused because the task moved on SHALL be reported with what to do next.

#### Scenario: Priority is visible on arrival

- **WHEN** the task detail screen is rendered for an actor who may change the task
- **THEN** the priority control is in the visible form, not behind a disclosure

#### Scenario: One form does not blank another's fields

- **WHEN** a form on the task screen is submitted
- **THEN** every attribute the screen holds is preserved, whether or not that form showed it

#### Scenario: A refused save says what to do

- **WHEN** a save is refused because the task changed while the page was open
- **THEN** the failure names the conflict and tells the reader to reload and reapply the change

### Requirement: A task row distinguishes its markings by shape

A task row carries several kinds of fact -- the workflow state it is in, its priority, its tags, and
the project it belongs to -- and each kind SHALL be given a form distinct from the others, so the
kind of a marking is readable without depending on its colour. A reader SHALL be able to tell a tag
from a project from a priority at a glance.

#### Scenario: Four kinds of marking are four shapes

- **WHEN** a task row renders its state, its priority, its tags and its project
- **THEN** each kind is rendered in a form that differs from the other three by more than its colour

#### Scenario: A tag and a project lead to the rest of their own

- **WHEN** a reader follows a tag or a project on a row
- **THEN** the task list is shown narrowed to that tag or that project

### Requirement: A task marking takes the same form on every screen

The same fact SHALL take the same form on every screen that shows it, so a state or a priority does
not change shape between the list, the board and a task's own screen.

#### Scenario: A marking reads the same everywhere

- **WHEN** the same workflow state is rendered on the task list, on the board and on a task's screen
- **THEN** it takes the same form on all three

### Requirement: The task list says how much is on it

The task list SHALL summarise the page it is showing: how much is outstanding, how much is finished,
how many projects the rows come from, and how much is blocked or held by an agent. The project count
SHALL be the number of distinct projects the rows actually belong to, so a reader can tell one
project's work from everything at once.

#### Scenario: The summary counts projects

- **WHEN** the task list renders rows belonging to more than one project
- **THEN** the summary says how many distinct projects they come from

### Requirement: The user listing shows and preserves each account's role

The user administration screen SHALL show, for every account it lists, the role that account's
membership of this tenant grants, and the edit control SHALL open with that role already selected.
Saving any other change to an account SHALL NOT alter its role unless the role was itself changed.
An account the screen cannot resolve a membership for SHALL be shown as having none, and its control
SHALL offer to leave the role unchanged rather than proposing one.

#### Scenario: The role is on the row

- **WHEN** the user listing renders an account whose membership grants it a role
- **THEN** that role is shown on the row

#### Scenario: Editing another field does not change the role

- **WHEN** an administrator opens an account, changes only whether it is disabled, and saves
- **THEN** the account keeps the role it had

#### Scenario: An account with no membership proposes no role

- **WHEN** the listing renders an account it can resolve no membership for
- **THEN** the control offers to leave the role unchanged and selects no role

### Requirement: The connection screen offers the action it describes

The live connection screen SHALL offer, for every connection it lists, the action that ends it, and
SHALL mark a connection held by the reader's own account. It SHALL state that ending closes the
socket without revoking anything, and SHALL link to the screens that do revoke -- API tokens, SSH
keys, and the account itself -- so an operator who meant to stop somebody is not left with an action
that looks like it did that and did not.

#### Scenario: Every listed connection can be ended

- **WHEN** the connection screen lists a live connection
- **THEN** it offers the control that ends that connection

#### Scenario: Ending points at revocation

- **WHEN** the connection screen is rendered
- **THEN** it says that ending is not revocation and links to the screens that revoke a credential

#### Scenario: The reader's own connection is marked

- **WHEN** the screen lists a connection held by the account the reader is signed in as
- **THEN** that connection is marked as theirs

### Requirement: Behaviour survives a boosted navigation

The browser interface swaps the page's contents on an internal navigation rather than loading a new
document, so the scripts in the document head run once per session. Every behaviour those scripts
provide SHALL therefore be bound in a way that survives such a swap: either delegated at the
document and resolved from the event, or re-established when the swap completes.

#### Scenario: Dragging works on a board reached by clicking

- **WHEN** a reader navigates to a project board from another screen by following a link
- **THEN** a card dragged to a legal column is moved, exactly as it is for a board reached by its own URL

#### Scenario: The live feed follows the reader

- **WHEN** a reader navigates to the activity feed from another screen by following a link
- **THEN** the feed updates as events arrive

### Requirement: A re-established script behaviour leaves no duplicate connection or handler

A behaviour SHALL NOT depend on an element that existed when the script first ran, and
re-establishing one SHALL NOT leave a second connection or a second handler behind.

#### Scenario: Re-establishing opens nothing twice

- **WHEN** a reader navigates between two screens repeatedly
- **THEN** no more than one event-stream connection is held at a time

### Requirement: An emptied form field clears the value it holds

A browser form field that a reader empties and saves SHALL clear the stored value. A field the submission
does not name at all SHALL leave the stored value unchanged.

This SHALL hold for a task's custom fields and for an account's display name, which are the two values the
browser could previously set and never remove.

#### Scenario: Emptying a custom field clears it

- **GIVEN** a task whose custom field holds a value
- **WHEN** the reader empties that field's input and saves
- **THEN** the field is shown empty on the task afterwards

#### Scenario: Emptying a display name removes it

- **GIVEN** an account with a display name
- **WHEN** an administrator empties the name input and saves
- **THEN** the account has no display name afterwards

### Requirement: Whether a form field is cleared depends on its key being submitted, not on its value

The distinction between emptying a field and not naming it SHALL be drawn on whether the submission
carries the field's key, not on whether the value it carries is empty, so that a submission naming some
of a record's fields changes only the fields it names.

#### Scenario: A save naming one field leaves the others alone

- **GIVEN** a task carrying values in two custom fields
- **WHEN** a submission names only the first of them
- **THEN** the first holds the submitted value and the second is unchanged

#### Scenario: A save carrying no custom field inputs changes none of them

- **GIVEN** a task whose custom field holds a value
- **WHEN** the reader saves the task's title from a form carrying no custom field input
- **THEN** the custom field still holds its value

#### Scenario: A save naming no display name leaves it alone

- **GIVEN** an account with a display name
- **WHEN** a submission changes the account's role and names no display name
- **THEN** the account keeps its display name

### Requirement: Clearing a required form field is refused and leaves the value as it was

Where clearing a value would leave a required field empty, the save SHALL be refused with a validation
error and the stored value SHALL be left as it was. A refused save SHALL NOT report success.

#### Scenario: Emptying a required field is refused

- **GIVEN** a task whose required custom field holds a value
- **WHEN** the reader empties that field's input and saves
- **THEN** the save is refused as invalid
- **AND** the stored value is unchanged

### Requirement: Putting a project away removes that project and nothing else

The task listing's visibility control SHALL remove from the listing exactly the projects the reader has
put away, whatever the number of projects in the tenant.

The listing SHALL be narrowed by excluding the projects put away, rather than by naming the projects to
show, so that the rows shown do not depend on any listing of projects having been complete.

#### Scenario: Hiding one project of many keeps the rest

- **GIVEN** a tenant with more projects than one page of the project listing returns
- **AND** a task in a project past that first page
- **WHEN** the reader puts a single other project away
- **THEN** that task is still listed

### Requirement: The task listing's visibility control offers every project of the tenant

The task listing's visibility control SHALL offer a box for every project in the tenant, so that a
project can be put away and brought back whatever its position in the project listing.

#### Scenario: The control names every project

- **GIVEN** a tenant with more projects than one page of the project listing returns
- **WHEN** the task listing is rendered
- **THEN** the visibility control offers a box for every project in the tenant

### Requirement: A browser screen reporting a whole-tenant figure counts the whole tenant

Where a browser screen presents a count as the number of records of a kind in the tenant, that count SHALL
be taken over all of them rather than over one page of a listing.

Where the count cannot be established, the screen SHALL say so in place of the figure rather than show a
number that is short.

A browser screen that resolves a record by identifier out of a listing SHALL find any record in the
tenant, not only one on the listing's first page.

#### Scenario: The tenant diagram counts every project

- **GIVEN** a tenant with more projects than one page of the project listing returns
- **WHEN** the tenant diagram is rendered
- **THEN** its project row shows the number of projects in the tenant

#### Scenario: A task in a distant project can be finished

- **GIVEN** a task in a project past the first page of the project listing
- **WHEN** the reader ticks it done
- **THEN** the task is finished

### Requirement: The delivery log offers its next page

The webhook delivery log SHALL render the position control every keyset listing on this surface renders,
offering the following page whenever one exists.

#### Scenario: An older delivery is reachable

- **GIVEN** more deliveries than one page of the log shows
- **WHEN** the log is rendered
- **THEN** it offers a link to the next page
- **AND** that page shows deliveries the first page did not

### Requirement: The task screen draws one selection as a list, a board or lanes

The task screen SHALL offer a control that switches between a list of rows, a board of columns and
lanes of projects, and SHALL remember the choice per browser so that it survives subsequent requests.

The list SHALL be the view an installation shows before any choice is made, and a stored value that
this build does not recognise SHALL read as the list rather than failing the screen.

#### Scenario: The choice survives the next request

- **GIVEN** a reader on the task screen
- **WHEN** they switch the view to the board or to lanes
- **AND** they request the task screen again
- **THEN** the chosen view is drawn without the switch being pressed a second time

#### Scenario: An untouched installation shows the list

- **GIVEN** a browser that has never used the switch
- **WHEN** it opens the task screen
- **THEN** the list is drawn

#### Scenario: An unrecognised stored choice shows the list

- **GIVEN** a browser carrying a view preference this build does not implement
- **WHEN** it opens the task screen
- **THEN** the list is drawn rather than an error

### Requirement: Switching the task view never changes which tasks are selected

The task screen's view choice SHALL change only how the selected tasks are drawn. The filter
expression, the deadline window, the sort, the page size, the page position and the project
visibility choice SHALL be resolved once and SHALL apply identically to every view, so that switching
never changes which tasks are on screen beyond the list's open-work default. Every view SHALL be
walked by the same pager.

#### Scenario: Switching keeps the filter

- **GIVEN** a task screen filtered to one tag
- **WHEN** the reader switches to the board
- **THEN** the board holds exactly the tasks the filtered list held
- **AND** the filter expression is still in the filter box

#### Scenario: Switching keeps the project visibility choice

- **GIVEN** a reader who has put one project away
- **WHEN** they switch to the board
- **THEN** no task of the project they put away appears on the board

### Requirement: Board columns merge only across workflows that are the same state machine

The task board SHALL draw one set of columns from the workflow the selected projects share, and SHALL
treat two projects' workflows as the same workflow only when their definitions agree on every element
a board draws or a move depends on: the states in their declared order, including each state's key,
label, terminal flag and category, and the transitions in their declared order, including each
transition's source, target, required scope and whether it requires a comment.

#### Scenario: A differing state category prevents a merge

- **GIVEN** two projects whose workflows declare the same state keys and transitions but give one state a different category
- **WHEN** the reader draws the board over both
- **THEN** no merged board is drawn

### Requirement: Board workflow agreement is structural, never by name or identifier

Agreement SHALL NOT be decided by the workflow's name, key or stored identifier. Two projects whose
workflows are stored separately but agree structurally SHALL merge into one set of columns. Two
projects whose workflows share a name but differ in any compared element SHALL NOT merge.

#### Scenario: Two projects with identical workflows share one board

- **GIVEN** two projects whose workflows are stored separately and describe the same states and transitions
- **WHEN** the reader draws the board over both
- **THEN** one set of columns is drawn, one column per state of the shared workflow
- **AND** the tasks of both projects appear in the column matching their own status

#### Scenario: Two same-named workflows that differ do not merge

- **GIVEN** two projects each running a workflow named `default`, one of which has a state the other does not
- **WHEN** the reader draws the board over both
- **THEN** no merged board is drawn

### Requirement: A board card is never offered a move its own project's workflow forbids

Every move a board card offers SHALL be derived from the workflow of that card's own project, never
from the definition the columns were drawn from.

A control for a transition the card's own workflow does not allow SHALL NOT be rendered, so that the
reader is refused before acting rather than by the service after acting. The service refusing such a
move SHALL remain the fallback and SHALL NOT be the only protection.

#### Scenario: An unreachable state is not offered

- **GIVEN** a card whose project's workflow permits no move from its current state to a given state
- **WHEN** the board is drawn
- **THEN** the card's move control offers no option for that state

#### Scenario: A card offers only its own project's routes

- **GIVEN** a merged board holding cards from two projects
- **WHEN** the board is drawn
- **THEN** each card's move control lists exactly the routes its own project's workflow allows from that card's state

### Requirement: A board drop applies one transition and a longer route names its stops

A drop onto a column SHALL apply a single transition, since a drop names a destination and nothing
else. A route passing through other states SHALL be offered only by a control that names the states
it passes through before it is applied.

#### Scenario: A multi-hop route names its intermediate states

- **GIVEN** a card whose workflow reaches a state only through another state
- **WHEN** that route is offered
- **THEN** the option names the states passed through and how many steps it takes

### Requirement: A board that cannot be drawn honestly is refused with its reason

When the projects the listing selects do not share one workflow, the task screen SHALL NOT draw a
board of a subset of them. It SHALL draw the list instead, and SHALL state that the board was
refused, naming each distinct workflow and the projects running it.

Each named group SHALL be reachable in one action as a listing narrowed to exactly its own projects,
carrying the reader's filter, deadline window and sort.

#### Scenario: Disagreeing workflows refuse the board and say which projects disagree

- **GIVEN** three selected projects, two sharing a workflow and one differing
- **WHEN** the reader asks for the board
- **THEN** the list is drawn
- **AND** the screen states that the projects do not share a workflow
- **AND** each distinct workflow is named with the projects that run it

### Requirement: A refused board leaves the reader's view preference unchanged

The reader's view preference SHALL be left as it is by the refusal, so that narrowing the selection
draws the board without the switch being pressed again.

#### Scenario: Narrowing to one group draws its board

- **GIVEN** a refused board naming two groups of projects
- **WHEN** the reader follows the entry for one group
- **THEN** the board of that group's shared workflow is drawn
- **AND** the switch did not have to be pressed again

### Requirement: A board refusal tells apart workflows that share a name

Where two named groups run workflows carrying the same name, the refusal SHALL distinguish them, so
that two entries reading alike cannot be the whole explanation.

#### Scenario: Two workflows of the same name are named apart

- **GIVEN** a refusal naming two groups whose workflows are both called `Default`
- **WHEN** the notice is drawn
- **THEN** the two entries are distinguishable from one another

### Requirement: A board reports a task whose status matches none of its columns

A task on the page whose status matches none of the drawn columns SHALL be reported on the screen
rather than omitted from it.

#### Scenario: A task in a state the workflow no longer has is reported

- **GIVEN** a board whose page holds a task whose status is in none of the drawn columns
- **WHEN** the board is drawn
- **THEN** the screen names that task as unplaced rather than leaving it off the board silently

### Requirement: A refused submission is answered by the form, not by an error page

Where a browser submission is refused for a reason the reader can act on by changing what they submitted —
a missing or invalid value, or a conflict with something that already exists — the screen SHALL re-render
the form it came from, carrying every value the submission held and the refusal's message beside the
control it concerns.

The message SHALL be associated with its control for assistive technology, and the control SHALL be marked
as invalid.

#### Scenario: A conflicting submission comes back

- **GIVEN** a tenant already holding a live token named `ci`
- **WHEN** a submission naming `ci` is sent
- **THEN** the token screen is rendered again with the name still in its input
- **AND** the message says the name already exists

### Requirement: A refused submission is checked by the server, not by the browser

Re-rendering the form SHALL be the response to the ordinary form submission, so that it holds with
scripting turned off. A client-side validation attribute on a control MAY be present as a convenience,
and SHALL NOT be the thing performing the check.

#### Scenario: A submission with a missing required value comes back

- **GIVEN** the API token screen's issue form
- **WHEN** a submission naming a token name but selecting no scope is sent
- **THEN** the token screen is rendered again rather than the error screen
- **AND** the name input still holds the submitted name
- **AND** the message explaining the refusal appears with the scope control

### Requirement: A refusal the reader cannot fix by editing still reaches the error screen

A refusal the reader cannot act on by editing the submission — an authentication or authorization failure,
or an internal fault — SHALL continue to be answered by the error screen, which is the only surface that
can explain it.

#### Scenario: An unauthorized submission still reaches the error screen

- **WHEN** a submission is refused because the caller lacks the required scope
- **THEN** the error screen is rendered

### Requirement: A tenant administrator can see and revoke another actor's token in the browser

The API token screen SHALL list every token of the tenant to a reader holding the tenant administration
scope, and SHALL list only the reader's own tokens to any other reader.

Where the listing spans more than the reader's own tokens, each row SHALL say which actor holds it, by that
actor's handle where the directory holds one. A row that is the reader's own SHALL be distinguishable from
a row that is not.

#### Scenario: A reader without the administration scope sees only their own

- **GIVEN** a reader holding the token administration scope and not the tenant administration scope
- **AND** a token held by another actor of the same tenant
- **WHEN** the token screen is read
- **THEN** the other actor's token is not listed
- **AND** no revocation control on the screen names that token
- **AND** the listing carries no owner column

#### Scenario: An administrator sees the tenant's tokens with the owner named

- **GIVEN** a reader holding the tenant administration scope
- **AND** a token held by another actor of the same tenant
- **WHEN** the token screen is read
- **THEN** that token is listed
- **AND** its row names the actor holding it
- **AND** a revocation control is offered for it

### Requirement: A token revocation control is offered only for a token the reader is shown

A revocation control SHALL be offered only for a token the reader is shown, and the control for a token
held by another actor SHALL say whose it is before it is used. No control SHALL be offered for a token the
reader may not see.

#### Scenario: The control for somebody else's token says whose it is

- **GIVEN** an administrator reading a token held by another actor
- **WHEN** the revocation dialog for that row is opened
- **THEN** it says the token is not the reader's own
- **AND** the button naming the token also names its owner

#### Scenario: An administrator revokes another actor's token from the browser

- **GIVEN** an administrator and a live token held by another actor
- **WHEN** the administrator submits that row's revocation
- **THEN** the token is revoked
- **AND** the row comes back marked revoked and offering no revocation control

### Requirement: A generated webhook signing secret is shown once

Where tix generates a signing secret for a delivery endpoint, because the operator registered one without
supplying a secret, the webhook screen SHALL present that value once, immediately after the registration,
and SHALL NOT present it on any later render.

A secret the operator supplied SHALL NOT be presented back to them.

#### Scenario: A generated secret is gone on the next render

- **GIVEN** a generated secret that has been presented once
- **WHEN** the webhook screen is read again
- **THEN** no secret region is rendered
- **AND** the value does not appear on the page

#### Scenario: A supplied secret is not echoed back

- **WHEN** an endpoint is registered with a signing secret the operator supplied
- **THEN** no secret region is rendered
- **AND** the supplied value does not appear on the page
- **AND** the endpoint is listed

### Requirement: A generated signing secret uses the one-time secret region

A generated signing secret SHALL be presented in the same one-time secret region the token screen uses
for a freshly issued token: a region of its own rather than the interface's transient confirmation
message, readable in full without being cropped, selectable, and accompanied by a copy control. The
value SHALL remain readable in full when no scripting is available.

#### Scenario: A generated secret appears once

- **WHEN** an endpoint is registered with the signing secret field left blank
- **THEN** the webhook screen presents the generated secret in its one-time secret region
- **AND** a copy control points at it

### Requirement: The signing secret field's help states what happens in each case

The field's own help SHALL state what happens in each case: when tix generates a secret, when the operator
supplies one, and when an existing endpoint is saved with the field left blank.

#### Scenario: The field's help is true in both cases

- **WHEN** the signing secret field's help is read
- **THEN** it says a generated secret is shown once
- **AND** it says a secret the operator supplies is not shown

### Requirement: One presentation for a value the server will not disclose again

A value the server will never disclose again SHALL be presented through one shared region, used by every
screen that has such a value, rather than through markup copied per screen.

A freshly issued API token and a generated webhook signing secret SHALL both be presented through it.

#### Scenario: Both screens use one region

- **WHEN** the token screen and the webhook screen each present a one-time secret
- **THEN** both are rendered by the same shared region

### Requirement: The one-time secret region is whole, selectable and copyable without scripting

The shared region SHALL be distinct from the interface's transient confirmation message, SHALL be
readable in full without being cropped, SHALL be selectable, and SHALL carry a copy control driven by
the interface's one generic copy affordance. It SHALL remain readable in full when no scripting is
available.

#### Scenario: The issued value is not presented as a confirmation message

- **WHEN** a token has just been issued
- **THEN** its value is in its own region rather than in the transient confirmation message
- **AND** a copy control points at it

### Requirement: The settings screen owns every per-browser display preference

The settings screen SHALL carry a control for every per-browser display preference this interface
keeps in a cookie, with no preference editable only from elsewhere, except the colour scheme, whose
one control is the sidebar's.

The set of preferences SHALL be declared in the code, and both the settings screen's coverage of it
and the documentation table that names it SHALL be decided against that declaration rather than
maintained by hand.

#### Scenario: Every preference has a control on the settings screen

- **GIVEN** the set of per-browser preferences this build declares
- **WHEN** a signed-in reader opens the settings screen
- **THEN** the screen carries a posting form for each one

#### Scenario: A preference added without a control fails

- **GIVEN** a per-browser preference declared in the code
- **WHEN** the settings screen carries no control that submits to its route
- **THEN** the guard over the declared set fails

#### Scenario: Every preference is documented

- **GIVEN** the set of per-browser preferences this build declares
- **WHEN** the preferences table in the web interface documentation is read
- **THEN** every declared preference has a row, and every row names a declared preference

### Requirement: Browser-stored layout state is not a display preference

Layout state a script keeps in the browser's own storage rather than in a cookie SHALL NOT count as
a display preference: the sidebar's width and whether it is collapsed, and whether the lanes view
shows completed tasks. Such state SHALL be changed where it is drawn, SHALL NOT be sent to the
server, and SHALL be named in the web interface documentation.

#### Scenario: Layout state stays in the browser

- **WHEN** a reader collapses the sidebar or turns on completed tasks in the lanes view
- **THEN** the choice is kept in the browser's own storage, no request carries it, and the settings
  screen offers no control for it

### Requirement: Every settings preference control is a plain, described form

Every preference control on the settings screen SHALL be a form the browser submits on its own, with
a method, an action and a submit button, and SHALL carry no scripted handler, so that it stores its
value with scripting turned off.

Every such control SHALL be accompanied by a description of what the preference does, associated
with the control so that assistive technology announces it, and written so that a reader who does not
already know the preference's name can tell what it changes.

#### Scenario: Saving a preference needs no scripting

- **GIVEN** a browser with scripting unavailable
- **WHEN** it submits any preference form on the settings screen as a plain form post
- **THEN** the value is stored
- **AND** the browser is returned to the screen the form was submitted from

### Requirement: A preference editable in two places is one control over one value

A preference whose control appears both on the settings screen and on the screen that uses it SHALL
be rendered from one template against state built by one constructor, and SHALL be resolved by one
reader and stored by one handler.

Changing such a preference on either screen SHALL be reflected in the control on the other, as that
control renders, and not merely in the stored value.

#### Scenario: A change made on settings reaches the screen that uses it

- **GIVEN** a reader who switches the task view to the board on the settings screen
- **WHEN** they open the task screen
- **THEN** the board is drawn
- **AND** the task screen's own switch shows the board as the current position

#### Scenario: A change made in context is shown on settings

- **GIVEN** a reader who puts one project away using the control on the task screen
- **WHEN** they open the settings screen
- **THEN** that project's checkbox in the settings control is unticked
- **AND** every project they kept is ticked

#### Scenario: A column change made on a listing is shown on settings

- **GIVEN** a reader who hides one optional column using the picker beside a listing
- **WHEN** they open the settings screen
- **THEN** that column's checkbox in that listing's section is unticked

#### Scenario: A column change made on settings reaches the listing

- **GIVEN** a reader who hides one optional column of a listing from the settings screen
- **WHEN** they open that listing
- **THEN** the hidden column is absent from the table's header row
- **AND** every column they kept is present

### Requirement: Column preferences are offered per listing

The settings screen SHALL offer a column picker for every listing whose columns can be chosen, and
for no listing that has none, each picker naming the listing it governs.

Each picker SHALL offer the same columns, in the same order, with the same current state, as the
picker that listing carries beside itself, and SHALL offer a reset that returns that listing to its
declared defaults.

#### Scenario: Each configurable listing has its own section

- **GIVEN** the listings this build declares columns for
- **WHEN** a reader opens the settings screen
- **THEN** each listing has exactly one picker, named after that listing

#### Scenario: A listing with no configurable columns is not offered one

- **GIVEN** a screen that declares no optional columns
- **WHEN** a reader opens the settings screen
- **THEN** no picker names it

### Requirement: The project visibility control names its projects and accounts for keys that name none

The project visibility control SHALL name each project as well as identify it by key, so that it can
be used on a screen that carries no project listing.

#### Scenario: A project is named, not only keyed

- **GIVEN** a tenant with a project whose key and name differ
- **WHEN** a reader opens the visibility control on either screen
- **THEN** the project's name and its key are both shown

### Requirement: A stored visibility key naming no project is named, ignored and discarded

Where the stored preference holds a key that no project on the control's own listing answers to, the
control SHALL name that key and SHALL say that no project answers to it, and SHALL not offer a
checkbox for it. Submitting the control SHALL discard such keys.

A stored key that names no project SHALL exclude nothing from the task listing, and SHALL not prevent
any screen from rendering.

#### Scenario: A key naming a deleted project does not break the screen

- **GIVEN** a browser whose stored preference hides a key no project answers to
- **WHEN** the reader opens the task screen or the settings screen
- **THEN** the screen renders
- **AND** the key is named as one no project answers to
- **AND** no checkbox is offered for it

#### Scenario: A key naming a deleted project is forgotten on the next submission

- **GIVEN** a browser whose stored preference hides a key no project answers to
- **WHEN** the reader applies the visibility control
- **THEN** the key is no longer held
- **AND** the choice they applied is

#### Scenario: A key naming a deleted project excludes nothing

- **GIVEN** a browser whose stored preference hides a key no project answers to
- **WHEN** the task listing is answered
- **THEN** every task of every project the reader kept is on it

### Requirement: A screen never presents part of a listing as the whole of it

Where the browser interface renders a figure or a control built from a keyset-paginated listing, it
SHALL either read that listing to its end or state, on the screen, that what it shows is not the
whole set.

A statement of incompleteness SHALL appear with the control or figure it qualifies, and SHALL be
written in the register the rest of the interface uses for such notes.

#### Scenario: A listing that reached the end says nothing

- **GIVEN** a tenant whose projects and actors are both fewer than the bounded walks cover
- **WHEN** a signed-in reader opens the task screen, the settings screen, the statistics screen, the
  token screen or the tenant screen
- **THEN** no control on those screens claims to be short
- **AND** every diagram figure whose walk completed is rendered as a number

#### Scenario: A walk that stopped at its bound is said on the screen

- **GIVEN** a tenant with more projects than the project walk covers
- **WHEN** a reader opens the task screen or the settings screen
- **THEN** the project visibility control states that the list it offers is not the whole set

#### Scenario: One caveat for one control rendered twice

- **GIVEN** the project visibility control, which the task screen and the settings screen both render
- **WHEN** the walk behind its choices stops before the end
- **THEN** both screens state it, from the control's own state rather than from either screen's

### Requirement: A figure from a walk stopped short prints that it is incomplete

A figure whose listing stopped before the end SHALL NOT be rendered as a number, since the number
would be a floor presented as a total; the screen SHALL print in its place that the figure is
incomplete, distinguishably from a listing that failed to be read at all.

#### Scenario: A figure whose walk stopped prints no total

- **GIVEN** a tenant with more projects than the project walk covers
- **WHEN** a reader opens the tenant screen
- **THEN** the Projects row of the diagram prints that the count is incomplete, in the figure's place

### Requirement: The tenant diagram's token figure means what the token screen shows

The count on the tenant diagram's API tokens row SHALL be the number of tokens the token screen lists
for the same reader, and SHALL be produced from the same listing that screen renders.

#### Scenario: An administrator's figure is the tenant's

- **GIVEN** a reader holding `tenant:admin` in a tenant where another actor holds a token
- **WHEN** they read the API tokens figure on the tenant screen and then open the token screen
- **THEN** the figure equals the number of rows the token screen lists
- **AND** the figure is not the count of the reader's own tokens alone

#### Scenario: A reader who cannot list tokens sees no figure

- **GIVEN** a reader who may read the tenant screen but may not list API tokens
- **WHEN** they read the API tokens row
- **THEN** the row states that the figure is unavailable rather than printing one

### Requirement: The token listing reaches every actor of the tenant

The token screen shown to a reader holding `tenant:admin` SHALL list the tokens of every actor the
actor directory holds, walked through the listing's cursor rather than read as a single page.

Where the walk reaches its bound before the end of the directory, the screen SHALL state that the
rows are not every token of the tenant.

#### Scenario: A token held by an actor past the first directory page

- **GIVEN** a tenant with more actors than one listing page holds, and a token belonging to an actor
  past that page
- **WHEN** a reader holding `tenant:admin` opens the token screen
- **THEN** that token is listed, with a revocation control of its own

#### Scenario: A directory longer than the walk

- **GIVEN** a tenant whose actor directory is longer than the bounded walk covers
- **WHEN** a reader holding `tenant:admin` opens the token screen
- **THEN** the screen states that the rows are not every token of the tenant

### Requirement: The statistics project picker offers every project

The project filter on the statistics screen SHALL offer every project of the tenant, read through the
listing's cursor rather than as a single page.

A failure to read the project listing SHALL leave the screen without a picker rather than without
figures, and the screen SHALL state that the picker does not list every project whenever it is
incomplete, whether because the read failed or because the walk stopped at its bound.

#### Scenario: Filtering by a project past the first listing page

- **GIVEN** a tenant with more projects than one default listing page holds
- **WHEN** a reader opens the statistics screen
- **THEN** the project picker offers every project of the tenant
- **AND** choosing one past the first page comes back as the selected option

#### Scenario: A picker that could not be read in full

- **GIVEN** a tenant whose project listing cannot be read to its end
- **WHEN** a reader opens the statistics screen
- **THEN** the figures are rendered
- **AND** the screen states that the picker does not list every project of the tenant

### Requirement: The position control counts the page it is on

The position control a keyset listing renders SHALL state which page of the walk the reader is on,
counting from one, and that number SHALL keep counting for as long as the walk continues. It SHALL
NOT stall at the bound of any memory the control keeps in its own links.

#### Scenario: Walking past the bound of the control's memory

- **GIVEN** a listing with more pages than the control's cursor memory holds
- **WHEN** a reader follows the control forward past that bound
- **THEN** each page states a number one higher than the page before it

#### Scenario: Walking back past the bound

- **WHEN** the reader follows the control back from a page past that bound
- **THEN** each page states a number one lower than the page before it, for every page the control
  can still reach

### Requirement: The position control's page number is carried in its own links

The number SHALL be derived from what the control's own links carry. Where that memory is bounded and
has discarded the earliest of it, the number SHALL be carried alongside rather than recomputed from
what is left.

A link short of that bound SHALL carry no page number of its own, since its history already counts
it.

#### Scenario: An early page's link carries no page number

- **WHEN** a reader follows the control forward or back from a page within the bound of its memory
- **THEN** the resulting address carries no page number parameter

### Requirement: A page number from the address bar never undercuts the link's history

A number arriving from the address bar SHALL NOT be believed below what the link's own history
already proves, SHALL be bounded, and SHALL reach no query. The worst a value a reader typed can cost
is a wrong label on the control.

#### Scenario: A number below what the history proves is ignored

- **GIVEN** a page whose link carries a history of forty positions
- **WHEN** the address bar names a page number lower than that history accounts for, or a value that
  is not a number at all
- **THEN** the control states the number the history itself counts

### Requirement: Rendered screens forbid storage

Every screen the browser interface renders SHALL carry `Cache-Control: no-store`.
This SHALL apply to successful renders, to the error screen, and to a form
redrawn after a refusal, so that no rendered response reaches a browser cache,
a shared cache, or the back-forward cache.

Static assets are excluded: they carry a strong validator and an explicit
freshness lifetime instead, because their URLs do not change between releases.

#### Scenario: An authenticated screen

- **WHEN** a signed-in reader loads the task list, a board, a project, the
  activity screen or any administration screen
- **THEN** the response carries `Cache-Control: no-store`

#### Scenario: A refused form

- **WHEN** a form is submitted and redrawn with a validation refusal
- **THEN** the redrawn screen carries `Cache-Control: no-store`, so the values
  the user typed are not written to a cache

#### Scenario: The error screen

- **WHEN** a request resolves to the error screen
- **THEN** that response carries `Cache-Control: no-store`

#### Scenario: Assets are not affected

- **WHEN** a stylesheet, script or image under the asset path is served
- **THEN** it carries its strong ETag and freshness lifetime, and does not
  carry `no-store`

### Requirement: A second step in the browser sign-in

When a tenant requires a second factor for the browser, the sign-in flow SHALL
present a second step after the password is accepted, taking a TOTP or recovery
code, and SHALL establish a session only once the code is accepted.

The second step SHALL work without JavaScript, as every other form in the
browser interface does, and SHALL preserve the `next` destination across both
steps so the reader lands where they were going.

#### Scenario: Password then code

- **WHEN** a user signs in to a tenant requiring a factor for the browser
- **THEN** the password form is followed by a code form, and the session begins only after the code is accepted

#### Scenario: The destination survives both steps

- **WHEN** a user is bounced to sign-in from a deep link and completes both steps
- **THEN** they arrive at the originally requested page

#### Scenario: An incomplete first step grants nothing

- **WHEN** a user passes the password step and abandons the code step
- **THEN** no session exists and no tenant data has been rendered

### Requirement: Enrolment and recovery codes in the browser

The settings screen SHALL offer enrolment, showing the `otpauth://` URI and a
scannable code, and SHALL display the recovery codes exactly once on
confirmation, with a means of copying them.

It SHALL report remaining recovery codes, offer regeneration, and offer removal
of one's own enrolment where the tenant does not require a factor.

#### Scenario: Codes are displayed once

- **WHEN** a user confirms enrolment in the browser
- **THEN** ten recovery codes are shown with a means of copying them, and revisiting settings does not show them again

#### Scenario: Removal is refused where a factor is required

- **WHEN** an enrolled user tries to remove their enrolment in a tenant that requires a factor on the surface they are using
- **THEN** the removal is refused with an explanation

### Requirement: Administrative view of enrolment

A tenant administrator SHALL be able to see which users hold an enrolment and to
clear one, and SHALL NOT be shown any secret or any recovery code.

#### Scenario: The administrator sees state, not secrets

- **WHEN** an administrator views the user list
- **THEN** each user's enrolment state is shown, and no secret or recovery code appears anywhere on the screen

### Requirement: A live screen recovers its connection, and says when it has not

Every screen that updates from the event stream, the activity feed included,
SHALL share one connection mechanism.

A connection that drops SHALL be reopened, on a delay that grows with each
failed attempt, is bounded, and is jittered so that a server returning does
not meet every open browser at once. A successful connection SHALL reset the
delay. A hidden page SHALL NOT retry; returning to it SHALL reconnect at once
and re-read what it missed.

#### Scenario: A dropped connection is reopened

- **GIVEN** a screen receiving events
- **WHEN** the connection drops
- **THEN** it is reopened after a bounded, growing, jittered delay, and updates resume

#### Scenario: A sleeping laptop does not storm the server

- **GIVEN** a page hidden while its connection is down
- **WHEN** it is hidden
- **THEN** it does not retry, and on becoming visible it reconnects at once and re-reads

### Requirement: A resubscription resumes from the last sequence the screen saw

A resubscription SHALL carry the sequence of the last event the screen saw, so
the server replays the gap. Where that cursor is no longer retained, the
screen SHALL subscribe afresh and re-fetch its fragment whole rather than
leaving a gap nothing fills.

#### Scenario: Nothing is lost across the gap

- **GIVEN** a screen that has seen events up to a known sequence
- **WHEN** it reconnects
- **THEN** it resubscribes from that sequence and the changes made while it was down are shown

#### Scenario: A cursor the server cannot honour

- **GIVEN** a screen resuming from a sequence that has been retained out of the log
- **WHEN** the server refuses the cursor
- **THEN** the screen subscribes from the present and re-fetches its fragment, rather than failing silently

### Requirement: A screen with no event connection polls and warns that it may be stale

Where no connection can be established at all, the screen SHALL poll its
fragment on a slow interval, and SHALL stop polling as soon as a connection
succeeds.

While a screen is not receiving events it SHALL say so on screen, naming the
consequence: that what is shown may be out of date.

#### Scenario: No socket available

- **GIVEN** a reader behind an intermediary that will not carry a WebSocket
- **WHEN** the screen cannot open one
- **THEN** it polls its fragment on a slow interval and stops as soon as a socket succeeds

#### Scenario: The reader is told the view may be stale

- **WHEN** a screen is not receiving events
- **THEN** it shows a notice saying the view may be out of date, and removes it when the stream is live again

### Requirement: An update is drawn at once and a burst is grouped

The first event in a quiet period SHALL be rendered without waiting. Further
events arriving within the screen's coalescing window SHALL be grouped into
one further render, and SHALL NOT postpone it indefinitely: a bounded maximum
wait SHALL apply however long the stream keeps arriving.

#### Scenario: A single change is not delayed

- **WHEN** one event arrives on an idle screen
- **THEN** the screen re-renders immediately rather than after the coalescing window

#### Scenario: A multi-hop transition settles once

- **WHEN** a transition writes several hops milliseconds apart
- **THEN** the screen renders on the first and once more when they have all landed, not once per hop

#### Scenario: Continuous change still redraws

- **WHEN** events keep arriving faster than the coalescing window
- **THEN** the screen still redraws within the bounded maximum wait

### Requirement: The sign-in screen presents the tenant in the tenant's colour

The sign-in screen SHALL present the tenant's brand lockup above the sign-in card, and SHALL carry a
soft accent glow behind that card. Every colour on the screen SHALL resolve a design token rather
than a literal value, so that a tenant accent configured with `tix theme` and each of the light,
dark and dim schemes reach it without a further edit.

The glow SHALL be softened by its own gradient stops rather than by a blur filter, and SHALL be
painted behind the card rather than over it.

#### Scenario: The tenant accent reaches the glow

- **WHEN** a tenant configures a non-default accent and its sign-in screen is served
- **THEN** the glow behind the card is drawn in that accent

#### Scenario: The screen renders in every scheme

- **WHEN** the sign-in screen is viewed in the light, dark and dim schemes
- **THEN** each scheme renders its own background, surface, ink and accent, and no element keeps a
  value belonging to another scheme

#### Scenario: No colour is hard-coded

- **WHEN** the stylesheet rules that paint the sign-in screen are read
- **THEN** each colour resolves a custom property rather than a literal value

### Requirement: The sign-in screen does not restyle the screens it shares a layout with

The shell-less layout SHALL serve both the sign-in screen and the refusal screen. Styling added for
the sign-in screen SHALL apply only to the sign-in card, and SHALL NOT change the measure, heading
size or spacing of any other screen rendered in that layout.

#### Scenario: The refusal screen keeps its own shape

- **WHEN** a refusal is rendered in the shell-less layout
- **THEN** its heading and measure are unchanged by the sign-in screen's rules

### Requirement: A live region is held open only by work the reader did

A screen that defers its refresh while the reader is mid-edit SHALL decide
that from what the reader changed, and not from what the browser did on its
own.

A live region SHALL NOT be reported as holding work in progress on a page that
has only been rendered.

#### Scenario: A board that has only been rendered updates

- **GIVEN** an open project board whose cards each offer a Move control
- **WHEN** a task on it changes anywhere else and the event reaches the screen
- **THEN** the board re-fetches its fragment and shows the change, rather than deferring

#### Scenario: A move the reader picked is not thrown away

- **GIVEN** a reader who has opened a card's Move menu and chosen a column
- **WHEN** an event arrives for that board
- **THEN** the refresh is deferred until the menu is back at its default, and then lands

### Requirement: A select that declares no default is untouched on its first option

A `<select>` whose rendered markup names no option as selected is presented by
every browser with its first option selected. That presentation SHALL NOT be
read as an edit: such a control SHALL be treated as untouched while it sits on
its first option, and as edited once it sits on any other. A `<select>` whose
markup does declare a default SHALL continue to be read against that default.

#### Scenario: A declared default is still what a choice is measured against

- **GIVEN** a select whose markup marks an option other than the first as selected
- **WHEN** the control sits on the first option instead
- **THEN** it is reported as edited

### Requirement: A control in a live region declares its own default

A control rendered inside a live region SHALL declare its own default, so that
what the reader is shown and what the markup asked for agree. The project
board's Move control SHALL mark its first route selected.

#### Scenario: The Move control names the option the browser will show

- **WHEN** the board is rendered, as a whole page or as its live fragment
- **THEN** each Move select carries exactly one selected option, and it is the first route

### Requirement: The task list opens on unfinished work

With no filter asked for, the task list SHALL show only tasks in states the
project's workflow does not treat as terminal. It SHALL say on screen that the
listing is narrowed and SHALL offer a single control that shows everything.

#### Scenario: A list with finished work in it

- **WHEN** a reader opens the task list without asking for a filter, in a
  tenant holding both open and finished tasks
- **THEN** only the unfinished tasks are listed, the summary says the listing
  is open work only, and a link shows the finished work too

#### Scenario: Asking for everything

- **WHEN** that reader follows the link
- **THEN** finished tasks are listed as well, and the summary counts both

#### Scenario: Everything is finished

- **WHEN** every task in the listing is in a terminal state
- **THEN** the summary reads as nothing open rather than as nothing on the
  list, because a project whose work is finished is not a project with no work

#### Scenario: A workflow that renamed its terminal state

- **WHEN** the tenant's workflow calls its terminal state something other than
  the built-in name
- **THEN** that state is still excluded from the default listing

#### Scenario: Paging through everything

- **WHEN** a reader who has asked for finished work follows the next-page
  control
- **THEN** the following page still lists finished work, because the position
  it addresses was walked over that same set

#### Scenario: Following the control from a listing the reader built

- **WHEN** a reader follows the control from a listing carrying a sort, a page
  size or a deadline window
- **THEN** the listing of everything keeps all three

### Requirement: The board is not narrowed to the open states

The board view SHALL draw every state its workflow declares, terminal states
included, whatever the list view's default is. Its columns are the workflow,
so a default that empties the terminal ones removes the only place a finished
card can be drawn.

#### Scenario: Finishing a card

- **WHEN** a reader on the board moves a card into a terminal state
- **THEN** the card is drawn in that state's column

#### Scenario: The board says nothing about a narrowing that did not happen

- **WHEN** a reader opens the board without asking for a filter
- **THEN** the screen does not report the listing as open work only

### Requirement: A finished task carries no deadline

A task in a terminal state SHALL NOT be drawn as overdue or due soon on any
surface.

#### Scenario: A task finished after its due date

- **WHEN** a task whose due date has passed is moved to a terminal state
- **THEN** it carries no due badge

#### Scenario: Reopening it

- **WHEN** that task is moved back out of the terminal state
- **THEN** its deadline is compared to the present again

### Requirement: Navigation offers only what the reader can reach

The configuration and data navigation groups SHALL be shown only to a reader
holding a scope that makes at least one screen in them reachable.

#### Scenario: A member refused at an administration screen

- **WHEN** a reader without any administration scope requests one of those
  screens and is refused
- **THEN** the refusal page does not offer the administration navigation

#### Scenario: A reader who can use one of those screens

- **WHEN** a reader holding the scope one of those screens is gated on, and no
  other administration scope, opens that screen
- **THEN** the page offers the navigation group the screen belongs to

### Requirement: The task board is one strip of columns at every width

The task board SHALL remain readable as the number of columns grows with the projects merged into
it, and SHALL NOT make the page scroll sideways.

The board SHALL be drawn as one row of columns at every viewport width, scrolled horizontally within
its own frame.

#### Scenario: Many columns scroll within the board rather than overflowing the page

- **GIVEN** a merged board with more columns than fit the viewport's width
- **WHEN** it is drawn
- **THEN** the board scrolls sideways within its own frame
- **AND** the page itself does not scroll horizontally

#### Scenario: The board stays one strip at phone width

- **GIVEN** a viewport of phone width
- **WHEN** the board is drawn
- **THEN** the columns are one row scrolled sideways rather than stacked one above another
- **AND** the board's height stays within the viewport rather than running past whatever follows it

### Requirement: Board columns neither wrap nor stack

The board's columns SHALL NOT wrap onto further rows and SHALL NOT be stacked one above another,
because a board column is a fixed-height scroller and both arrangements grow past that height,
overrunning whatever the page draws beneath the board.

#### Scenario: The board does not draw over what follows it

- **GIVEN** a project board with a settings panel beneath it
- **WHEN** the board is drawn at phone width
- **THEN** the settings panel is visible and operable below the board rather than covered by it

### Requirement: At phone width the board snaps one column to the viewport

At phone width the board SHALL snap one column to the viewport, so that a column is read whole
rather than in parts.

#### Scenario: A phone settles on one column at a time

- **GIVEN** a viewport of phone width
- **WHEN** the reader swipes the board sideways
- **THEN** it comes to rest with a single column filling the viewport

### Requirement: A card on a board of several projects names its project

A card on a board covering more than one project SHALL name the project it belongs to.

#### Scenario: A merged card names its project

- **GIVEN** a board merging two projects
- **WHEN** a card is drawn
- **THEN** the card carries its own project's key

### Requirement: A task row sheds its least-identifying attributes as the viewport narrows

A task row SHALL keep its title legible as the viewport narrows, by omitting the attributes that
identify a task least before the title is compressed.

Below a narrow threshold the row SHALL omit the tags and the last-updated stamp, and below a
narrower one it SHALL also omit the project. The status, the priority, the assignee, the lease
state, the deadline mark and the reference SHALL be shown at every width.

#### Scenario: A phone row keeps its title

- **GIVEN** a task with several tags, an assignee, a project and a deadline
- **WHEN** its row is drawn at phone width
- **THEN** the title is drawn without being compressed to make room for the tags

#### Scenario: Identifying attributes survive every width

- **GIVEN** any viewport width the interface supports
- **WHEN** a task row is drawn
- **THEN** its status, priority, assignee, lease state, deadline mark and reference are present

### Requirement: An attribute a narrow row omits stays on the task screen

An attribute omitted for width SHALL remain available on the task's own screen, which the row
links to, so that nothing is unreachable rather than merely unshown.

#### Scenario: The shed attributes are on the task screen

- **GIVEN** a row drawn at a width that omits its tags
- **WHEN** the reader opens the task
- **THEN** the tags are listed there

### Requirement: A failure states itself in the flow of its own screen

When a screen cannot be drawn, the reason SHALL be rendered in the flow of the page, beneath the
heading that announces the failure, and SHALL be set to a measure that keeps it readable as a
sentence.

The reason SHALL NOT be rendered as a transient notification, so that it cannot be placed apart from
the heading it explains, mistaken for an unrelated notice, or drawn outside the reader's view.

#### Scenario: The reason sits under its heading

- **GIVEN** a request that cannot be served
- **WHEN** the failure screen is drawn
- **THEN** the reason appears beneath the heading, in the flow of the page

#### Scenario: The reason is not a toast

- **GIVEN** a failure screen
- **WHEN** it is drawn
- **THEN** the reason is not rendered as the transient notification the rest of the interface uses for flashes

### Requirement: The failure screen's ways out are drawn as navigation controls

The screen SHALL offer the ways out of it as controls carrying the same appearance as the other
navigation controls in the interface.

#### Scenario: The ways out look like the controls they are

- **GIVEN** a failure screen offering a way back
- **WHEN** it is drawn
- **THEN** that way back is drawn as a control rather than as bare text

### Requirement: The error screen's ways out lead where they say

Every link the error screen offers SHALL be labelled with where it leads, and no link SHALL point at
an address that redirects somewhere other than its label. A failed mutation SHALL offer the screen it
was submitted from. A failed page SHALL offer the same-origin page that linked to it, and SHALL offer
the task list when there is none or when that page is the failing one. The task list SHALL always be
one of the ways out.

#### Scenario: A missing page with no referrer

- **WHEN** a signed-in reader requests a page that does not exist, with no referring page
- **THEN** the error screen offers "Back to tasks" leading to the task list
- **AND** it offers no link to the root address

#### Scenario: A missing page reached from a filtered listing

- **GIVEN** a reader on the task list filtered to one tag
- **WHEN** they follow a link to a task that no longer exists
- **THEN** the error screen leads back to that filtered listing and also offers the task list

### Requirement: The error screen reaches the browser as HTML at its own status

The error screen SHALL reach the browser as the page it is, at its own status, under the server that
also serves the API, and SHALL NOT be replaced by the API's error envelope.

#### Scenario: A missing task under the full server

- **GIVEN** `tix serve` serving the API and the browser interface together
- **WHEN** a signed-in reader opens a task reference that does not exist
- **THEN** they get the HTML error screen with a not found status, not a JSON body

### Requirement: The task screen's back link returns to the listing it was opened from

The task screen SHALL offer a link back to the listing the reader opened it from, filter, sort, view
and page intact, and SHALL label it with that listing's name.

#### Scenario: Opened from a filtered list

- **GIVEN** the task list filtered to `tag:ops`
- **WHEN** the reader opens a task from it
- **THEN** the back link is labelled "Back to tasks" and leads to the list filtered to `tag:ops`

### Requirement: The back link's listing comes from next, then the referrer, then the task list

The listing SHALL be taken from the `next` the link carried, else from the referring page when that
is a task listing or a project board, else from the `next` of the referring page when that page is
the same task screen, else the task list. A `next` that is not a relative path on this origin SHALL
be replaced by the task list.

#### Scenario: Opened from a project board

- **WHEN** the reader opens a task from a card on the `infra` project's board
- **THEN** the back link is labelled "Back to the infra board" and leads to that board

#### Scenario: Opened from nowhere in particular

- **WHEN** the reader opens a task from a link that carries no listing and no listing referred them
- **THEN** the back link leads to the task list

### Requirement: Saving or deleting on the task screen keeps the listing it came from

Every redirect that returns the reader to the task screen after a change made on it SHALL keep the
listing, so the link survives any number of saves. Deleting the task SHALL return the reader to that
listing.

#### Scenario: The listing survives a save

- **GIVEN** a task opened from the task list filtered to `tag:ops`
- **WHEN** the reader adds a tag and then a comment on the task screen
- **THEN** the back link still leads to the list filtered to `tag:ops`

### Requirement: Adding a task stays on the listing by default

Creating a task from the task list, by the one-line box or by Add with details, SHALL return the
reader to the listing they added from, filter, sort, view and page intact, unless this browser has
turned the "Open a new task" preference on. A browser that has never chosen SHALL be treated as
having chosen to stay. A browser whose stored preference says it turned opening on SHALL keep
opening the new task.

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

### Requirement: The listing returned to after adding names and marks the new task

The listing SHALL then say that the task was created, SHALL link the new task, and SHALL mark its
row or card when it is on the page.

#### Scenario: Adding twice names the second task

- **GIVEN** the reader has just added one task and is on the listing with its toast
- **WHEN** they add another
- **THEN** the toast links and the marked row is the second task, not the first

### Requirement: The task fields chosen in View apply to every drawing of the tasks

The task listing's View menu SHALL offer a Star field and a Repeats field among the task fields, both
shown by default. The Repeats field SHALL draw the repeat glyph together with the task's rule. Board
cards and lane cards SHALL draw a field the cards carry -- star, status on a lane card, priority,
tags, repeat rule and reference -- only when it is chosen.

#### Scenario: A repeating task shows its rule

- **GIVEN** a task repeating `every week` and a browser with no column choice
- **WHEN** the list is drawn
- **THEN** its row shows the repeat glyph and the words `every week`

#### Scenario: Hiding a field hides it on cards

- **GIVEN** the reader has hidden Tags and Repeats under View
- **WHEN** the tasks are drawn as lanes or as a board
- **THEN** no card draws a tag or a repeat rule

### Requirement: Hiding the Star field keeps starred order and the task screen's star

Hiding Star SHALL remove the star toggle from rows and cards without changing the order starred tasks
are drawn in, and the star SHALL remain on the task's own screen.

#### Scenario: Hiding the star keeps the order

- **GIVEN** a starred task and an unstarred one, and Star hidden under View
- **WHEN** the list is drawn
- **THEN** no star toggle is drawn and the starred task is still first

### Requirement: The task view preference has no unset position

The task view preference SHALL have exactly three positions on every control that offers it: the
list, the board and lanes. No control SHALL offer a position meaning that no choice has been made.

The list SHALL be what an absent, empty or unrecognised stored value means, and choosing the list
SHALL store nothing.

#### Scenario: The switch on settings has the same three positions

- **GIVEN** a reader on the settings screen
- **WHEN** they read the view switch
- **THEN** it offers the list, the board and lanes and nothing else
- **AND** exactly one of them is marked as current

#### Scenario: An untouched browser reads as the list on both screens

- **GIVEN** a browser that has never used either switch
- **WHEN** the reader opens the settings screen and the task screen
- **THEN** both show the list as the current position

### Requirement: Row and task screen actions answer in place

With JavaScript, starring, moving or changing the priority of a task from a list row SHALL replace
that row only, and starring, moving, re-prioritising, tagging or commenting on the task screen SHALL
replace the page content without the sidebar. The scroll position SHALL be kept and focus SHALL
return to the control that was used. Without JavaScript the same forms SHALL post and redirect.

#### Scenario: Starring a row halfway down the list

- **GIVEN** the task list scrolled so the twentieth row is in view
- **WHEN** the reader stars that row
- **THEN** the star fills at once, only that row is replaced, the list stays where it was, and the
  star keeps focus

#### Scenario: No JavaScript

- **GIVEN** a browser with scripting off
- **WHEN** the reader stars a row
- **THEN** the form posts and the listing is drawn again on that row

### Requirement: An in-place action draws its result first and puts it back if refused

The chosen state SHALL be drawn before the server answers, and a refused or failed action SHALL put
the previous state back and say why.

#### Scenario: A move the server refuses

- **WHEN** a move chosen from a row's status menu is refused
- **THEN** the row shows its previous status and a toast says why

### Requirement: A toast offers to undo what it reports

A toast reporting a move SHALL offer Undo when the task's workflow has a route back to the state it
left; one reporting a priority change SHALL offer the previous priority back; one reporting a soft
delete SHALL offer to restore the task. Undo SHALL be a form that works without JavaScript, and the
undo SHALL NOT be carried into any address the screen builds from the request.

#### Scenario: Moving a task by mistake

- **WHEN** the reader moves a task from todo to doing
- **THEN** the toast offers Undo, and pressing it moves the task back to todo

#### Scenario: A move with no way back

- **GIVEN** a workflow in which done has no transition to review, directly or through other states
- **WHEN** a task is moved from review to done
- **THEN** the toast offers no Undo

#### Scenario: A permanent delete

- **WHEN** a task is deleted permanently
- **THEN** the toast offers no Undo

### Requirement: Live refreshes never take the screen from the reader

A live refresh SHALL wait while an action in the refreshed region is on its way or a menu in it is
open, and when it lands it SHALL keep the scroll position of scrolled boxes inside the region, keep
focus on the same control, and leave the new links and forms working as boosted ones.

#### Scenario: A refresh after the reader scrolled the list

- **GIVEN** the task list scrolled down
- **WHEN** another reader changes a task and the list is refreshed
- **THEN** the list stays scrolled where it was and a title clicked afterwards opens without a full
  page load

### Requirement: Moving between screens feels immediate

Links into tasks and listings SHALL start on the primary mouse button's press, except a link on a
card that can be dragged, which SHALL open on its click so the press can start a drag. The task
filter SHALL apply as the reader types, after a pause, and when a select beside it changes, and the
Apply button SHALL remain. The list, board and lanes switch SHALL swap the page without reloading
its assets.

#### Scenario: Dragging a card by its title

- **GIVEN** a board with dragging on
- **WHEN** the reader presses a card's title and drags it to another column
- **THEN** the card moves to that column and the task is not opened

#### Scenario: Dropping a starrable card

- **GIVEN** a board card that carries a star button ahead of its Move form
- **WHEN** the reader drops the card on a column it may move to
- **THEN** the move is posted to the Move form's action and the star is left as it was

### Requirement: Boosted navigations cross-fade unless the reader prefers reduced motion

Boosted navigations SHALL cross-fade where the browser supports view transitions, and SHALL NOT for
a reader who prefers reduced motion.

#### Scenario: Reduced motion

- **GIVEN** a reader whose system prefers reduced motion
- **WHEN** they open a task from the list
- **THEN** the screen is swapped with no cross-fade

### Requirement: Back to the task list restores where it was scrolled

Back to the task list SHALL restore the list's scroll position.

#### Scenario: Back to the list

- **GIVEN** the task list scrolled to the fortieth row
- **WHEN** the reader opens a task and presses Back
- **THEN** the list is shown scrolled to where it was

### Requirement: The keyboard shortcuts can be found

The sidebar SHALL carry a control that opens the keyboard shortcuts overlay, and "?" SHALL open the
overlay on every screen, including after any number of boosted navigations.

#### Scenario: After navigating

- **WHEN** the reader opens a task from the list and presses "?"
- **THEN** the shortcuts overlay opens

### Requirement: Every screen's heading is spaced the same way

On every signed-in screen, the distance from the page heading block to the first content under it
SHALL be the same at a given viewport width, and a line qualifying the heading SHALL sit the same
distance under the heading on every screen that has one. Both distances SHALL come from shared
tokens, and controls beside the heading SHALL NOT make the heading block deeper.

#### Scenario: A screen with header controls and one without line up

- **GIVEN** a viewport 1440px wide
- **WHEN** the reader opens the API tokens screen, whose header carries a Columns button, and then the
  workflows screen, whose header carries none
- **THEN** the first content under the heading begins the same distance below the heading on both

#### Scenario: A screen opening on a section

- **GIVEN** a screen whose first content under the heading is a section heading
- **WHEN** it is drawn
- **THEN** the section heading begins the same distance under the page heading as a table or a form
  does on other screens

### Requirement: Every screen's content column has the same width cap

On a viewport wide enough to show the sidebar as a column, the content column SHALL be capped by one
measure on every signed-in screen, so its right edge is in the same place on every screen at a given
viewport width. Prose, form fields, forms and charts inside it MAY carry narrower measures of their
own.

#### Scenario: A form screen and a listing end in the same place

- **GIVEN** a viewport 1920px wide
- **WHEN** the reader opens the task screen and then the projects screen
- **THEN** the content of both ends the same distance from the right edge of the window

### Requirement: Controls in one row share one height

Every input, select, button and link styled as a button in the task screen's quick-add row, its
filter row and its header SHALL have the height set by the shared control height token, so no
control in a row is taller or shorter than its neighbours.

#### Scenario: The quick-add row

- **WHEN** the task screen is drawn on a desktop
- **THEN** the title box, the project select, the Add button and the Add with details button are the
  same height

### Requirement: Add with details opens from the quick-add row

Where the fuller add form is offered on the task screen, the control that opens it SHALL be a button
at the end of the quick-add row, at the row's control height, and the form SHALL open over the page
without moving the task list. The form SHALL close on Escape, on a click outside it and on its own
Cancel button, without discarding what was typed. On a phone width the button MAY show its icon
alone and SHALL keep its accessible name.

#### Scenario: Opening the fuller form

- **GIVEN** a reader on the task screen with the advanced screens on
- **WHEN** they press Add with details
- **THEN** the form with title, project, priority, notes, tags and repeat opens under the button
- **AND** the task list stays where it was

#### Scenario: Closing and reopening keeps the draft

- **GIVEN** the reader typed a title into the fuller form
- **WHEN** they press Escape and then Add with details again
- **THEN** the title they typed is still there

### Requirement: Screens and assets are compressed

The browser interface SHALL send HTML screens, fragments and static assets gzip-compressed to a
client whose `Accept-Encoding` accepts gzip, SHALL send them uncompressed otherwise, and SHALL mark
every response `Vary: Accept-Encoding`. An asset's ETag SHALL be the digest of the bytes sent in
that encoding.

#### Scenario: A browser that accepts gzip opens the task list

- **WHEN** a browser sending `Accept-Encoding: gzip, deflate, br` opens `/tasks`
- **THEN** the response carries `Content-Encoding: gzip` and is under a third of the page's size

#### Scenario: A client that refuses gzip

- **WHEN** a client sends `Accept-Encoding: gzip;q=0`
- **THEN** the page and the stylesheet arrive uncompressed

### Requirement: A form's CSRF value differs on every response

Every form on a rendered screen SHALL carry the browser's CSRF value masked with a one-time pad drawn
for that response, and the cookie's own value SHALL NOT appear in any page. A submission SHALL be
accepted when its value unmasks to the cookie's value or equals it, and refused otherwise.

#### Scenario: Two renders of the same screen

- **WHEN** the task list is rendered twice for one browser
- **THEN** the form token differs between the two, and both are accepted on a post

#### Scenario: A tampered token

- **WHEN** a form is posted with one character of its token changed
- **THEN** the post is refused as forbidden

### Requirement: The settings screen is navigable by section

The settings screen SHALL group its controls into named sections and SHALL offer a navigation listing
every section as a link to that section on the same page. The navigation SHALL work without
scripting. Where the screen has room beside the sections the navigation SHALL be a column that stays
in view while the sections scroll; where it does not, it SHALL be drawn above the sections and wrap
rather than scroll sideways.

#### Scenario: Every section is linked

- **WHEN** a signed-in reader opens the settings screen
- **THEN** the section navigation carries one link per section, each pointing at that section's id

#### Scenario: A narrow screen does not scroll sideways

- **GIVEN** a viewport 390px wide
- **WHEN** the settings screen is drawn
- **THEN** the page is no wider than the viewport

### Requirement: A setting is a row with its control in one column

On a wide viewport each setting SHALL be drawn as a row with its name and a short description on the
left and its control on the right, and the controls of every row SHALL share one column width so that
they align. At a viewport 900px wide or narrower each row SHALL stack its control under its
description. A setting whose explanation is longer than its description SHALL offer it behind an
information disclosure beside its name.

#### Scenario: Rows stack on a narrow screen

- **GIVEN** a viewport 900px wide
- **WHEN** the settings screen is drawn
- **THEN** each row's control sits under its description

### Requirement: An on/off setting is an accessible switch that still posts a form

An on/off preference SHALL be drawn as a switch that exposes its state to assistive technology,
names its state in words such as On and Off or Shown and Hidden, and remains a plain form
submission.

#### Scenario: An on/off preference is a switch

- **WHEN** the settings screen is drawn
- **THEN** each on/off preference's submit button has the switch role and reports whether it is on

### Requirement: Saving a setting says so

Submitting a preference from the settings screen SHALL return the reader to the section the
preference belongs to with a message naming what was saved.

#### Scenario: Saving returns to the section

- **WHEN** the reader turns Drag to move off on the settings screen
- **THEN** the browser is returned to the settings screen at the Appearance section
- **AND** a message says the preference was saved

### Requirement: Hover effects are a per-browser preference

Rows, cards, lane cards, navigation items, buttons and links SHALL respond to a pointer hovering them
on devices that can hover. Whether they do SHALL be a per-browser preference, on by default, kept and
controlled like every other display preference. When it is off the root element SHALL carry an
attribute that disables every hover effect the preference governs. A transition used by a hover
effect SHALL NOT run when the reader's system asks for reduced motion.

#### Scenario: On by default

- **GIVEN** a browser that has never chosen
- **WHEN** any signed-in screen is drawn
- **THEN** the root element does not carry the off attribute

#### Scenario: Turned off

- **WHEN** the reader turns Hover effects off on the settings screen
- **THEN** every later screen's root element carries the off attribute

#### Scenario: Reduced motion

- **GIVEN** a system that asks for reduced motion
- **WHEN** the reader hovers a row
- **THEN** no transition runs

### Requirement: Configure screens hold together at every width

The configuration and data screens SHALL draw without the page scrolling sideways at viewport widths
of 1440px, 900px and 390px. A table that cannot fit SHALL scroll inside its own frame, a row button's
label SHALL NOT break between its letters, and an information note SHALL stay inside the viewport.

#### Scenario: The token table with Revoke open

- **GIVEN** a viewport 900px wide and a tenant with API tokens
- **WHEN** the reader opens a token's Revoke dialog
- **THEN** the page is no wider than the viewport

### Requirement: The page content starts the same distance from the sidebar on every screen

On a viewport wide enough to show the sidebar as a column, the horizontal distance between the
sidebar's edge and the start of the page content SHALL be the same on every signed-in screen at a
given viewport width, and SHALL come from a single rule. A screen MAY cap how wide its content grows,
but that cap SHALL NOT move where the content starts.

#### Scenario: A form screen and a listing start in the same place

- **GIVEN** a viewport 1920px wide
- **WHEN** the reader opens the task screen and then the projects screen
- **THEN** the content of both begins the same distance from the sidebar

#### Scenario: A screen's measure does not centre it

- **GIVEN** a screen whose content is capped narrower than the space beside the sidebar
- **WHEN** it is drawn
- **THEN** its content begins at the gutter beside the sidebar and the spare width is on the right

### Requirement: The sidebar can be resized by dragging its edge

On a viewport wide enough to show the sidebar as a column, the sidebar's right edge SHALL be a
control that changes the sidebar's width when dragged. The width SHALL be clamped between a minimum
and a maximum, SHALL be kept per browser, and SHALL be applied before the page paints, so a reload or
a navigation never draws the default width first. Restoring the default width SHALL remove what was
kept.

#### Scenario: A dragged width survives a reload

- **GIVEN** a reader who drags the sidebar's edge to 320px
- **WHEN** they reload the page
- **THEN** the sidebar is 320px wide from the first paint

#### Scenario: A drag past the maximum stops at the maximum

- **WHEN** the reader drags the edge further right than the maximum width
- **THEN** the sidebar is drawn at the maximum width

### Requirement: The sidebar's resize edge is a keyboard separator, absent without scripting

The control SHALL be reachable by keyboard as a vertical separator that exposes its current value,
minimum and maximum, and the arrow keys SHALL change the width. Without scripting the control SHALL
be absent and the sidebar SHALL keep its default width.

#### Scenario: The edge is moved from the keyboard

- **GIVEN** the separator has focus
- **WHEN** the reader presses the right arrow
- **THEN** the sidebar widens and the separator's value reports the new width

#### Scenario: No script, no handle

- **GIVEN** a browser with scripting off
- **WHEN** any signed-in screen is drawn
- **THEN** no resize control is offered and the sidebar is its default width

### Requirement: The sidebar collapses to a rail

On a viewport wide enough to show the sidebar as a column, the sidebar SHALL offer a button that
collapses it to a narrow rail and expands it again, and dragging its edge below a threshold SHALL
collapse it. The collapsed state SHALL be kept per browser and applied before the page paints, and
expanding SHALL return the sidebar to the width it had.

#### Scenario: Dragging it small collapses it

- **WHEN** the reader drags the sidebar's edge below the collapse threshold and lets go
- **THEN** the sidebar is drawn as a rail holding the logo and the navigation icons
- **AND** a button to expand it is visible

#### Scenario: The button works on its own

- **GIVEN** an expanded sidebar
- **WHEN** the reader presses the collapse button and then presses it again
- **THEN** the sidebar is a rail after the first press and back at its previous width after the second

### Requirement: A collapsed sidebar rail keeps every entry's icon and name

The rail SHALL keep the product logo and every navigation entry as an icon, and every entry in the
rail SHALL keep its accessible name.

#### Scenario: A rail entry still has its name

- **GIVEN** a collapsed sidebar
- **WHEN** assistive technology reads a navigation entry in the rail
- **THEN** the entry is announced by the same name it has when the sidebar is expanded

### Requirement: The collapsed sidebar has no effect below the column layout

On a viewport too narrow for the sidebar column, the collapsed state SHALL have no effect, and the
sidebar SHALL be the bar and menu it is without it.

#### Scenario: The phone layout ignores the collapsed state

- **GIVEN** a browser that collapsed the sidebar on a desktop
- **WHEN** a signed-in screen is drawn at phone width
- **THEN** the brand bar and the menu button are drawn as they are for a browser that never collapsed it

### Requirement: A browser chooses whether a task opens as a page or in a dialog

The web interface SHALL offer a per-browser preference, Task opens in, stored in the `tix_task_open`
cookie, whose positions are Page and Dialog. Absent, or any value other than `dialog`, SHALL mean
Page, which SHALL behave exactly as the interface did before the preference existed. The control
SHALL be a segmented control in the Listings section of Settings that posts without scripting, names
both positions and marks the one in force.

#### Scenario: An untouched browser opens tasks as pages

- **GIVEN** a browser that has never set Task opens in
- **WHEN** the reader opens the task list
- **THEN** the page carries no task dialog
- **AND** the Settings control for Task opens in marks Page as current

#### Scenario: Choosing Dialog

- **WHEN** the reader presses Dialog in the Task opens in control on Settings
- **THEN** the browser stores `tix_task_open=dialog`
- **AND** the reader is returned to the Listings section with a toast naming the setting
- **AND** the task list carries the task dialog

### Requirement: In Dialog mode a task link opens the task screen over the page

When Task opens in is Dialog, a plain primary click on a link to a task screen, on any screen other
than a task screen, SHALL open that task in a modal dialog over the page instead of navigating. This
SHALL apply to task titles in the list, on board cards and lane cards, to the activity feed, to
search results and to the toast offered after adding a task. Under 600px the dialog SHALL fill the
screen.

#### Scenario: Opening and closing from the list

- **GIVEN** Task opens in is Dialog and the reader is on the task list
- **WHEN** they click a task's title
- **THEN** the task screen is drawn in a modal dialog over the list
- **AND** the address is the task's
- **WHEN** they press Escape
- **THEN** the dialog closes, the address is the list's again, and focus is on the title they clicked

### Requirement: A task dialog carries the task's address and Back closes it

The address SHALL change to the task's address, so that reloading or opening the address elsewhere
draws the full task page, and Back SHALL close the dialog and leave the page under it as it was.

#### Scenario: Back closes the dialog

- **GIVEN** a task open in the dialog over a filtered list
- **WHEN** the reader presses the browser's Back
- **THEN** the dialog closes and the filtered list is still on screen, not reloaded

#### Scenario: A deep link is the full page

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader reloads while a task is open in the dialog
- **THEN** the full task page is drawn, with its back link

### Requirement: A task dialog closes on Escape, Close or its backdrop and returns focus

Escape, the dialog's Close button and a click on the backdrop SHALL close the dialog, and focus
SHALL return to the element that opened it, or to the same task's row when that element has been
redrawn.

#### Scenario: A click on the backdrop closes the dialog

- **GIVEN** Task opens in is Dialog and a task opened in the dialog from the list
- **WHEN** the reader clicks the backdrop outside the dialog
- **THEN** the dialog closes and focus is on the title they clicked

### Requirement: A modified click or no scripting opens the full task page instead of the dialog

A click with a modifier key, a middle click, and any click in a browser without scripting SHALL open
the full task page as before.

#### Scenario: A modified click opens a new tab

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader ctrl-clicks a task's title
- **THEN** no dialog opens and the browser opens the full task page as it would for any link

### Requirement: The server draws the task screen alone for the dialog

A request for a task screen carrying the header `X-Tix-Dialog: 1` SHALL be answered with the task
screen's content and its flash, without the layout, the sidebar or the page's live binding, inside
one element marking it as the dialog's answer. The back link SHALL be a Close button. The same
request without the header SHALL be answered with the full page. Other screens SHALL ignore the
header.

#### Scenario: The dialog's answer

- **WHEN** a signed-in reader requests a task screen with `X-Tix-Dialog: 1`
- **THEN** the answer carries the task's title, its edit form and its comments
- **AND** it carries no `<html>`, no sidebar and no live binding
- **AND** it offers Close where the page offers a back link

#### Scenario: Saving from the dialog

- **GIVEN** a task open in the dialog
- **WHEN** the reader saves its edit form, changes its status or priority, stars it, adds a tag or a
  comment
- **THEN** the dialog is redrawn with the result in place
- **AND** the row or card for the task on the page under it shows the change

#### Scenario: Deleting from the dialog

- **GIVEN** a task open in the dialog
- **WHEN** the reader deletes it
- **THEN** the dialog closes and the listing is drawn without the task

### Requirement: The dialog's answer keeps its ids apart from the page's

Ids the task screen shares with a listing's rows SHALL be distinct in the dialog's answer, so a
control in the dialog cannot address an element of the page behind it.

#### Scenario: The dialog's ids are its own

- **WHEN** a signed-in reader requests a task screen with `X-Tix-Dialog: 1`
- **THEN** its comment field and its priority menu carry ids of the dialog's own
- **AND** neither carries the id the same control has on the full page or on a list row

### Requirement: A browser chooses whether clicking a title edits it

The web interface SHALL offer a per-browser preference, Clicking a title, stored in the
`tix_title_click` cookie, whose positions are Open the task and Edit the title. Absent, or any value
other than `edit`, SHALL mean Open the task. The control SHALL be a segmented control in the Listings
section of Settings that posts without scripting, names both positions and marks the one in force. A
post naming a position SHALL store it; a post naming none SHALL move to the other position.

#### Scenario: Choosing Edits the title

- **WHEN** the reader presses Edit the title in the Clicking a title control on Settings
- **THEN** the browser stores `tix_title_click=edit`
- **AND** every task title in the list, on board cards and on lane cards is marked as editable and
  has an open link beside it

### Requirement: A title can be edited where it is listed

When Clicking a title is Edits the title, a plain click on a task's title in the list, on a board
card or on a lane card SHALL turn it into a text field holding the title. Enter, or leaving the
field, SHALL save it through the task's existing edit address with the version the listing was drawn
with; Escape SHALL put the title back unchanged. An unchanged title SHALL send nothing. Enter or F2
on a selected row SHALL edit its title.

#### Scenario: Renaming from the list

- **GIVEN** Clicking a title is Edits the title
- **WHEN** the reader clicks a title, types a new one and presses Enter
- **THEN** the task's title is the new one and the row shows it
- **AND** the task's description and assignee are unchanged

### Requirement: A refused inline rename is shown beside the field it leaves open

A refusal, including a conflict with a newer version and an empty title, SHALL be shown beside the
field with the field kept open.

#### Scenario: Somebody else saved first

- **GIVEN** a listing drawn at a task's version 3, and the task since saved at version 4
- **WHEN** the reader renames it from the listing
- **THEN** the save is refused and the reason is shown beside the field
- **AND** the title is unchanged

### Requirement: An editable title keeps an open link and is a plain link without scripting

The open link beside the title SHALL open the task, as a page or in the dialog as Task opens in
says. Without scripting the title SHALL be a link to the task.

#### Scenario: An editable title still leads to its task

- **GIVEN** Clicking a title is Edits the title
- **WHEN** the tasks are drawn as a list, a board or lanes
- **THEN** every title has an open link to its task beside it
- **AND** the title itself is a link to the task, which a browser without scripting follows

### Requirement: A save that does not name the description or assignee leaves them alone

Saving a task through the task screen's edit address SHALL change the description only when the
submission names `body`, and the assignee only when it names `assignee`. A submission naming either
with an empty value SHALL clear it, as before.

#### Scenario: Saving only the title

- **GIVEN** a task with a description and an assignee
- **WHEN** a submission to its edit address names only `version` and `title`
- **THEN** the title changes and the description and assignee do not

### Requirement: Screens are drawn in the language the browser asks for

The browser interface SHALL negotiate a language from the request's `Accept-Language` header, choosing
the highest-weighted language it ships, matching an exact tag before its base language, and SHALL draw
English when the header names none it ships. Text a language's catalog lacks SHALL be drawn in
English. The interface SHALL ship English and Bulgarian.

#### Scenario: A language the build does not ship

- **WHEN** a browser sends `Accept-Language: de-DE,fr;q=0.8`
- **THEN** the screen is drawn in English with `lang="en"`

### Requirement: Every screen carries its negotiated language in lang and draws its text in it

Every screen SHALL carry the chosen language in the root element's `lang` attribute and SHALL draw its
text, flashes, page title, error heading and hint, and fixed labels in that language.

#### Scenario: A Bulgarian browser

- **WHEN** a browser sends `Accept-Language: bg-BG,bg;q=0.9,en;q=0.5` and opens the task screen
- **THEN** the root says `lang="bg"` and the navigation, the flash and the error screen are in Bulgarian

### Requirement: A browser can choose its language on settings

The settings screen SHALL offer a Language row listing the browser default and every shipped
language, each named in its own language. A chosen language SHALL be remembered per browser in the
`tix_lang` cookie and SHALL outrank `Accept-Language`. Choosing the browser default, or a value the
build does not ship, SHALL remove the cookie. The saved toast SHALL be drawn in the page's language.

#### Scenario: Bulgarian chosen in an English browser

- **WHEN** a browser sending `Accept-Language: en-US` chooses Bulgarian on settings
- **THEN** the next screen says `lang="bg"` and the toast reads "Език: запазено"

#### Scenario: Back to the browser default

- **WHEN** that browser then submits a language the build does not ship
- **THEN** the cookie is removed and screens follow `Accept-Language` again

### Requirement: Text written by scripts follows the page's language

For a language other than English, every screen SHALL carry, in its head, the translations of the
texts the shipped scripts write themselves, and the scripts SHALL use them. A script SHALL fall back
to the English text when no translation is carried. An English page SHALL carry no such table.

#### Scenario: The sidebar toggle in Bulgarian

- **WHEN** a Bulgarian page is loaded
- **THEN** the head carries the Bulgarian text for "Collapse sidebar" and the toggle is labelled with it

### Requirement: Every user-facing template string goes through the catalog

No template SHALL write visible text, or a `title`, `placeholder`, `aria-label`, `alt`,
`data-rail-name`, `aria-valuetext` or `data-confirm` value, other than through the translation
helpers, except text inside `code`, `kbd`, `pre` and `samp`. A screen exempted from this check SHALL
be named, with its reason, in the test that makes it.

#### Scenario: A new English literal

- **WHEN** a template gains `<p class="group">Data</p>`
- **THEN** the test suite fails naming the template and the literal

### Requirement: Every translation catalog holds exactly the keys the interface uses

Every language other than English SHALL hold an entry for every key the interface uses and no entry
it does not, and each entry SHALL format the same number of arguments and carry the same markup as
its key.

#### Scenario: A missing translation

- **WHEN** a key used by a template has no Bulgarian entry
- **THEN** the test suite fails naming the key and where it is used

### Requirement: Unfamiliar concepts carry an info tip

Beside a concept a newcomer cannot decode from the screen (claims and leases, tokens and scopes,
the filter syntax, recurrence, stars, list, board and lane views, workflows, states and categories,
webhooks, connections, sync, domains, import and export, retention, SSH keys, statistics figures,
the activity feed) the interface SHALL offer an info tip. A tip SHALL NOT link to another origin.

#### Scenario: A tip names its page rather than linking it

- **WHEN** a reader opens the tip beside the Domains screen's heading
- **THEN** it names `docs/tenancy.md` as text
- **AND** it carries no link

### Requirement: An info tip is a named disclosure that works without scripting and names its doc

An info tip SHALL be a disclosure that works without scripting, from the keyboard and on touch, is
named "About" its topic for assistive technology, explains the concept in one or two plain
sentences in the page's language, and names the `docs/` page that says more where one exists.

#### Scenario: The view switch explains itself

- **WHEN** a reader opens the tip beside the task screen's view switch
- **THEN** it explains list, board and lanes and names `docs/web-ui.md`, and its button is announced as "About Views"

### Requirement: Completing a task animates only the task that was completed

The web interface SHALL animate the completion mark only on the row that was acted on.

A checkbox that is already complete SHALL NOT replay its completion animation when the page is rendered
for any other reason: a load, a filter change, a column change or a navigation. Motion on a screen means
something just happened, and a page that animates fifteen checkboxes in sequence on load says that fifteen
things happened when nothing did.

#### Scenario: Ticking a task

- **WHEN** a task is completed from the list
- **THEN** exactly one completion mark animates, on the row that was completed

#### Scenario: Loading a list that already contains completed tasks

- **WHEN** a list containing completed tasks is rendered
- **THEN** no completion mark animates

### Requirement: The static completed state is painted without the animation

Everything static about the completed state SHALL remain outside the animation, so that a page rendered
without JavaScript still paints completion marks correctly.

#### Scenario: Completion marks render without JavaScript

- **WHEN** the page is rendered and no script runs
- **THEN** completed tasks still show their completion mark

### Requirement: Assets carry a cache validator

Every static asset response SHALL carry a validator derived from the bytes served.

An asset URL that does not change between releases and carries no validator lets a browser keep serving
the previous release's stylesheet against the current release's markup, with nothing to tell either side
that it has happened. This was observed during review: a reader saw new markup styled by an old sheet and
reported the interface as broken.

#### Scenario: An upgrade replaces a cached asset

- **WHEN** an asset's bytes change and a browser revalidates with the validator from before
- **THEN** it is sent the new bytes rather than told to reuse what it has

#### Scenario: Every asset validates independently

- **WHEN** two assets are served
- **THEN** their validators differ, so changing one does not invalidate the other

### Requirement: An unchanged asset is revalidated without its body

Every static asset response SHALL answer a conditional request for an unchanged asset without sending the
body again.

#### Scenario: An asset is revalidated rather than refetched

- **WHEN** a browser requests an asset it already holds, quoting the validator it was given
- **THEN** the response says it is unchanged and carries no body

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

#### Scenario: A listing with more than one page

- **WHEN** a listing has a further page
- **THEN** it offers a next control and says which page is on screen

#### Scenario: Every paged listing renders the same control

- **WHEN** any keyset-paged listing is rendered
- **THEN** its position control is the shared one, not a control of its own

### Requirement: A keyset listing that fits on one page renders no position control

A listing that fits on one page SHALL render no control at all, rather than a pair of controls that lead
nowhere.

#### Scenario: A listing that fits on one page

- **WHEN** a listing has no further page and no previous one
- **THEN** it renders no position control

### Requirement: A reader can go back to the page they came from

A keyset cursor addresses only the page that follows it, so a listing SHALL carry, in its own URL, the
cursors of the pages already walked, and the previous control SHALL return to the last of them.

The whole position SHALL live in the URL, so a link to a page deep in a listing can be shared, reloaded
and bookmarked, and SHALL require no change to the store, the domain or the API.

#### Scenario: Walking back

- **WHEN** a reader on the third page uses the previous control
- **THEN** they are returned to the second page, and its own previous control returns to the first

### Requirement: The first page of a keyset listing offers no previous control

The first page has no cursor, so the previous control SHALL be absent there rather than present and
inert.

#### Scenario: The first page

- **WHEN** the first page of a listing is rendered
- **THEN** it offers no previous control

### Requirement: A record of walked pages that was not issued here is discarded whole

The record of the walk SHALL be treated as untrusted input: a value carrying anything this deployment
did not issue, or more entries than the bound this deployment sets, SHALL be discarded whole rather than
partly kept, and the reader SHALL be returned to the first page. Keeping part of it would send the
previous control to a page the reader was never on, and a value that grows without bound would lengthen
with every page walked.

#### Scenario: A record of the walk that was not issued here

- **WHEN** a listing is requested with a record of the walk carrying a value this deployment did not issue
- **THEN** the screen renders, the value is discarded, and the previous control returns to the first page

#### Scenario: A record of the walk longer than the bound

- **WHEN** a listing is requested with more walked pages than the deployment's bound allows
- **THEN** the value is discarded rather than walked

### Requirement: Changing the filter or the sort discards the record of walked pages

Changing the filter or the sort SHALL discard the record of the walk. Its cursors address positions in one
ordered result set; under a different filter, or a different ordering, they name rows that were never on
the reader's screen.

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

A task claimed again after an expiry SHALL report the live claim rather than the old expiry.

#### Scenario: The expiry is no longer recent

- **WHEN** the expiry is older than the window the domain treats as recent
- **THEN** the row reports nothing about it

#### Scenario: Claimed again

- **WHEN** a task with an earlier expiry is claimed again and the new lease is live
- **THEN** the row reports the live claim, not the earlier expiry

### Requirement: The expired-claim report is read from durable evidence, not the lease fields

The report SHALL be read from the durable evidence of the expiry rather than from the lease fields, which
a sweeper clears within a minute of the lease lapsing. Reading the lease fields made the report
unreachable in practice: a task an agent took and stopped answering for looked exactly like a task nobody
had ever touched, which is the opposite of what the report is for.

#### Scenario: The sweeper has cleared the lease

- **WHEN** a task whose claim expired is listed after the sweeper has cleared its lease fields
- **THEN** the row reports the expired claim, how long ago it lapsed, and who held it

### Requirement: An expired claim is shown with how many times the task was claimed

How many times a task has been claimed SHALL be shown beside such an expiry, where repeated claims
followed by a lapse distinguish a holder that keeps failing from work that has simply changed hands.

#### Scenario: A holder that keeps failing

- **WHEN** a task that has been claimed more than once reports an expired claim
- **THEN** the row also says how many times it has been claimed

### Requirement: A column preference records what is hidden

The cookie that carries a reader's column choice SHALL record, for each listing, the columns that listing
leaves OUT, and SHALL show every other column the build declares.

Recording the columns shown cannot distinguish a column the reader turned off from a column that did not
exist when the reader chose, so every column added afterwards read as one the reader had refused. The
readers it silenced were exactly those who had used the picker at all.

#### Scenario: A column is added to a listing

- **WHEN** a build declares a column that did not exist when a reader chose that listing's columns
- **THEN** the column is shown to that reader without the stored choice being touched

### Requirement: Column and project visibility preferences both record what is hidden

Project visibility already records what is hidden, for the same reason, and the two preferences SHALL
agree on that.

#### Scenario: A project is created after projects were put away

- **WHEN** a reader has put a project away and a new project is then created and given a task
- **THEN** the task screen shows the new project's task without the reader ticking it

### Requirement: A listing never chosen for keeps the declared default columns

A listing the reader has never chosen for SHALL keep the declared defaults, which is a different state
from a listing whose hidden set is empty: the first shows the columns declared on by default, the second
shows every declared column. The stored value SHALL distinguish them.

#### Scenario: Hiding nothing

- **WHEN** a reader ticks every column a listing offers
- **THEN** the listing shows the column that is declared off by default, which an untouched install does not

### Requirement: A column preference in an earlier form is converted, not misread

A value written in an earlier form SHALL NOT be read as though it were written in the current one. It
SHALL be recognised and converted, so the reader's recorded choices survive unchanged and no choice is
inverted, and the columns that earlier form could not name SHALL be shown rather than counted as refused.

#### Scenario: A choice recorded in the earlier form

- **WHEN** a stored value written when the cookie recorded the columns shown is read
- **THEN** every column that reader put away is still put away, and every column they kept is still shown

### Requirement: The widest column preference fits what the build reads back

The value SHALL stay within the size this build will read back for the widest choice the picker can
produce, which is every listing with every column hidden.

#### Scenario: The widest choice

- **WHEN** every listing is stored with every column hidden
- **THEN** the value is within the size the build reads back, so the choice is not silently discarded

### Requirement: A refused filter reports on the filter bar

A filter expression the listing cannot answer SHALL be reported beside the filter box, on the listing,
with the expression left in the box, and SHALL NOT replace the screen with the error page.

The screen SHALL be served as a success, because it is the listing that was asked for, and a browser that
swaps a fragment swaps nothing from a response that is not one, which would leave the previous page on
screen carrying no message at all.

#### Scenario: A handle nobody has

- **WHEN** a filter names an actor this tenant does not have
- **THEN** the listing renders with the message beside the filter box and the expression still in it

### Requirement: Every way a filter term is refused is reported alike

Every way a term can be refused SHALL be treated alike: an expression that does not parse, and one that
names a project, status, tag or actor this tenant does not have. A rule written per term goes stale the
next time the filter language learns to refuse something.

#### Scenario: An expression that does not parse

- **WHEN** a filter expression cannot be parsed
- **THEN** it is reported the same way, on the filter bar

### Requirement: A listing failure that is not the filter's fault still fails the page

A failure that is not the expression's fault SHALL still fail the page.

#### Scenario: A failure of the page itself

- **WHEN** a listing fails for a reason that is not the filter expression
- **THEN** the page fails as it did before

### Requirement: The view panel's arrangement and its decoration agree

The panel that holds the column choice and the project choice SHALL size each to what it holds: the
column list is fixed by the build, the project list grows with the tenant.

A rule drawn between the two SHALL follow the arrangement they are actually in, rather than a viewport
width that only guesses at it. A track count chosen by the browser can wrap the sections apart at any
width, leaving a rule that separates nothing.

#### Scenario: The panel is too narrow for two columns

- **WHEN** the panel renders in one column
- **THEN** the rule between the sections runs across rather than down

### Requirement: Each submit in the view panel names what it commits

Each choice's submit SHALL name what it commits, since two controls in one panel both labelled only
"Apply" say nothing about which is which.

#### Scenario: Committing one of the two choices

- **WHEN** the panel is open
- **THEN** each submit names the choice it applies

### Requirement: The task listing is named for what it shows

The task listing SHALL carry a heading, and a navigation entry, that name the set of tasks it actually
renders. The listing is the tenant's queue: it carries every actor's rows, people and agents alike, and
the summary beneath it counts the projects they span and the ones an agent is holding.

The heading and the navigation entry SHALL agree with each other and with the rows below them, so that
neither can be corrected while the other goes on claiming otherwise.

#### Scenario: The entry that leads to it

- **WHEN** the navigation is rendered
- **THEN** the entry leading to the task listing names it the same way its heading does

### Requirement: The tenant's queue is not presented as the reader's own work

The listing SHALL NOT be presented as the reader's own work while it renders everybody's. A reader who
wants only their own rows already has a precise way to ask for them, the `assignee:<handle>` term of the
filter expression, which the assignee column writes into the filter box in one click; a heading is not a
second, weaker way of saying the same thing.

#### Scenario: The listing carries rows assigned to other actors

- **WHEN** the task listing renders rows belonging to several actors
- **THEN** its heading names the listing rather than claiming it is the reader's own work

### Requirement: A retention window is written the way it is shown

A retention window SHALL be rendered in the duration vocabulary the product prints elsewhere, so that a
thirty day window reads as `30d` rather than as `720h0m0s`.

A value the duration grammar does not accept SHALL still be refused, with a message naming the field and
the value and saying what a window looks like.

#### Scenario: A window that is not a duration

- **WHEN** a retention field is submitted with a value the duration grammar does not accept
- **THEN** the submission is refused and the message says what a window looks like

### Requirement: A retention field accepts every value it renders

A retention field SHALL accept every value it renders. A form that prints a value its own handler refuses
is a form that rejects its own contents on save, so the rendering and the parsing are one change and not
two: the screen SHALL read a submitted window through the same grammar that produced the value in the
box, which knows the day unit that Go's own duration syntax does not.

#### Scenario: Saving the value the screen rendered

- **WHEN** the value a retention field renders is submitted back unchanged
- **THEN** it is accepted, and the field renders the same value again

### Requirement: Every row of the tenant tree carries a figure or says why it has none

Each row of the diagram showing what sits under a tenant SHALL render either its live count or, in the
count's place, a short statement of why it carries none.

A row whose listing errored SHALL say that its count is unavailable rather than rendering nothing.

#### Scenario: A kind the service can count

- **WHEN** the tenant tree renders a kind whose listing returns
- **THEN** that row carries the figure

#### Scenario: A listing that failed

- **WHEN** a count cannot be made because its listing errored
- **THEN** the row says the count is unavailable rather than leaving a gap

### Requirement: A tenant tree row the service cannot count names where its figure lives

A blank beside a column of figures reads as a count that failed rather than one deliberately not made,
and a count that really did fail read exactly like one nobody attempted. A row whose figure the service
cannot produce SHALL name where that figure does live instead: a task listing is paginated by cursor and
carries no total, and a custom field is declared on one project at a time.

#### Scenario: A kind that has no tenant-wide figure

- **WHEN** the tenant tree renders a kind the service cannot total
- **THEN** that row says where its figure lives, in the figure's place

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
value's purpose is not evident from the label, the same notice idiom the fields beside it carry.

The tenant key was a bare run of body text between a field's help and the form's submit, reading as
output left behind rather than as a value anybody was meant to use.

#### Scenario: A value whose purpose the label alone does not give

- **WHEN** such a value is one used outside the browser
- **THEN** its notice says where it is used and that this form cannot change it

### Requirement: A value no control can change is read-only, unsubmitted and selectable

Such a value SHALL be rendered as read-only rather than editable, and SHALL NOT be submitted with the
form.

A label and a notice are what say which value it is, that a shell addresses this tenant by it, and that
this form cannot change it; the read-only rendering is what stops a reader typing into a box whose Save
was never going to carry it.

The value SHALL remain selectable, because the reason it is on the screen is that it has to be copied
somewhere else.

#### Scenario: A value no handler saves

- **WHEN** a form renders a value that none of its handlers accept
- **THEN** the value is labelled, rendered read-only, and not submitted with the form

### Requirement: Arriving at a listing leaves the reader on the listing

A listing SHALL NOT move focus away from the page when it loads. The reader arrived to read the rows, and
a form at the top of the screen taking focus makes the form the subject: it draws a focus ring onto the
loudest position on the screen, starts a screen reader at a text box rather than at the page's heading,
and on a small screen raises the keyboard over the rows.

#### Scenario: Loading the task list

- **WHEN** the task list is rendered, with or without a message from a previous action
- **THEN** no control on it takes focus, and the reader's starting point is the page itself

### Requirement: A listing control worth reaching has a key in the keyboard scheme

Where a listing carries a control worth reaching without a pointer, the keyboard scheme SHALL bind a key
to it and that binding SHALL appear in the shortcut help, so reaching the control costs one keystroke
rather than a page that has already decided for everybody.

#### Scenario: Reaching the quick-add field

- **WHEN** a reader presses the key their scheme binds to the new-task action
- **THEN** focus moves to the quick-add field

### Requirement: A row draws its project icon according to the icon's kind

When the task list shows the project's icon beside a row, it SHALL draw a text icon as text, a
built-in icon from the interface's own symbol sprite and an uploaded image through an image element.

#### Scenario: A built-in icon is drawn from the sprite

- **WHEN** the task list renders a task belonging to a project whose icon is the built-in rocket
- **THEN** the row draws the sprite's rocket symbol and no emoji

#### Scenario: An uploaded icon is drawn as an image

- **WHEN** the task list renders a task belonging to a project whose icon is an uploaded image
- **THEN** the row draws an image element whose address names the project and the image's digest

### Requirement: Project icon picker

The project creation form and the project settings form SHALL offer the icon as a choice of kind,
Emoji, Icon or Upload, together with a choice of no icon. The Emoji kind SHALL take the text icon,
the Icon kind SHALL offer every built-in symbol as a grid of choices each drawn from the sprite and
labelled in the reader's language, and the Upload kind SHALL take a file.

#### Scenario: Choosing a built-in icon

- **WHEN** a reader picks Icon, selects the rocket and saves the project
- **THEN** the project carries the built-in rocket icon and its board header draws it

#### Scenario: Uploading an image

- **WHEN** a reader picks Upload, chooses a PNG file and saves the project
- **THEN** the project carries that image and the settings form previews it

#### Scenario: Removing the icon

- **WHEN** a reader picks no icon and saves the project
- **THEN** the project carries no icon of any kind

### Requirement: The project icon form previews the icon and keeps an uploaded image

Each form offering the project icon picker SHALL show a preview of the icon the project has, SHALL
carry an explanatory tip like the other settings, and SHALL keep an uploaded image when the form is
saved without a new file.

#### Scenario: Saving other settings keeps the image

- **WHEN** a project with an uploaded icon has its name changed on the settings form and no new file chosen
- **THEN** the project keeps its image

### Requirement: Project icon images are served safely

The browser interface SHALL serve a project's uploaded icon at `/projects/{key}/icon` with the
image's own content type, `X-Content-Type-Options: nosniff`, a content security policy that forbids
script and sandboxes the document, and a strong entity tag derived from the image's digest, and
SHALL answer a request whose `If-None-Match` matches with `304 Not Modified`. A project without an
uploaded icon SHALL answer not found.

#### Scenario: Headers on an icon

- **WHEN** a signed-in reader requests a project's icon
- **THEN** the response carries the image bytes, its content type, `nosniff`, a `sandbox` policy and an ETag

#### Scenario: Revalidation

- **WHEN** the same request is repeated with the ETag in `If-None-Match`
- **THEN** the response is 304 with no body

### Requirement: A custom project colour is drawn legibly wherever a palette colour is

A custom colour SHALL be drawn wherever a palette colour is drawn: the row stripe, the board card,
the lane, the project list and the project key. In each colour scheme it SHALL be drawn in a shade
that has a contrast ratio of at least 4.5:1 against that scheme's page and surface backgrounds, and a
colour that already meets that ratio SHALL be drawn exactly as chosen. Only a colour the server
parsed and formatted itself SHALL reach a `style` attribute.

#### Scenario: A custom colour is legible in every scheme

- **WHEN** a project carries a custom colour and is rendered under the light, low contrast and dark schemes
- **THEN** each scheme draws a shade of it with a contrast ratio of at least 4.5:1 against that scheme's page and surface

#### Scenario: A custom colour that already reads is not changed

- **WHEN** a project carries a custom colour that meets 4.5:1 against the light scheme's page and surface
- **THEN** the light scheme draws exactly that colour

### Requirement: Every colour control offers the predefined colours and a custom one

Every control in the web interface that chooses a colour SHALL be the same shared control. It SHALL
offer the predefined choices as swatches, SHALL offer a custom colour through a native colour input
and a text field accepting `#` followed by six hexadecimal digits, SHALL show a live preview of the
custom colour, and SHALL carry an information tip explaining the two. Where a colour is optional it
SHALL offer none as a choice.

#### Scenario: The project form offers both

- **WHEN** a reader opens the new project form
- **THEN** it offers every palette colour, none, and a custom colour with a colour input, a hex field and a preview

#### Scenario: The current custom colour is shown as chosen

- **WHEN** the project settings form is opened for a project with a custom colour
- **THEN** the custom choice is selected and the hex field and colour input hold that colour

### Requirement: The shared colour control works without scripting on every colour form

The control SHALL submit a custom colour without scripting through the text field. The new project
form, the project settings form and the tenant theme form SHALL use it.

#### Scenario: A custom colour is saved from the browser

- **WHEN** a reader chooses custom and enters `#1a7f5a` on the project settings form and saves
- **THEN** the project carries `#1a7f5a` and its rows are drawn in it

#### Scenario: The tenant theme form offers a custom accent

- **WHEN** an administrator opens the tenant form
- **THEN** the theme is chosen with the same control, offering every resolvable theme and a custom accent

### Requirement: The field form carries the whole definition

The browser's field definition form SHALL offer every part of a definition the service takes: key,
label, type, enum options, required, default and position. A default SHALL be converted to
the field's declared type before it is saved, and one that does not fit SHALL be refused with the
reason.

#### Scenario: A default saved from the browser

- **WHEN** a field `points` of type `int` is saved from the form with default 3 and position 2
- **THEN** the stored definition carries default 3 and sits at position 2

#### Scenario: A default that does not fit

- **WHEN** a field of type `int` is saved with default `big`
- **THEN** the form answers 400 and nothing is stored

### Requirement: The new project form starts from the name

The projects screen SHALL offer "New project" as a button in its header. The form it opens SHALL put
the name first and focus it, then the key, the workflow and the description, and SHALL group the
colour and icon controls under "Appearance". Name and key SHALL be marked as required. The project
settings on a board SHALL use the same layout and the same Appearance group.

#### Scenario: Opening the form

- **WHEN** the reader opens New project
- **THEN** the name field is first and focused, the key follows it, and colour and icon sit under Appearance

### Requirement: The new project form suggests a free key from the name

With scripting on, the key SHALL be suggested from the name until the reader edits it, SHALL avoid
keys already in use, and the form SHALL say what the project's tasks will be numbered.

#### Scenario: Suggesting a key

- **GIVEN** a project with the key `web`
- **WHEN** the reader types the name "Web shop"
- **THEN** the key field reads `web2` until the reader types a key of their own

### Requirement: A refused project form keeps what was typed

A project create or settings save the service refuses for a reason the reader can correct SHALL be
answered with the same screen, the form drawn open, every submitted value kept, and the message under
the field the refusal names. The response SHALL carry the refusal's status.

#### Scenario: A key already in use

- **GIVEN** a project with the key `infra`
- **WHEN** the reader creates a project named "Infra two" with the key `infra`
- **THEN** the projects screen is drawn with the form open, the name still "Infra two", and the refusal under the key field

#### Scenario: Clearing a project's name

- **WHEN** the reader saves a project's settings with an empty name
- **THEN** the board is drawn with its settings open and the refusal under the name field

### Requirement: Create and settings forms share their keyboard and actions

The add task, new project and project settings forms SHALL place their primary action at the bottom
right with Cancel beside it. With scripting on, Ctrl+Enter or Cmd+Enter SHALL submit the form through
its ordinary submission, and Escape SHALL cancel it. With scripting off every form SHALL still submit
and every refusal SHALL still be drawn in place.

#### Scenario: Submitting from the keyboard

- **WHEN** the reader presses Ctrl+Enter in the description of Add with details
- **THEN** the form is submitted as if Create task had been pressed

#### Scenario: Cancelling a form drawn in place

- **GIVEN** a refused new project form drawn in place
- **WHEN** the reader presses Escape
- **THEN** the browser follows the form's Cancel link

### Requirement: Deleting a project in the browser is confirmed on a screen that asks about its tasks

The project's delete control SHALL lead to a confirmation screen rather than deleting. For a project holding no
tasks the screen SHALL say so and offer one button that deletes it.

Choosing another target SHALL redisplay the mapping for it without the CSRF token entering a URL, and SHALL work
without scripting. The screen SHALL carry a help tip explaining the two choices.

#### Scenario: The delete button asks first

- **WHEN** the reader presses Delete project in the danger zone
- **THEN** a confirmation screen opens and the project still exists

### Requirement: The confirmation for a project holding tasks offers moving or deleting them

For a project holding tasks the screen SHALL state how many, offer moving them and deleting them as two radio
choices, offer a select of the other live projects as the move target, show the status mapping the selected target
implies before anything is confirmed, and state plainly that deletion is permanent and cannot be undone. When no
other live project exists the move choice SHALL be shown as unavailable with the reason.

#### Scenario: The move choice shows the mapping

- **GIVEN** a project with tasks and another project with a different workflow
- **WHEN** the confirmation screen is opened with that project selected
- **THEN** each source status is listed with the status it maps to and its task count

#### Scenario: Moving from the browser

- **WHEN** the move choice is submitted with a target
- **THEN** the project is deleted, its tasks are in the target, and the reader lands on the target's board

### Requirement: Deleting a project's tasks in the browser needs the project key typed

The delete path SHALL require the project key to be typed; a submission whose typed key does not match SHALL be
refused on the same screen and SHALL delete nothing.

#### Scenario: A mistyped key deletes nothing

- **WHEN** the delete choice is submitted with a key that is not the project's
- **THEN** the screen is shown again with the refusal beside the key field and the project remains

### Requirement: The deletion confirmation is drawn in the shared form layout

The project deletion confirmation SHALL be drawn in the layout the create forms use: one card, controls of the
shared control height, refusals under the field they are about, and Cancel beside the action at the bottom
right. Each answer about the tasks SHALL be a card holding the controls that answer needs, and the mapping table
SHALL fit the card at any width without scrolling sideways.

#### Scenario: Each answer holds its own controls

- **WHEN** the confirmation is shown for a project with tasks and another live project
- **THEN** the move card holds the target select and the mapping, and the delete card holds the key field

### Requirement: The deletion confirmation leaves with Escape and keeps its actions in reach on a phone

Escape SHALL follow Cancel back to the project's board. On a phone the actions of the project deletion confirmation
SHALL stay at the bottom edge of the screen while the choices scroll.

#### Scenario: Escape leaves the confirmation

- **WHEN** the reader presses Escape in the confirmation
- **THEN** the browser returns to the project's board and nothing is deleted

### Requirement: The project settings change the key after a confirmation

The project settings form SHALL offer the project's key alongside its name and description, with an
explanation that task references change and old links redirect. Submitting a different key SHALL NOT
change anything at once: the reader SHALL be shown a confirmation naming the old and new key with
example task references before and after, and SHALL change the key only by confirming.

#### Scenario: A changed key asks first

- **GIVEN** project `infra` holding task `infra-1`
- **WHEN** the reader submits the settings with key `ops`
- **THEN** a confirmation shows `infra-1` becoming `ops-1` and the project is unchanged

#### Scenario: Confirming renames and lands on the new board

- **WHEN** the reader confirms the change of `infra` to `ops`
- **THEN** the key is `ops` and the reader is taken to `/projects/ops`

### Requirement: The key change confirmation carries the rest of the settings save

The confirmation SHALL carry every other submitted field so confirming also saves them, and SHALL work
without scripting. An icon image uploaded in the same save cannot be carried to a second screen, so
the confirmation SHALL ask for it again rather than drop it. Submitting the current key SHALL save
without a confirmation.

#### Scenario: The icon chosen with a new key is kept

- **WHEN** the reader picks a built-in icon and a new key in one save and confirms
- **THEN** the project carries both the new key and the icon
- **AND** had the reader uploaded an image instead, the confirmation asks for the file again

### Requirement: Old project and task URLs redirect to the new ones

A GET request for a project screen or a task screen addressed by a kept alias SHALL be answered with
a permanent redirect to the same screen addressed by the current key, keeping the rest of the path
and the query. A form post addressed by an alias SHALL be served without a redirect.

#### Scenario: An old project URL redirects

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** the reader opens `/projects/infra`
- **THEN** the response is a 301 to `/projects/ops`

#### Scenario: An old task URL redirects

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** the reader opens `/tasks/infra-8`
- **THEN** the response is a 301 to `/tasks/ops-8`

### Requirement: A command palette finds tasks, projects, screens and actions by name

Every signed-in screen SHALL carry a command palette, a modal dialog with one search box over a list
grouped into Tasks, Projects, Screens and Actions. It SHALL open on ctrl+k and on command+k in every
shipped key scheme, including while a text field has focus, and from a Search button in the sidebar and
in the phone's bar. The "?" overlay SHALL list its key. The sign-in screen SHALL NOT carry the palette.

#### Scenario: Opening from the keyboard

- **GIVEN** a signed-in reader on any screen, in any key scheme
- **WHEN** they press ctrl+k, or command+k on a Mac
- **THEN** the palette opens with the search box focused
- **AND** the screens and actions are listed at once

#### Scenario: The overlay names the key

- **WHEN** the reader presses "?"
- **THEN** the overlay lists ctrl+k and meta+k against Open the command palette

### Requirement: The palette offers every sidebar destination, Settings section and listed action

Screens SHALL be every sidebar destination the reader's sidebar shows and every section of Settings.
Actions SHALL include New task, New task in a named project, New project, Open the filter, Show tasks
as a list, a board and in lanes, Switch the colour scheme, Keyboard shortcuts and Sign out, each
driving a control the interface already has. Every label SHALL be in the page's language.

#### Scenario: A Bulgarian reader

- **GIVEN** a reader whose language is Bulgarian
- **WHEN** they open the palette
- **THEN** its placeholder, group names, screens and actions are in Bulgarian

### Requirement: The palette asks the server for tasks only once something is typed

Projects SHALL be read from `GET /palette/projects` when the palette opens. Tasks SHALL be asked for
from `GET /palette/tasks?q=` only when the search box holds text, once the reader has paused typing for
150ms, and an answer to an older query SHALL NOT be drawn.

#### Scenario: Opening sends no task search

- **WHEN** the reader opens the palette
- **THEN** no request is made for tasks

#### Scenario: Typing searches once

- **WHEN** the reader types "dep" quickly, one letter at a time
- **THEN** one task search for "dep" is made after they pause
- **AND** the matching tasks are listed under Tasks

### Requirement: The palette matches screens and actions in the browser

Screens and actions SHALL be embedded in the page and matched in the browser with a fuzzy match. Once
something is typed, the group holding the best match SHALL be listed first, and an answer arriving from
the server SHALL NOT move the selection off the option the reader had selected.

#### Scenario: The best match leads its group

- **WHEN** the reader types "board"
- **THEN** the Actions group, holding Show tasks as a board, is listed before Screens and Tasks

### Requirement: The palette's task search reads the filter language and stops at ten tasks

The task search SHALL read the query with the filter language, falling back to plain words when it does
not parse, SHALL put the task a ref-shaped query names first, SHALL leave out deleted tasks, and SHALL
answer at most ten tasks with keyset pagination.

#### Scenario: A ref finds its task

- **WHEN** the reader types a task's ref in any case
- **THEN** that task is the first under Tasks

### Requirement: The palette's routes answer only a signed-in reader's own tenant

Both palette routes SHALL require a signed-in reader and SHALL answer only the session tenant's
records. They SHALL add no capability: the API answers the same through `GET /api/v1/tasks?q=`,
`GET /api/v1/tasks/{ref}` and `GET /api/v1/projects`.

#### Scenario: Another tenant's task is never offered

- **GIVEN** a task in another tenant whose title matches the query
- **WHEN** the reader searches for it by title or by ref
- **THEN** the palette offers nothing for it

#### Scenario: A signed-out request is refused

- **WHEN** a browser without a session asks either palette route
- **THEN** it is not answered with JSON

### Requirement: The palette is driven from the keyboard and returns focus

ArrowDown and ArrowUp SHALL move the selection through every option, wrapping at either end, Enter
SHALL choose the selected option, a click SHALL choose the option clicked, and Escape, the Close button
or a click on the backdrop SHALL close the palette. Tab SHALL stay inside the palette. When the palette
closes without leaving the page, focus SHALL return to the element that had it.

#### Scenario: Choosing with the arrows

- **WHEN** the reader presses ArrowUp on the first option and then Enter
- **THEN** the last option is chosen

#### Scenario: Escape puts focus back

- **GIVEN** the reader opened the palette from a task row
- **WHEN** they press Escape
- **THEN** the palette closes and the row has focus again

### Requirement: The palette's search box is a combobox and its options fit a finger

The search box SHALL be a combobox naming the selected option with `aria-activedescendant`. On a coarse
pointer each option SHALL be at least 44px tall, and under 600px the palette SHALL fill the screen.

#### Scenario: The selection is announced

- **WHEN** the reader moves the selection with ArrowDown
- **THEN** the search box's `aria-activedescendant` names the newly selected option

### Requirement: A task chosen in the palette opens the way the reader chose

Choosing a task SHALL open it in the task dialog when Task opens in is Dialog and the page carries
the dialog, with focus returning to what had it before the palette opened when the dialog closes,
and SHALL open the task's page otherwise.

#### Scenario: Dialog mode

- **GIVEN** Task opens in is Dialog and the reader is on the task list
- **WHEN** they choose a task in the palette
- **THEN** the task opens in the dialog over the list

#### Scenario: Page mode

- **GIVEN** Task opens in is Page
- **WHEN** they choose a task in the palette
- **THEN** the browser goes to the task's page

### Requirement: Recently used items come first

The palette SHALL remember the last eight items chosen in this browser and SHALL list them first,
most recent first, under Recent when nothing is typed, and rank them higher when something is.
When the page offers `tix.recentTasks()`, the tasks it names SHALL come first; when it does not,
the palette SHALL work the same without them.

#### Scenario: A screen chosen before

- **GIVEN** the reader chose Settings from the palette
- **WHEN** they open the palette again
- **THEN** Settings is the first option

#### Scenario: Recently viewed tasks

- **GIVEN** the page offers recently viewed tasks
- **WHEN** the reader opens the palette
- **THEN** those tasks are listed first

### Requirement: Saved filters in the sidebar

The filter bar SHALL offer "Save this filter" with a name and a share choice whenever it holds an
expression or a deadline window, saving the expression, the deadline window, the sort and the view in
force. The sidebar SHALL list the reader's saved filters under Tasks, own first, then shared, each a link
that opens the task screen on the filter and sets the view it names, marked current when the screen shows
exactly that filter.

#### Scenario: Save from the filter bar

- **GIVEN** the task screen filtered with `tag:ops` and sorted by priority on the board
- **WHEN** the reader saves it as "Ops"
- **THEN** "Ops" is listed in the sidebar, and opening it later shows the board filtered and sorted the same way

### Requirement: A saved filter's menu renames, shares, moves and deletes it in place

A filter the reader may change SHALL carry a three dots menu that renames, shares or stops sharing,
moves up or down among the reader's own, and deletes; its forms work without scripting and redraw the
sidebar in place with the heading menus' focus return and keyboard rules.

#### Scenario: Moving a saved filter up from its menu

- **GIVEN** a reader's own saved filters "first" and then "second"
- **WHEN** the reader moves "second" up from its menu
- **THEN** the sidebar lists "second" before "first"

### Requirement: My work

The sidebar SHALL carry a My work entry showing the reader's open tasks assigned to them as the list,
grouped under Overdue, Today, This week, Later and No date read in the reader's time zone, leaving out an
empty group. A switch SHALL add the tasks the reader holds a claim on. The groups SHALL be arranged like
any list group, per reader, under the view `mine`.

#### Scenario: Grouped by deadline

- **GIVEN** a task of mine due yesterday, one due next month and one with no deadline
- **WHEN** I open My work
- **THEN** they appear under Overdue, Later and No date, in that order

### Requirement: Recently viewed

The sidebar SHALL list the last ten tasks this browser opened for the signed-in reader, newest first, in a
collapsible section, kept in the browser's own storage and drawn by script, so with scripting off it is
not offered. `tix.recentTasks()` SHALL answer the same list as `{ref, title, href, at}` entries.

#### Scenario: A task opened again moves to the top

- **GIVEN** a reader opened tasks A then B
- **WHEN** they open A again
- **THEN** the list reads A, B

### Requirement: The rail draws the reader's entries as icons

On the collapsed sidebar rail, My work and each saved filter SHALL be drawn as an icon keeping its name
for assistive technology, the saved filter menus SHALL be put away, and the recent section SHALL fold to
an icon that opens the sidebar.

#### Scenario: Collapsed rail

- **WHEN** the reader collapses the sidebar
- **THEN** My work and every saved filter are icons with their names as accessible names

### Requirement: Calendar feed settings

The Settings screen SHALL carry a Calendar feed section with an (i) tip. Its first row SHALL state
whether the reader has a feed. Without a feed it SHALL say No feed and offer a filter field and a
Create button; with one it SHALL say Active and show the filter with a Save button, a Rotate button and
a Revoke button.

#### Scenario: An empty section says there is no feed

- **WHEN** a signed-in reader with no feed opens Settings
- **THEN** the Calendar feed section's first row says No feed, above the filter field and Create

### Requirement: Calendar feed actions in Settings answer with the URL or a notice

Revoking SHALL return to the Calendar feed section with a notice saying so. Creating and rotating
SHALL answer a screen showing the URL once with a Copy button and a link to subscribe, readable and
selectable with scripting off.

#### Scenario: Creating a feed in the browser

- **WHEN** a signed-in reader presses Create in the Calendar feed section
- **THEN** the next screen shows the feed URL beside a Copy button, and returning to Settings shows
  the feed as active without the URL

#### Scenario: Revoking returns to the section

- **WHEN** a reader presses Revoke in the Calendar feed section
- **THEN** Settings opens at the Calendar feed section with a notice that the feed was revoked, and the
  section says No feed

### Requirement: Inbox in the web interface

The sidebar SHALL carry an Inbox entry with a badge counting unread items, hidden at zero, refreshed
live over the event stream when an `inbox.updated` event names the reader. Settings SHALL offer one
switch per kind to mute it, kept with the account.

#### Scenario: The badge counts unread items

- **GIVEN** a reader with two unread items
- **WHEN** they open any page
- **THEN** the sidebar's Inbox entry shows 2

#### Scenario: Muting from settings

- **WHEN** a reader turns off status changes in Settings
- **THEN** `status` is muted for them

### Requirement: The inbox page lists the reader's items with read controls

The inbox page SHALL list the reader's items newest first, each naming the task, the actor, what
happened and when, unread items marked, with a filter for unread only, per-item mark read and mark
unread, mark all read, and keyset paging. Every action SHALL work without scripting and swap in
place with it.

#### Scenario: Filtering to unread items

- **GIVEN** a reader with one read item and one unread item
- **WHEN** they open the inbox showing unread items only
- **THEN** the list holds the unread item and not the read one

### Requirement: Opening an inbox item marks it read, with or without scripting

With scripting, opening an item SHALL mark it read and open the task the way the reader's dialog
setting says; without it, the link SHALL open the task and the row's button SHALL mark it read.

#### Scenario: Opening an item

- **WHEN** a reader opens an unread item
- **THEN** the item is marked read and the task opens

### Requirement: Mention completion

Comment and description boxes SHALL offer the tenant's handles when the reader types `@` followed by
the start of a handle, filtered as they type, chosen with the arrow keys and Enter or Tab, dismissed
with Escape without closing anything else. Without scripting the box SHALL stay a plain text box.

#### Scenario: Completing a handle

- **WHEN** a reader types "@bo" in a comment box and presses Enter on "bob"
- **THEN** the box reads "@bob " and the comment is not submitted

### Requirement: Notification channels in Settings

Settings SHALL have a Notifications section listing the reader's channels and a form, in the create-form
layout, adding one. Each channel SHALL have a screen showing its signing secret once after creation, its
settings as a form, a Send test button and its recent deliveries with state and a Redeliver action for one
not delivered. A refused form SHALL be drawn again with the message beside the field and the values kept.

#### Scenario: Adding a channel

- **WHEN** a reader adds a channel in Settings
- **THEN** its screen shows the signing secret, and reloading it does not

### Requirement: The inbox redraws itself

The inbox page SHALL redraw its bar and list in place when an `inbox.updated` event arrives, keeping the
reader's scroll position and focus, and SHALL highlight the rows the redraw brought for a few seconds.

#### Scenario: A new item arrives while the inbox is open

- **WHEN** somebody mentions the reader while their inbox is on screen
- **THEN** the item appears at the top of the list, highlighted, without a reload

### Requirement: The sidebar's groups fold

The sidebar's Saved filters, Recently viewed, Configure and Data groups SHALL each have a heading that,
with scripting, is a button folding and unfolding the group, exposing `aria-expanded` and naming the
group's body with `aria-controls`. A folded group SHALL still show the entry for the page being read. On
the collapsed rail a fold SHALL have no effect. Without scripting the heading SHALL be plain text and
every group open.

#### Scenario: A folded group shows where the reader is

- **GIVEN** a folded Configure group
- **WHEN** the reader opens Users
- **THEN** Users is shown under the Configure heading, marked current, and the other Configure entries are not

#### Scenario: The rail ignores folds

- **GIVEN** a folded Configure group and a collapsed sidebar
- **WHEN** a signed-in screen is drawn at desktop width
- **THEN** every Configure entry is drawn as an icon in the rail

#### Scenario: No script, nothing to fold

- **GIVEN** a browser with scripting off
- **WHEN** any signed-in screen is drawn
- **THEN** each group heading is its name, not a button, and every group is open

### Requirement: Folded sidebar groups are kept in a cookie and drawn by the server

Which groups are folded SHALL be kept per browser in the `tix_sidebar` cookie and SHALL be drawn by the
server, so a folded group is folded from the first paint.

#### Scenario: Folding Configure is remembered

- **GIVEN** a reader who presses the Configure heading
- **WHEN** they open another screen
- **THEN** Configure is drawn folded from the first paint, its heading reporting it collapsed

### Requirement: Configure and Data start folded until the reader arranges the groups

Configure and Data SHALL start folded for a browser that has not folded or opened any group, since they
hold administration screens read far less often than tasks; once the reader opens or folds a group, the
reader's arrangement SHALL be kept instead, every group open included.

#### Scenario: The admin groups start folded

- **GIVEN** a browser with no sidebar cookie
- **WHEN** any screen is drawn
- **THEN** Configure and Data are drawn folded and Saved filters and Recently viewed are open

#### Scenario: Opening every group is kept

- **GIVEN** a reader who unfolded Configure and Data
- **WHEN** they open another screen
- **THEN** both groups are drawn open

### Requirement: Navigating does not move the sidebar

A boosted navigation SHALL draw the sidebar in its final state in the first frame after the swap: no
entry, control or label in it SHALL change position between the swap and the scripts that run after it.
A control in the sidebar that needs scripting SHALL be shown by a class set on the document root before
the body is parsed, not revealed by its script after a swap.

#### Scenario: Going from Tasks to Projects

- **GIVEN** a reader on Tasks with a recently viewed list and the Configure group showing
- **WHEN** they follow Projects in the sidebar
- **THEN** every sidebar entry is at the position it had before the click in every frame of the navigation

### Requirement: The sidebar's width and rail are kept in a cookie and drawn by the server

The sidebar's width and rail SHALL be kept per browser in the `tix_sidebar` cookie and drawn by the
server.

#### Scenario: A stored rail survives a navigation without a flash

- **GIVEN** a reader who collapsed the sidebar
- **WHEN** they follow any sidebar entry
- **THEN** the page is drawn with the rail in its first frame

#### Scenario: Earlier browser storage is adopted

- **GIVEN** a browser that kept the sidebar's width and rail in `localStorage` under an earlier build
- **WHEN** it loads a page
- **THEN** the cookie holds the same width and rail and the old keys are removed

### Requirement: The recently viewed list survives a boosted navigation

The recently viewed list SHALL be kept across a boosted navigation rather than redrawn from empty, and
opening one of its entries SHALL be a boosted navigation like any other sidebar entry.

#### Scenario: Moving between recently viewed tasks

- **GIVEN** a recently viewed list holding two tasks, the first one open
- **WHEN** the reader follows the second entry
- **THEN** the same two entries stay in place and only the current mark moves to the second

### Requirement: Project pickers name projects in the reader's chosen style

Every web control a project is chosen in (the add box, Add with details, bulk Copy to, a heading
menu's New task in, Copy to, New from template, a template's project, the move target of a project
delete and the statistics filter) SHALL label each project in the style held in the
`tix_project_label` cookie: `name-key` draws `Name (KEY)`, `key-name` draws `KEY · Name`, `name`
draws `Name` and `key` draws `KEY`. No cookie, or a value naming no style, SHALL draw `Name (KEY)`.

#### Scenario: Default style

- **GIVEN** a project `infra` named Infrastructure and no `tix_project_label` cookie
- **WHEN** the reader opens the task screen
- **THEN** the add box offers `<option value="infra">Infrastructure (INFRA)</option>`

### Requirement: A project label keeps the key as its value and falls back to it

The task screen's Project row SHALL name its project in the reader's chosen label style, linked by
the key. A project with no name SHALL be labelled by its key. A project with a text icon SHALL have it
before the label.
Each option's value SHALL be the stored project key whatever the style.

#### Scenario: Key first

- **GIVEN** the cookie holds `key-name`
- **WHEN** the reader opens Copy to on a task
- **THEN** the project option reads `INFRA · Infrastructure` and its value is `infra`

### Requirement: A project label shortens a long name and narrows to the key on a phone

A name longer than 24 characters SHALL be shortened with an ellipsis, and the key SHALL never be
shortened. On a screen at most 639px wide, the add box and a heading's add field SHALL list each
project by its key alone.

#### Scenario: A phone

- **GIVEN** a screen 390px wide
- **WHEN** the reader opens the task screen
- **THEN** the add box's project picker reads `INFRA`, whole

#### Scenario: A long name

- **GIVEN** a project named with more than 24 characters
- **WHEN** a picker lists it
- **THEN** the name ends in an ellipsis and the whole key follows it

### Requirement: The project label style is chosen on Settings

The Settings screen SHALL offer the project label style in its Listings section as a select with an
info tip, posting to `/projectlabel`. Posting a known non-default style SHALL store it in the
`tix_project_label` cookie for a year; posting the default or an unknown value SHALL clear the
cookie. The preference SHALL be listed in `web.Preferences` and documented in the web UI cookie
table.

#### Scenario: Choosing a style

- **WHEN** the reader posts `label=key-name` to `/projectlabel`
- **THEN** the cookie holds `key-name` and Settings shows it selected

#### Scenario: An unknown value

- **WHEN** the reader posts `label=bogus`
- **THEN** no `tix_project_label` cookie is left

### Requirement: Keys and references are drawn upper case in the browser

Every project key and task reference the web interface shows a reader (rows, cards, lanes, the task
screen and its title, breadcrumbs and relations, toasts, aria labels and titles, activity, inbox,
templates, the projects screen, the command palette, Recently viewed and its folded rail names, and the
question asked before a permanent delete) SHALL be drawn upper case, as
`INFRA` and `INFRA-8`.

#### Scenario: Task screen

- **WHEN** the reader opens `/tasks/infra-1`
- **THEN** the page title and the heading read `INFRA-1` and the project link reads `INFRA` over `/projects/infra`

### Requirement: Links keep lower-case keys and addresses answer in any case

Links, element ids, form values and data attributes SHALL keep the stored lower-case value. Addresses
SHALL answer in any case: `/tasks/INFRA-8` and `/projects/INFRA` SHALL show the same task and project
as their lower-case forms without a redirect, and a filter `project:INFRA` SHALL select the tasks of
`infra`.

#### Scenario: Upper-case address

- **WHEN** the reader opens `/tasks/INFRA-1`
- **THEN** the task is shown with status 200

### Requirement: The release screen leads with one status card

The release screen SHALL open with one card stating the installation's release status as exactly one
of: an update is available, up to date, the versions cannot be compared, the check failed, or not
checked yet. The card SHALL show the running version and the newest published version, or say the
newest is not known. An installation whose check has never succeeded SHALL NOT be told it is up to
date, and one whose check failed SHALL be told so rather than that it has not checked.

#### Scenario: Update available

- **WHEN** the newest published release is v0.30.0 and the server runs v0.28.0
- **THEN** the card says v0.30.0 is available and shows both versions

#### Scenario: Up to date

- **WHEN** the newest published release is the running version
- **THEN** the card says the installation is up to date

#### Scenario: Feed unreachable

- **WHEN** the last check of the release feed got no usable answer and none ever succeeded
- **THEN** the card says the feed could not be reached and the newest version is not known

#### Scenario: Not checked yet

- **WHEN** no check has finished
- **THEN** the card says the installation has not been checked yet

### Requirement: The release screen gives copyable upgrade commands

When a newer release exists, the release screen SHALL list one upgrade command per install method:
`tix update` for a release binary, a `podman pull` of the new image tag for the container, and
`go install` of the new version for the Go toolchain, each with a copy control bound to that command.
When no newer release is known it SHALL list none.

#### Scenario: Commands carry the version

- **WHEN** v0.30.0 is newer than the running version
- **THEN** the page offers `podman pull ghcr.io/heliopsy/tix:0.30.0` and `go install github.com/heliopsy/tix@v0.30.0` with copy buttons

#### Scenario: Nothing to upgrade to

- **WHEN** the installation is up to date or the check failed
- **THEN** no upgrade command is shown

### Requirement: Release notes render as sanitised Markdown

The release screen SHALL render the newest release's notes as Markdown through the same sanitising
renderer as a task description, with every link removed, including a bare `www.` address. A leading
heading that names the release SHALL be dropped, because the section heading names it.

#### Scenario: A release-please changelog

- **WHEN** the notes are a release-please changelog with Features and Bug Fixes headings and bold scopes
- **THEN** the page shows headings, list items and bold text, and no Markdown markers or links

### Requirement: The running version opens what changed

The running version in the sidebar footer SHALL be a link to the "What's new" section of the release
screen, `/release#changes`, and SHALL work without scripting.

#### Scenario: Clicking the version

- **WHEN** a signed-in reader clicks the version in the sidebar footer
- **THEN** the browser opens `/release` at the section with id `changes`

### Requirement: The release screen shows the bundled changelog

The release screen SHALL show a "What's new" section drawn from the `CHANGELOG.md` embedded in the
binary at build time. When the running version is a tagged release with an entry, that entry SHALL be
shown first and marked as the version running here, followed by up to ten earlier entries, each
collapsible.

#### Scenario: A tagged release

- **WHEN** the server runs v0.31.0 and the changelog has entries for 0.31.0 and earlier
- **THEN** the 0.31.0 entry is shown first, marked as running here, and earlier entries are folded

### Requirement: A build that is not a tagged release lists the newest changelog entries

When the running version is not a plain `major.minor.patch` with an entry (a development build, a build
from an edited tree, or a fork's version), the section SHALL say the build is not a tagged release and
list the newest entries instead.

#### Scenario: A development build

- **WHEN** the server runs a build whose version is `dev` or `0.31.0+dirty`
- **THEN** the section says the build is not a tagged release and lists the newest entries, none marked as running

### Requirement: Changelog entries render without links below the update card

Entries SHALL render through the sanitising Markdown renderer with every link removed. When a newer
release exists, the update card and the upgrade commands SHALL stay above the section.

#### Scenario: Links and raw HTML

- **WHEN** an entry carries Markdown links, commit hashes or raw HTML
- **THEN** the link text is shown without its target and no raw HTML reaches the page

### Requirement: Feed notes only for a newer release

The release screen SHALL show the newest published release's notes only when that release is newer
than the running version, because otherwise they repeat the running version's changelog entry.

#### Scenario: Up to date

- **WHEN** the newest published release is the running version
- **THEN** the feed notes section is not shown

### Requirement: The browser sets housekeeping

The Tenant screen SHALL carry a Housekeeping section setting the tenant defaults, and each project's board SHALL
carry a Housekeeping panel setting the overrides as inherit, off or a number of days. Both SHALL say how many
tasks each rule would act on now, and that the rules are paused while the trash is off.

#### Scenario: The count is shown

- **GIVEN** a done rule of 30 days and two tasks done 40 days ago
- **WHEN** an admin opens the Tenant screen
- **THEN** the Housekeeping section says 2 tasks would be moved to the trash now

### Requirement: The browser sets and explains lease length

The task screen SHALL show a Lease row a writer can edit, and under it how long a claim would last and which
level decided it. Each project's board SHALL carry a Claims panel setting the project's lease length and longest
lease, blank to inherit.

#### Scenario: Saving a project's lease length

- **WHEN** a writer saves 2h as the lease length in a project's Claims panel
- **THEN** the project's lease default is 2h, and a task in it with no lease of its own says claims last 2h, from the project

### Requirement: The sidebar is ordered by use

The sidebar SHALL draw, from the top: the tenant's name with the rail toggle, Search, then Inbox, My
work, Tasks, Projects, Activity and Statistics as top-level entries in that order, then the reader's
pinned projects under Projects, then Saved filters and Recently viewed, then the Admin group, then the
account menu and the version. The tenant's name SHALL be plain text, since a session belongs to one
tenant.

#### Scenario: Entries in order of use

- **GIVEN** a signed-in reader
- **WHEN** any screen is drawn
- **THEN** the navigation lists Inbox, My work, Tasks, Projects, Activity and Statistics in that order, each at the top level

### Requirement: My work is a top-level entry counting overdue assigned tasks

My work SHALL be a top-level entry, not indented under another. My work SHALL carry a badge counting
the reader's open assigned tasks that are overdue, hidden at zero and reading 99+ past 99, rendered by
the server and refreshed by the same live stream as the unread badge.

#### Scenario: Overdue work is counted on My work

- **GIVEN** a reader assigned two open tasks past their deadline and one finished task past its deadline
- **WHEN** any screen is drawn
- **THEN** My work's badge reads 2 and names them as overdue tasks for a screen reader

#### Scenario: Nothing overdue, no badge

- **GIVEN** a reader with no overdue open tasks
- **WHEN** any screen is drawn
- **THEN** My work's badge is hidden

### Requirement: Projects can be pinned to the sidebar

A reader SHALL be able to pin a project to the sidebar from the projects screen and unpin it there.
Pinning SHALL be the reader's own star on the project's lane, so a starred lane is pinned and a pinned
project's lane is starred.

#### Scenario: Pinning a project

- **GIVEN** a reader on the projects screen
- **WHEN** they press Pin to sidebar on a project
- **THEN** the project is listed under Projects in the sidebar, linking to its board, and its row offers Unpin

#### Scenario: A starred lane is pinned

- **GIVEN** a reader who starred a project's lane from the lane menu
- **WHEN** any screen is drawn
- **THEN** the project is listed under Projects

#### Scenario: Another reader's pins are their own

- **GIVEN** a project pinned by one reader
- **WHEN** another reader opens any screen
- **THEN** that reader's sidebar does not list it

### Requirement: Pinned projects are listed with their mark and label

Pinned projects SHALL be listed under Projects, each a link to the project's board showing its icon, or
the first letter of its key, in its colour, named by the reader's project label preference with the key
upper-cased. An archived project, or one the reader cannot read, SHALL be left out. On the rail each
SHALL show its mark and keep its name as its accessible name.

#### Scenario: A pinned project follows the label preference

- **GIVEN** a reader who pinned project infra and chose to label projects by key
- **WHEN** any screen is drawn
- **THEN** the sidebar lists it as INFRA, marked with the letter I and linking to its board

### Requirement: The administration screens fold under one Admin group

Trash and the screens the Configure and Data groups held SHALL be one group, Admin, folded by default.
Trash SHALL be listed only while the tenant's trash is on; the configuration screens only with Advanced
screens on; with neither, no Admin group SHALL be drawn. A folded Admin group SHALL still show the entry
for the page being read. A `tix_sidebar` cookie written before this change that folded both Configure
and Data SHALL fold Admin, and one that left either open SHALL open it.

#### Scenario: Admin starts folded

- **GIVEN** a browser with no sidebar cookie
- **WHEN** any screen is drawn
- **THEN** Admin is drawn folded

#### Scenario: A browser that had Configure open

- **GIVEN** a sidebar cookie from the previous build folding Data but not Configure
- **WHEN** any screen is drawn
- **THEN** Admin is drawn open

#### Scenario: A browser that had both folded

- **GIVEN** a sidebar cookie from the previous build folding Configure and Data
- **WHEN** any screen is drawn
- **THEN** Admin is drawn folded

#### Scenario: Trash off

- **GIVEN** a tenant whose trash is turned off
- **WHEN** any screen is drawn
- **THEN** Admin has no Trash entry

### Requirement: One account menu at the foot of the sidebar

The sidebar SHALL end with one control showing the reader's initials and handle, opening a menu that
holds Settings, the colour scheme switch, Keyboard shortcuts and Sign out, with the version link under
the control. On the rail the control SHALL show the initials and keep the handle as its name.

#### Scenario: Opening the account menu

- **GIVEN** a signed-in reader
- **WHEN** they press their name at the foot of the sidebar
- **THEN** a menu offers Settings, the colour scheme, Keyboard shortcuts and Sign out

### Requirement: The account menu opens without scripting and closes alone on Escape

The menu SHALL open and close without scripting, Escape SHALL close it alone, including inside the
phone menu, and focus SHALL return to its control. Keyboard shortcuts SHALL be shown only with
scripting.

#### Scenario: Escape inside the phone menu

- **GIVEN** a reader on a phone with the menu open and the account menu open inside it
- **WHEN** they press Escape
- **THEN** the account menu closes, the phone menu stays open, and focus is on the account control

### Requirement: A board cleans up its own project now

A project's Housekeeping panel SHALL offer Clean up now to a tenant admin, which runs housekeeping once for
that project only, after a confirmation that names how many of its tasks would go to the trash and how many
would be deleted for good. It SHALL be disabled, with the reason, while nothing is due or the trash is off.
After the run the board SHALL say how many tasks were moved to the trash and how many deleted for good.

#### Scenario: Only the project is cleaned up

- **GIVEN** a done rule of 1 day and a task done 2 days ago in each of INFRA and OPS
- **WHEN** an admin confirms Clean up now on the INFRA board
- **THEN** the INFRA task is in the trash, the OPS task is not, and the board says 1 was moved to the trash

#### Scenario: A run needs the confirmation

- **WHEN** a post to the project's cleanup run arrives without the confirmation
- **THEN** nothing is moved and the request is refused

#### Scenario: Nothing to do

- **GIVEN** no task is due, or the trash is off
- **WHEN** an admin opens the board's Housekeeping panel
- **THEN** Clean up now is disabled and says why

#### Scenario: A member cannot run it

- **WHEN** a member who is not a tenant admin posts to the project's cleanup run
- **THEN** the request is refused as forbidden and nothing is moved

### Requirement: The new task form sets a lease

The browser's Add with details form SHALL carry a Lease field under More options, blank to inherit. A lease above
the maximum SHALL draw the form again with the refusal on that field and create nothing.

#### Scenario: Adding a task with a lease in the browser

- **WHEN** a writer creates a task with 2h in the Lease field
- **THEN** the task screen's Lease row holds 2h and says claims last 2h, set on this task

### Requirement: A preference among named positions is a segmented control

A preference choosing between two or more named positions SHALL be drawn as a segmented control
naming every position, the current one marked with `aria-current`, each position a submit button
that works without scripting, and SHALL NOT be drawn as a switch.

#### Scenario: Two named positions are a segmented control

- **WHEN** the settings screen is drawn
- **THEN** the Task opens in row offers Page and Dialog as two submit buttons and no switch
- **AND** the Clicking a title row offers Open the task and Edit the title as two submit buttons and
  no switch

### Requirement: A post to Task opens in stores the position it names

A post naming a position SHALL store it; a post naming none SHALL move to the other position.

#### Scenario: Pressing the current position keeps it

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader presses Dialog again
- **THEN** the browser still stores `tix_task_open=dialog`

### Requirement: The database target is for administrators

The Settings screen SHALL draw its Database section, and the section navigation's link to it, only for
a reader holding `tenant:admin`. The command palette SHALL offer the Database entry to the same
readers only. The target SHALL be set at body size and SHALL break anywhere rather than widen the page.

#### Scenario: A member does not see the target

- **GIVEN** a server started with a target description
- **WHEN** a reader without `tenant:admin` opens Settings
- **THEN** the page carries no Database section, no link to it and no part of the target

#### Scenario: An administrator does

- **GIVEN** a server started with a target description
- **WHEN** a tenant administrator opens Settings
- **THEN** the Database section shows the target and the section navigation links to it

### Requirement: The calendar feed says where its address went

When the reader has a calendar feed, the Calendar feed section's first row SHALL state that the feed is
Active once, SHALL say when it was created, and SHALL say that its address is shown only once, when the
feed is created or rotated, so a new address comes from Rotate.

#### Scenario: An active feed explains its address

- **GIVEN** a reader with a calendar feed
- **WHEN** they open Settings
- **THEN** the Calendar feed section's first row says Active once, names the creation date and says
  the address is shown once and Rotate issues a new one

### Requirement: Settings sections share one shape

Every Settings section SHALL be a heading followed by its card of rows. A section with a description
SHALL draw it as the card's first row. No section heading SHALL carry an information disclosure; each
row SHALL carry its own. Existing notification channels SHALL be rows in a card, and the form adding a
channel SHALL be folded behind an Add disclosure that opens by itself when the form was refused.

#### Scenario: Descriptions sit in the card

- **WHEN** the settings screen is drawn
- **THEN** the Inbox and Notifications descriptions are inside their sections' cards
- **AND** no section heading has an information disclosure

#### Scenario: Add a channel is folded

- **WHEN** a reader with one channel opens Settings
- **THEN** the channel is a row in a card and Add a channel is a closed disclosure

### Requirement: An information note pushes the page down

An information disclosure beside a setting's name, a field's label or a heading SHALL, when opened,
draw its note in the flow under that line, moving what follows down, rather than over the content
below it.

#### Scenario: Opening a note on a phone

- **GIVEN** a viewport 390px wide
- **WHEN** the reader opens the (i) beside Colour scheme on Settings
- **THEN** the note is drawn under the name and the Colour rows by row starts below the note

### Requirement: Placeholders read as hints

A placeholder SHALL be drawn in the muted colour, in italics and in the body font, including in a
monospace field, so it cannot be mistaken for a value. A placeholder giving an example value SHALL say
it is an example.

#### Scenario: The feed filter's placeholder is an example

- **WHEN** a reader opens the Calendar feed section
- **THEN** the filter field's placeholder begins "For example"

### Requirement: Touch-only devices are not offered hover and keyboard settings

On a device whose primary input cannot hover and is a coarse pointer, the Settings screen SHALL NOT draw the
Hover effects row or the Keyboard section and its navigation link, and the account menu's Keyboard
shortcuts entry, the command palette's key hints and the create forms' key line SHALL NOT be drawn. A
device whose primary input is a fine, hovering pointer SHALL keep them.

#### Scenario: A phone drops them

- **GIVEN** a touch-only phone
- **WHEN** the reader opens Settings
- **THEN** neither Hover effects nor the Keyboard section is drawn

#### Scenario: A touch laptop keeps them

- **GIVEN** a laptop with a touch screen, whose primary input is its trackpad
- **WHEN** the reader opens Settings
- **THEN** Hover effects and the Keyboard section are drawn

### Requirement: A toolbar note flows on a narrow or touch-only screen

On a viewport narrower than 900px, or on a device whose primary input cannot hover and is a coarse
pointer, an information disclosure in a toolbar, such as the view switch or the task screen's header,
SHALL open its note in the flow under the toolbar rather than over the content below it. A wider
screen with a hovering pointer MAY keep the note floating.

#### Scenario: Opening the Views note on a phone

- **GIVEN** a viewport 390px wide
- **WHEN** the reader opens the (i) beside the view switch on Tasks
- **THEN** the note is drawn under the toolbar and the listing starts below it

### Requirement: Touch-only devices are not offered drag to move

On a device whose primary input cannot hover and is a coarse pointer, a board card SHALL NOT be
draggable or show a grab cursor, and Settings SHALL NOT draw the Drag to move row. Every card's Move
control SHALL stay available.

#### Scenario: A board on a phone

- **GIVEN** a touch-only phone and Drag to move on
- **WHEN** the reader opens a board
- **THEN** a long press does not lift a card and each card still offers its Move control

### Requirement: The palette offers nothing a touch-only device hides

On a device whose primary input cannot hover and is a coarse pointer, the command palette SHALL NOT
offer the Keyboard section of Settings or the Keyboard shortcuts action, nor any other entry pointing
at something that device does not draw.

#### Scenario: Typing keyboard on a phone

- **GIVEN** a touch-only phone
- **WHEN** the reader opens the palette and types "keyboard"
- **THEN** neither the Keyboard section nor Keyboard shortcuts is offered

### Requirement: A create action is a header button that opens a dialog

A screen whose action makes a record SHALL offer it as a primary button in the page header, or on the
right of the heading of the section it adds to, and SHALL NOT fold the form at the foot of the screen.
The button SHALL open the form in a modal dialog with a Cancel control. An empty list SHALL carry the
same button.

#### Scenario: Issuing a token starts from the header

- **WHEN** the token screen is read
- **THEN** its header holds an Issue a token button
- **AND** the form it opens is in a dialog, not under the list

### Requirement: A create dialog works without scripting and after a refusal

Without scripting a create button SHALL be a link to the same screen with the dialog drawn open in
place. A submission the service refuses as correctable SHALL come back to that screen with the dialog
open, the message inside it beside its field and the values typed kept.

#### Scenario: A refused user comes back in its dialog

- **WHEN** a user is created with an invalid email
- **THEN** the users screen is drawn with the Create a user dialog open
- **AND** the message and the typed display name are inside it

#### Scenario: The fallback address draws the form

- **WHEN** the token screen is read at `?dialog=token-new`
- **THEN** the Issue a token form is drawn open in place

### Requirement: A project's settings are a screen of their own

A project's settings SHALL be the screen `/projects/{key}/settings`, reached from a Settings button in
the board's header, holding Project, Claims, Housekeeping and, last, Danger zone as always-drawn headed
sections. The board SHALL hold the columns and none of these forms.

#### Scenario: The board links to its settings

- **WHEN** a project board is read
- **THEN** its header carries a Settings button to `/projects/{key}/settings`
- **AND** the board carries no settings, claims, housekeeping or archive form

### Requirement: A destructive action is a red button and a confirm dialog

An action that cannot be undone SHALL be a red button that opens a dialog saying what happens, and the
form that takes the step SHALL be only in that dialog. Revoking a token, deleting a user or a template,
removing a notification channel, a project's Clean up now and archiving a project SHALL work this way.

#### Scenario: Deleting a template asks first

- **WHEN** a template's screen is read
- **THEN** Delete template is a red button opening a dialog
- **AND** the delete form is inside the dialog and nowhere else

### Requirement: No page action, setting or explanation is folded

A template SHALL NOT draw a page action, a page's settings or an explanation inside a `<details>`
element. A toggle on one row or one field MAY stay one, and a test SHALL name each such toggle with
its reason.

#### Scenario: A new fold fails the build

- **WHEN** a template adds a `<details>` that is not a listed toggle
- **THEN** the template guard fails

### Requirement: The source form carries the settings

The web form that configures a sync source SHALL offer the mapping path, url, project, query, file and page
size, store those filled in, and the Sources table SHALL show each source's stored settings.

#### Scenario: Saving a source with a url

- **WHEN** a jira source is saved from the form with a url
- **THEN** the source stores the url and its row in the Sources table shows it

### Requirement: Task history pages back to its first change

The task screen's History SHALL draw the newest changes, at most one page at the browser's configured
page size, and SHALL draw a Show older control under them while older changes exist. Following Show older
until it is gone SHALL draw every change exactly once, newest first. The last page SHALL NOT offer Show
older.

#### Scenario: A long history is walked to the start

- **GIVEN** a task with 120 audit entries and a page size of 25
- **WHEN** the reader opens the task and follows Show older until it is gone
- **THEN** all 120 changes have been drawn, newest first, none twice

#### Scenario: A history ending on a page boundary

- **GIVEN** a task with exactly two pages of audit entries
- **WHEN** the reader follows Show older once
- **THEN** the second page is drawn without a Show older control

### Requirement: Show older works with and without JavaScript

Show older SHALL be a link to the task screen at the older page, which opens History and offers a way
back to the newest changes. With JavaScript it SHALL fetch only the older rows and append them in place
of itself. A reader without the audit read scope SHALL be told so rather than handed rows.

#### Scenario: Without JavaScript

- **WHEN** a browser without JavaScript follows Show older
- **THEN** the task screen loads with History open on the older page and a Newest changes link

#### Scenario: A reader without audit scope

- **WHEN** a reader who may read the task but not the audit log fetches older history rows
- **THEN** the answer says they do not hold the audit read scope and carries no rows

### Requirement: A move is not split across history pages

A move that crosses several workflow states SHALL be drawn as one row even when its hops fall either side
of a page boundary: the older hops SHALL join the page holding the newer ones.

#### Scenario: A two-hop move on the boundary

- **GIVEN** a task whose 25th and 26th newest entries are the two hops of one completion
- **WHEN** the reader opens the task
- **THEN** the first page draws the completion as one row of two steps
- **AND** the next page draws neither hop

### Requirement: A parsed field shows what it takes before it is typed in

The due date, repeat and quick-add fields SHALL carry an (i) tip listing a few values to type, drawn from
the parser's own lists, so the syntax is visible before a refusal. A due date example SHALL be written in
the reader's date format. Every example shown SHALL be accepted by the parser that reads the field.

#### Scenario: Due examples in the reader's format

- **GIVEN** a reader whose date format is Dotted
- **WHEN** they open the due date tip on Add with details
- **THEN** it lists `tomorrow`, `fri`, `+3d`, `+2w` and `20.11.2026`

#### Scenario: Repeat examples

- **WHEN** a reader opens the repeat tip on the task screen
- **THEN** it lists rules such as `daily`, `every 2 weeks` and `every month on day 15`, each one a rule the parser accepts

### Requirement: A due date field takes words and the reader's own dates

The due date fields on Add with details and the task screen SHALL be text fields that take a word such as
`tomorrow` or `fri`, an offset such as `+3d`, `YYYY-MM-DD`, or a date in the reader's date format, and SHALL
show a saved due date in that format, so saving the field unchanged keeps the day.

#### Scenario: The placeholder shows examples

- **GIVEN** a reader whose date format is Dotted
- **WHEN** the due date field is empty
- **THEN** its placeholder reads `tomorrow, +3d or 20.11.2026`

#### Scenario: A dotted date round-trips

- **GIVEN** a reader whose date format is Dotted
- **WHEN** they save `06.11.2026` in the task screen's Due field
- **THEN** the task falls due on 6 November 2026 and the field shows `06.11.2026`

### Requirement: A due date field keeps a calendar

Each typed due date field SHALL carry a calendar button with an accessible label that opens the browser's
date picker and writes the picked day into the field in the reader's date format, or YYYY-MM-DD for a
format that writes the month as a word, so the server reads it back as the same day. Without scripting the
button SHALL NOT be drawn.

#### Scenario: Picking a day under Dotted

- **GIVEN** a reader whose date format is Dotted
- **WHEN** they pick 6 November 2026 from the calendar beside the due date field
- **THEN** the field reads `06.11.2026` and saving it sets that day

### Requirement: Every screen state has a way back

Every state a screen can be drawn in, including a result, a preview and a refusal, SHALL show a way
back to where the reader came from, and that way back SHALL lead to an address that draws a screen.

#### Scenario: A sync run's result

- **WHEN** a reader runs a sync source from the sync screen
- **THEN** the result is drawn at the top of the sync screen with a link back to the sources

#### Scenario: A deletion preview

- **WHEN** a reader previews where a deleted project's tasks would move
- **THEN** the confirmation still offers Cancel and the back link to the board

### Requirement: A failed action with no screen leads back to the screen it was posted from

When a mutation whose address draws no screen fails on the error screen, the way back SHALL lead to
the same-origin screen that posted it, or to the task list when there is none, and never to the
action's own address.

#### Scenario: A sync run refused for its mapping file

- **GIVEN** a sync source whose mapping file does not exist
- **WHEN** the reader runs it
- **THEN** the sync screen shows why the run was refused with a link back to the sources
- **AND** no way back leads to `/sync/run`

### Requirement: A result posted to an action is shown at its screen's address

A result or a refused dialog drawn in answer to a POST to an action SHALL ask the browser to show the
address of the screen it is drawn on, so reloading it or going back to it does not post again.

#### Scenario: Reloading a sync result

- **WHEN** a reader with scripting enabled reloads the page after a sync run
- **THEN** the browser loads `/sync` and the run is not repeated

### Requirement: The deletion preview describes the target the picker shows

The project deletion screen SHALL select, in its target picker, the project its status mapping
describes, whatever case the target was named in.

#### Scenario: A target named in capitals

- **WHEN** the deletion screen is opened with `move_to` naming a project key in capitals
- **THEN** the picker selects that project and the mapping describes it

### Requirement: Closing a dialog leaves no mark behind

A dialog's opener SHALL name an id no other element on the screen carries, so the button opens the
dialog rather than following its link. Closing the dialog SHALL take both its `dialog` parameter and a
fragment naming it out of the address. No row SHALL stay outlined or tinted after the dialog closes.

#### Scenario: Manage on the user listing

- **WHEN** the reader clicks Manage on a user's row and then closes the dialog
- **THEN** the address carries neither the dialog nor a fragment, and the row looks as it did before

### Requirement: A row a link lands on flashes once

The row a fragment, a deep link or a toast names SHALL be lit with a brief tint that fades by itself,
with or without scripting. It SHALL NOT keep an outline or a tint after the flash ends.

#### Scenario: A link from the activity feed

- **WHEN** the reader follows a link to `/admin/users#user-ID`
- **THEN** that row flashes and then returns to its resting look

### Requirement: Every table row takes the same states

Every table and every list drawn as one SHALL carry the class `rows`. Its rows SHALL take one tint
under the pointer and while a control in them has keyboard focus, one colour when picked or focused by
the keyboard cursor, and no other hover border, bar, shadow, movement or cursor, in every theme.

#### Scenario: Hovering rows on different screens

- **WHEN** the reader points at a row of the user listing, the members table, the task list and the project list
- **THEN** each takes the same background tint and nothing else about it changes

### Requirement: A pressed button is pending at once without hiding behind its spinner

A submit button, or a button that makes its own request, SHALL be marked pending in the frame it
is pressed, with no transition. Its spinner SHALL take the place of its icon with the label still
readable, or of its label when it has no icon, and its width SHALL NOT change. A second press
while pending SHALL submit nothing, and the mark SHALL clear when the request ends.

#### Scenario: Pressing Add

- **WHEN** the reader presses a submit button whose request is slow
- **THEN** in the same frame it is pending, its spinner overlaps no visible text, its width is unchanged, and pressing it again sends nothing

#### Scenario: The busy pointer waits

- **WHEN** a form's request answers within 300ms
- **THEN** the pointer never changes to progress

### Requirement: No screen scrolls sideways

No screen of the web interface SHALL be wider than the window it is drawn in, at any viewport and
whatever the names, titles, tags or descriptions it shows.

#### Scenario: A long name on a phone

- **GIVEN** a user and a project whose names hold a 50 character word
- **WHEN** a screen showing them is drawn at 390px
- **THEN** `document.scrollingElement.scrollWidth` is at most the window's width

### Requirement: Content stays inside its box

Text or an image SHALL NOT pass the right edge of a box that neither clips nor scrolls its overflow
by more than 3px. A box that clips or scrolls has chosen how its overflow is shown.

#### Scenario: An unbroken token in a row

- **GIVEN** a task whose title and tags hold unbroken 60 character tokens
- **WHEN** a listing or board shows it
- **THEN** no text of the task runs past the edge of its row or card

### Requirement: Times are machine-readable and never raw

Every `<time>` element SHALL carry a `datetime` attribute holding a real time, and no visible text or
tooltip SHALL show Go's default time formatting or its zero time.

#### Scenario: A timestamp on a screen

- **WHEN** any screen shows a time
- **THEN** its `<time>` has a `datetime`, and neither its text nor its title holds `0001-01-01`,
  `+0000 UTC` or `m=+`

### Requirement: The global admin area

The browser SHALL offer a Global admin entry only to a global admin. Its screen SHALL list every tenant
with create, rename and delete in dialogs, the global admins with grant and revoke, and link each tenant to
a screen managing its users.

#### Scenario: A tenant admin's sidebar

- **WHEN** a tenant admin who is not a global admin opens any screen
- **THEN** the sidebar has no Global admin entry and `/admin/global` answers 403

### Requirement: Generated passwords are shown once

After a tenant is created from the browser, the next load of the global admin screen SHALL show each
generated password with its email and a copy button, and a reload SHALL show none.

#### Scenario: Reloading

- **WHEN** the screen showing the passwords is loaded again
- **THEN** none of the passwords appears

### Requirement: An open info tip stays inside the window

An open info tip's note SHALL lie inside the viewport at every width: a note that would open past an
edge SHALL be moved, and narrowed if the viewport is narrower than it, so the page does not scroll
sideways. Without scripting the note SHALL still open where the stylesheet places it.

#### Scenario: A tip at the right edge of a wide screen

- **GIVEN** a viewport 1920px wide
- **WHEN** the reader opens the tip beside the My work heading
- **THEN** the note is inside the viewport and the page is no wider than the viewport

### Requirement: Long names and tags do not widen a screen

A project name, a user name in an activity line and a tag on a board or lane card SHALL fit their row
or card at every width: a name SHALL wrap, breaking inside a word longer than its line, and a card's
tag SHALL end in an ellipsis with its full name as its title.

#### Scenario: A long project name on a phone

- **GIVEN** a viewport 390px wide and a project named with a 60-letter word
- **WHEN** the projects screen is drawn
- **THEN** the page is no wider than the viewport and the row's name does not run under its links

### Requirement: A screen for connecting an agent

The browser interface SHALL serve a Connect an agent screen at `/connect` to every signed-in reader,
reachable from the main menu and from the API tokens screen. It SHALL show commands for installing the
CLI, issuing a token, configuring a CLI context, registering `tix mcp` with each client docs/mcp.md
names, installing the agent skill and calling the REST API, each with a copy button.

#### Scenario: Commands name this server

- **GIVEN** a reader signed in to tenant `acme` at `https://tix.example.com`
- **WHEN** they open `/connect`
- **THEN** the CLI context command reads `tix ctx add acme --server https://tix.example.com` with the
  reader's tenant key, and no command carries the placeholder `URL`

### Requirement: Issuing a token on the connect screen

The connect screen SHALL issue an API token for the reader from a preset scope bundle or a custom set,
offering only scopes the reader holds, and SHALL show the value once. Its script SHALL write the value
into every command on the screen without storing it or placing it in a URL. Before a token is issued
the commands SHALL read `$TIX_TOKEN`.

#### Scenario: The token fills every command

- **GIVEN** a reader holding `token:admin`
- **WHEN** they issue a token with the agent preset
- **THEN** the screen shows the value once, and every token position in the commands holds it

#### Scenario: A reader who cannot issue tokens

- **GIVEN** a reader without `token:admin`
- **WHEN** they open `/connect`
- **THEN** no issuing form is drawn and the commands read `$TIX_TOKEN`

### Requirement: The SSH section follows the listener

The connect screen SHALL show the SSH commands only when the serving process runs an SSH listener.
The address SHALL be `ssh.public_address` when it is set, otherwise the request's host name and the
port the listener binds.

#### Scenario: A container publishes another port

- **GIVEN** `tix serve --ssh-listen 0.0.0.0:2222` with `ssh.public_address` set to `tix.example.com:2020`
- **WHEN** a reader in tenant `acme` opens `/connect`
- **THEN** the screen shows `ssh -p 2020 acme@tix.example.com`

### Requirement: Documentation is a menu entry

The main menu SHALL carry a Docs entry, opening in a new tab, and each section of the connect screen
SHALL link to its documentation page. Both SHALL go through `/docs/{page}`, which redirects to that
page for the running version and SHALL refuse a page that is not in the documentation.

#### Scenario: An unknown page is not redirected

- **WHEN** a client asks for `/docs/evil.md`
- **THEN** the answer is 404 and no redirect is issued

### Requirement: One scope picker for every token form

Every browser token form SHALL draw each scope as a labelled checkbox in a group with a note on what it
allows. A scope the reader does not hold SHALL be drawn disabled with the reason. Select all SHALL tick
each scope the reader holds and never `*`, which SHALL be a separate choice offered only to a reader
holding it. The issued token's confirmation SHALL list the scopes it was granted.

#### Scenario: A scope the reader lacks

- **GIVEN** a reader holding `token:admin` and `task:read` only
- **WHEN** they open the token form and submit `task:write` anyway
- **THEN** the `task:write` box is disabled with its reason, and the submission is refused with 403

### Requirement: Create a user by password or by invite link

The Users screen's create dialog SHALL offer a choice between Set a password and Send an invite
link, both visible controls rather than a fold. With a password, a Require a password change at
first sign-in checkbox SHALL be ticked by default. An invite SHALL answer with a dialog showing the
link once with a Copy button and the words send this link to them.

#### Scenario: Inviting from the browser

- **WHEN** an administrator chooses Send an invite link and submits an email
- **THEN** a dialog shows the link with a Copy button, and the Pending invites section lists the address

### Requirement: Pending invites on the Users screen

The Users screen SHALL list pending invites with their address, role, inviter and expiry, each with
a Re-issue button and a red Revoke button opening a confirm dialog. The Manage dialog SHALL carry the
require password change checkbox.

#### Scenario: Revoking from the list

- **WHEN** an administrator confirms Revoke on a pending invite
- **THEN** the invite leaves the list and its link stops working

### Requirement: Invite and password change pages

`/invite/TOKEN` SHALL show a form to choose a password and a display name, or to confirm an existing
password, and sign the person in. A flagged sign-in SHALL show a Choose a new password page whose
only other action returns to sign-in; nothing else is reachable because no session exists.

#### Scenario: Forced change in the browser

- **GIVEN** a flagged account
- **WHEN** its owner signs in on the web
- **THEN** the Choose a new password page is shown, and opening the projects list sends them to sign-in

### Requirement: Date preferences are kept with the account

The settings screen SHALL save the date format and timezone on the person's account, offer Inherit for each,
and say what is in use and where it came from. The Tenant screen SHALL offer the tenant default the same way,
its Inherit naming the deployment's value.

#### Scenario: Another browser reads the choice

- **GIVEN** a person who chose `us` in one browser
- **WHEN** they open the settings screen in another browser
- **THEN** `us` is selected there and their timestamps read `us`

### Requirement: Legacy date cookies move onto the account

A request carrying the `tix_time_format` or `tix_timezone` cookie SHALL move each valid value onto the
account when the account holds none for that field, and SHALL expire both cookies.

#### Scenario: The account wins over a cookie

- **GIVEN** an account format of `long` and a cookie format of `us` with a cookie zone of `Asia/Tokyo`
- **WHEN** the browser opens any screen
- **THEN** the account keeps `long`, gains `Asia/Tokyo`, and both cookies are gone

### Requirement: Every timestamp carries its full instant

Every timestamp a screen shows as text SHALL be a `<time>` whose `datetime` is RFC 3339, whose text is in the
reader's style, and whose title, accessible label and focus tip carry the instant with seconds, zone name
and offset, plus the relative form when the text is absolute. It SHALL take focus, so a tap or Tab shows it.

#### Scenario: Activity row

- **WHEN** a reader in `dotted`, `Europe/Sofia` focuses "2 hours ago" in the activity feed
- **THEN** a tip reads like `06.11.2026 14:05:32 EET (UTC+02:00)`

### Requirement: A guard holds every screen to the reader's format

A test SHALL render every screen in the screenshot manifest on a seeded database as a reader in `dotted`
and `Asia/Tokyo`, and fail on a timestamp in another layout, a timestamp written as text outside `<time>`,
or a `<time>` missing part of its tooltip.

#### Scenario: A raw field fails the guard

- **WHEN** a template prints `{{.CreatedAt}}` or `{{compact .CreatedAt}}` as text
- **THEN** the guard fails naming the screen and the value

### Requirement: Activity and History show the channel

Each Activity row and each task History row SHALL show its access channel as an icon and words such as via SSH, via token ci-agent or via MCP (token scout-runner), with every recorded detail in its hover and its raw view.

#### Scenario: A token's row

- **WHEN** the feed shows a change made with the ci-agent token
- **THEN** the row reads via token ci-agent and its hover names the client address

#### Scenario: A task's history

- **WHEN** a task's History is shown
- **THEN** its Channel column shows each change's channel

### Requirement: Activity filters by channel and token

The Activity filter SHALL offer a channel control and a token box carried in the query string as via and token, each shown back as a removable chip, and each row's channel SHALL link to the feed narrowed to that channel and token.

#### Scenario: Narrowing to SSH

- **WHEN** the reader picks SSH in the channel control
- **THEN** only changes made over SSH are listed and a via SSH chip clears it

### Requirement: Project export dialog

A project's page SHALL offer an Export button in its header to readers holding the export scope, opening a
dialog with the format, date style, CSV separator, filter, inclusion switches and an ordered list of
columns, each column with a move up and a move down button, and SHALL download the file on submit.

#### Scenario: Exporting from the board

- **WHEN** a member opens Export on a project, ticks title and ref, and submits
- **THEN** the browser downloads the project's tasks with those columns

#### Scenario: Viewer

- **WHEN** a viewer opens a project's page
- **THEN** there is no Export button

### Requirement: Refused export keeps the dialog

A refused export from the dialog SHALL draw the project again with the dialog open, every choice as
submitted, and the reason beside the field it concerns.

#### Scenario: Filter that does not parse

- **WHEN** a reader submits the export dialog with a filter that does not parse
- **THEN** the dialog is open again holding that filter and the refusal

### Requirement: Import steps

A project's page SHALL offer Import to readers holding the import scope, leading through upload, column
mapping, value mapping, preview and result, each step at its own address with a Back that keeps what was
chosen and a Cancel that leaves the import.

#### Scenario: Back keeps the mapping

- **WHEN** a reader changes a column's field, goes on to the values and presses Back
- **THEN** the columns step shows the field they chose

### Requirement: The simple paths are offered first

The first import step SHALL say that a few tasks need no import and that the mapping is optional, SHALL
take a pasted list of quick-add lines, and the steps SHALL offer Skip to preview, said outright when every
column was recognised.

#### Scenario: Every column recognised

- **WHEN** a file whose every header is recognised is uploaded
- **THEN** the columns step says so and offers Skip to preview

### Requirement: A history row's raw entry opens in a dialog

The Raw control on an activity row and on a task's History row SHALL open a modal dialog titled Raw entry,
naming the entry's action, subject and time, with a close control. Escape SHALL close it and return focus to
the control. It SHALL NOT open a disclosure or popover inside the row.

#### Scenario: Raw opens a dialog

- **WHEN** a reader presses Raw on an activity row
- **THEN** a modal Raw entry dialog opens at least as wide as the window allows up to 56rem
- **AND** Escape closes it and focus is back on Raw

### Requirement: The raw entry dialog shows the recorded entry readably

The dialog SHALL show each entry's actor, action, subject, full timestamp, source and sequence, and each
recorded snapshot as indented JSON with markup shown as text, each with a Copy button and one Copy all as
JSON button. Long values SHALL wrap inside the dialog.

#### Scenario: A hostile title reads as text

- **WHEN** a task titled with a script tag and a long unbroken word is retitled and its raw entry opened
- **THEN** the before snapshot shows the tag as text, indented, with nothing scrolling sideways

### Requirement: The raw entry dialog sets changed fields side by side

When an entry has both a before and an after snapshot, the dialog SHALL list the top-level fields that were
added, removed or changed, each marked with which, with its value on each side.

#### Scenario: A retitle lists the title

- **WHEN** a task's title and priority are changed and the entry's raw view opened
- **THEN** title and priority are listed as changed with the old and new values

### Requirement: The raw entry dialog works without scripting

Without scripting the Raw control SHALL be a link to the same screen with the dialog drawn open in place,
whose close control leads back to the screen without it.

#### Scenario: The fallback address draws the dialog

- **WHEN** the activity feed is read at `?dialog=raw-SEQ` for a row on it
- **THEN** that row's Raw entry dialog is drawn open, with a Close link back to `/activity`

### Requirement: A help button on every screen

Every screen of the browser interface SHALL carry a help link at the right end of its header row,
named "Help: " followed by the screen's title and opening in a new tab with `rel="noopener"`. It
SHALL lead through `/docs/{page}` to the documentation section about that screen, for the running
release. The task screen drawn in the task dialog SHALL carry none.

#### Scenario: The task list's help

- **GIVEN** a reader on `/tasks`
- **WHEN** they follow the help link in the header
- **THEN** it opens `/docs/web-ui.md?section=the-task-list` in a new tab, which redirects to that page
  of the running release with `#the-task-list`

### Requirement: The docs redirect carries a section

`GET /docs/{page}` SHALL accept a `section` query parameter and SHALL append it to the redirect target
as a fragment when it consists of lower-case letters, digits, hyphens and underscores. Any other value
SHALL be ignored, and the redirect SHALL go to the page alone.

#### Scenario: A malformed section is dropped

- **GIVEN** a request for `/docs/web-ui.md?section=Bad%20Section`
- **WHEN** the server answers
- **THEN** it redirects to the page with no fragment

### Requirement: A task's activity is tabs with counts

The task screen and the task dialog SHALL show Comments, History and Artifacts as a tab strip under the
task's dependencies, each tab naming its count and a zero count shown muted. One panel SHALL show at a
time, the tabs SHALL follow the ARIA tabs pattern with arrow keys, Home and End, and none SHALL sit in
an expander.

#### Scenario: Switching with the keyboard

- **GIVEN** a task screen showing the Comments tab
- **WHEN** the reader focuses it and presses the right arrow
- **THEN** the History tab is selected and focused and only its panel is shown

### Requirement: The activity tab chosen is kept and addressable

The browser SHALL remember the activity tab a reader last chose. A fragment naming a panel or a record
inside one, an older page of history, or a raw entry in the address SHALL open the tab that holds it
instead. A live update SHALL redraw the lists and counts and keep the tab shown.

#### Scenario: A history link opens History

- **GIVEN** a reader whose last chosen tab was Artifacts
- **WHEN** they follow a link to the task ending in `#history`
- **THEN** the History tab is the one shown

#### Scenario: A comment arrives while History is shown

- **WHEN** a live update brings a new comment
- **THEN** the Comments count goes up and History stays the tab shown

### Requirement: The activity panels all show without scripting

Without scripting the tab strip SHALL NOT be drawn and every activity panel SHALL show under its own
heading, one under another.

#### Scenario: Scripting off

- **WHEN** a task screen is drawn with scripting disabled
- **THEN** Comments, History and Artifacts each show with their heading and nothing is hidden

### Requirement: The task panel's due hint fits its column

The due field in the task panel SHALL show a hint that fits its column whole: the date in the reader's
format, with the words and offsets it also takes in its tip.

#### Scenario: A task with no due date

- **WHEN** a task with no due date is shown at any viewport
- **THEN** its due field's hint is a whole date, not one cut off

### Requirement: The Manage dialog sends only what changed

Saving a user's Manage dialog on the Users screen SHALL send the service only the fields whose value
differs from the user as stored: display name, role, disabled and the forced password change. A save that
changes nothing SHALL make no update.

#### Scenario: Renaming a person who belongs to two tenants

- **GIVEN** a tenant admin who is not a global admin, and a person who is a member of that tenant and another
- **WHEN** the admin changes only the person's display name or role in the Manage dialog and saves
- **THEN** the change is saved and the forced password change is neither sent nor set

#### Scenario: Saving the form as drawn

- **WHEN** an admin saves a Manage dialog without changing anything
- **THEN** no user update is recorded

### Requirement: A cross-tenant refusal is shown on the Manage dialog

When the service refuses a Manage dialog change with reason `account_in_other_tenants`, the Users screen
SHALL be drawn again with that dialog open, holding what was submitted, and a message naming the person
beside the control that asked: the forced password change box, or the clear second factor part.

#### Scenario: Asking a two-tenant person to change their password

- **WHEN** a tenant admin who is not a global admin ticks the forced password change for a person who also belongs to another tenant and saves
- **THEN** the dialog comes back open with the typed values and says that only the person or a global admin can ask them to change their password, and nothing is changed
