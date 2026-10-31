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

Editing part of a workflow SHALL preserve the rest. A save from the workflow editor SHALL change only what its form edits -- the initial state, the state keys with their labels and terminality, the permitted transitions, and the state migrations -- and SHALL carry every other field of a state, a transition or the definition through from what is stored. A state or transition the form no longer names SHALL be removed.

A state key the form renames SHALL keep the fields the editor does not show when the save declares the rename in its migration lines. A rename the save does not declare SHALL be treated as removing one state and adding another, which the service refuses outright while any task still sits in the removed state.

The third field of a state line SHALL accept `terminal` or `open` and `true` or `false`, SHALL read an empty field as open, and SHALL refuse a value that names neither rather than reading it as open. The help text beside the field SHALL name the values it accepts.

#### Scenario: Workflow is edited

- **WHEN** an authorized user adds a state and a transition to a project's workflow
- **THEN** the board shows the new column and the new transition is permitted

#### Scenario: Saving without changing anything changes nothing

- **WHEN** an authorized user opens the workflow editor and saves it with no edits
- **THEN** every state keeps its reporting category and its lease-expiry revert, every transition keeps the scope and the comment it requires, and the workflow keeps its default lease

#### Scenario: A restricted transition stays restricted

- **WHEN** a transition requires a scope and the workflow is saved from the editor with no edits
- **THEN** an actor holding the plain transition scope but not the required one is still refused that move, and an actor holding the required scope may still make it

#### Scenario: A renamed state keeps what the editor does not show

- **WHEN** an authorized user changes a state's key and gives the rename as a migration line
- **THEN** the state under its new key keeps the category and the lease-expiry revert the old key carried

#### Scenario: A removed state is removed

- **WHEN** an authorized user deletes a state line and the transitions naming it
- **THEN** that state and those transitions are gone from the stored workflow

#### Scenario: Invalid workflow is refused

- **WHEN** a workflow edit would leave tasks in a state the workflow no longer defines
- **THEN** the change is refused with an explanation and the workflow is unchanged

#### Scenario: The documented terminal words are read as written

- **WHEN** a state line ends in `true`, having followed the help text beside the field
- **THEN** the state is stored as terminal

#### Scenario: An unreadable terminal field is refused

- **WHEN** a state line ends in a word that is neither a yes nor a no
- **THEN** the save is refused with an explanation and no workflow is stored

#### Scenario: Field definition is created

- **WHEN** an authorized user defines a typed custom field
- **THEN** the field appears on task detail screens for that project and values are validated against its type

### Requirement: Tenant, domain, user, and token administration

The web UI SHALL provide administration screens for tenants and their memberships, for domain mappings, for
users, and for API tokens, restricted to callers holding the corresponding administrative scopes.

The token screen's issue form SHALL be able to set every property of a token the service accepts and the
listing displays, which SHALL include its scopes and its expiry. The expiry control SHALL offer a set of
durations and an explicit "never expires" choice, and SHALL propose an expiry rather than proposing no
expiry. A value the control does not offer SHALL be refused rather than read as no expiry.

The token listing SHALL show, as optional columns behaving like every other optional column, when each
token was created and when it expires, and SHALL show whether each token still authenticates: active,
expired, or revoked with the time it was revoked.

A revoked token SHALL NOT offer a revocation control. Revocation, being irreversible and ending a
credential something may be holding, SHALL be behind a confirming disclosure that names the token it ends
and says what revoking does.

A newly issued token's value SHALL be presented in a region of its own, distinct from the interface's
transient confirmation message, readable in full without being cropped, selectable, and accompanied by a
copy control. The value SHALL remain readable in full when no scripting is available.

#### Scenario: Administrative screens require scope

- **WHEN** a caller without administrative scope requests an administration screen
- **THEN** access is refused and no administrative data is rendered

#### Scenario: Token is issued once

- **WHEN** an authorized administrator creates an API token
- **THEN** the token value is displayed once at creation and is not retrievable afterwards

#### Scenario: The issued value is not presented as a confirmation message

- **WHEN** a token has just been issued
- **THEN** its value is in its own region rather than in the transient confirmation message
- **AND** a copy control points at it

#### Scenario: An expiry set on the form reaches the token

- **WHEN** an administrator issues a token choosing an expiry of seven days
- **THEN** the token's stored expiry is seven days ahead
- **AND** the listing's expires column shows it

#### Scenario: A token issued with no expiry says so

- **WHEN** an administrator issues a token choosing never expires
- **THEN** the listing's expires column says the token does not expire

#### Scenario: The listing says when a token was created

- **WHEN** the token listing is read
- **THEN** each row shows when that token was created

#### Scenario: A revoked token is shown as revoked and offers no revocation

- **GIVEN** a revoked token
- **WHEN** the token listing is read
- **THEN** the row is marked revoked
- **AND** the row offers no control to revoke it

#### Scenario: Revocation is confirmed before it happens

- **GIVEN** a live token
- **WHEN** the token listing is read
- **THEN** reaching the revoke control requires opening a disclosure that names the token and says that
  whatever holds it stops authenticating

#### Scenario: Domain mapping is manageable

- **WHEN** an authorized administrator adds a domain mapping for a tenant
- **THEN** the mapping appears in the domain list and its verification state is shown

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
allow a project's colour and icon to be set from the palette and cleared again.

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
render that declared default. Each listing SHALL always render the column naming its rows, whatever
is chosen, so no choice can produce a listing with nothing in it or put a record out of reach.
Hiding a column is a display choice and not a permission: every hidden value SHALL remain reachable
on the record's own screen. The choice SHALL be recorded per browser rather than in tenant data, so
two people working in one tenant may read a listing differently, and SHALL survive later requests
from that browser. A recorded choice that this build does not recognise SHALL be discarded in favour
of the declared default rather than failing the page.

#### Scenario: An untouched installation is unchanged

- **WHEN** a listing is rendered for a browser that has chosen no columns
- **THEN** it renders exactly the declared default set, in the declared order

#### Scenario: A chosen set changes what renders

- **WHEN** a reader chooses a column set for a listing
- **THEN** that listing renders the chosen columns and omits the rest

#### Scenario: The choice survives the next request

- **WHEN** the same browser requests the listing again
- **THEN** it is rendered with the chosen set rather than with the default

#### Scenario: The choice belongs to the browser

- **WHEN** a second browser requests the same listing
- **THEN** it is rendered with the default set, unaffected by the first browser's choice

#### Scenario: An unrecognised choice falls back

- **WHEN** a listing is rendered for a browser whose recorded preference names an unknown listing or
  column, or is malformed or oversized
- **THEN** the declared default is rendered and no error is raised

#### Scenario: Nothing is put out of reach

- **WHEN** a column is hidden from a listing
- **THEN** the listing still names each row, and the hidden value is still shown on the record's own
  screen

#### Scenario: The control needs no scripting

- **WHEN** the column control is submitted with JavaScript disabled
- **THEN** the choice is recorded and the reader is returned to the listing they chose it from,
  filter and position intact

#### Scenario: Resetting restores the default

- **WHEN** a reader resets a listing's columns
- **THEN** the listing renders the declared default set again

### Requirement: An actor is shown by name rather than by identifier

The browser interface SHALL name the actor behind a record -- a creator, an assignee, a comment
byline, an audit row -- by that actor's handle when the directory holds one. Where no
handle can be resolved, it SHALL show a two-word name derived from the identifier itself, so that
one identifier always yields the same name in every process and on every installation. A generated
name SHALL be a display aid only: the identifier SHALL remain reachable on the element that carries
the name, and machine-readable output SHALL be unchanged, carrying the identifier and no generated
name. Resolving a name SHALL be scoped to the caller's own tenant.

#### Scenario: A handle is used where one exists

- **WHEN** a screen names an actor that has a handle
- **THEN** the handle is shown rather than the identifier or a generated name

#### Scenario: A name is generated where no handle exists

- **WHEN** a screen names an actor whose handle cannot be resolved
- **THEN** a two-word name derived from the identifier is shown in its place

#### Scenario: The same identifier always reads the same

- **WHEN** one identifier is rendered in two processes or on two installations
- **THEN** both produce the same generated name

#### Scenario: The identifier stays reachable

- **WHEN** a generated name or a handle is shown for an actor
- **THEN** the identifier it stands for is available on the element, and the record's machine-readable
  form still carries the identifier and gains no name field

#### Scenario: Names do not cross the tenant boundary

- **WHEN** a record names an actor belonging to another tenant
- **THEN** that actor's handle is not disclosed and the generated name is shown instead

#### Scenario: A membership names the person it grants a role to

- **WHEN** the tenant administration screen lists this tenant's members
- **THEN** each row names the actor rather than showing its bare identifier

### Requirement: Every display preference has exactly one always-reachable control

The browser interface SHALL gather the preferences that apply to every screen into a settings screen
reachable from the sidebar on every signed-in screen. Every control within it SHALL be a plain form
that works with scripting disabled, and the screen SHALL render under every colour scheme on offer,
including the low contrast one. Preferences scoped to one listing, such as which columns it shows,
SHALL stay on that listing rather than moving into the settings screen.

The colour scheme SHALL instead be set by a single control in the sidebar, present on every
signed-in screen, and SHALL NOT also be settable from the settings screen, so that one cookie has
one control. That control SHALL be a plain form that works with scripting disabled, SHALL move to
the next scheme on offer each time it is submitted, and SHALL return the reader to the screen they
submitted it from. It SHALL be reachable by keyboard, and its accessible name SHALL state both the
scheme in force and the scheme submitting it moves to, rather than leaving either to a hover-only
attribute. The settings screen SHALL name the scheme in force and say where the control is.

#### Scenario: The menu is on every screen

- **WHEN** any signed-in screen is rendered
- **THEN** the sidebar carries a link to the settings screen, which gathers the advanced toggle and
  every other preference not scoped to one listing

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

#### Scenario: The switch says what it is and what it does

- **WHEN** the sidebar's scheme control is rendered
- **THEN** its accessible name states the scheme in force and the scheme submitting it moves to

#### Scenario: Every scheme renders it

- **WHEN** the interface is rendered under the system, light, dark, and low contrast schemes
- **THEN** the settings screen renders in each, and the sidebar's control names the scheme the page
  is drawn in

#### Scenario: The menu needs no scripting

- **WHEN** a preference is changed with JavaScript disabled
- **THEN** the change is recorded and the reader is returned to the page they made it on

### Requirement: The project listing offers every project control

The project listing SHALL make every operation on a project reachable from the listing itself: its
name, workflow, colour, icon and description, and the controls to archive and to delete it, in
addition to creating a new one. A reader on the listing SHALL be able to tell that a project can be
changed without first opening it. The controls SHALL be plain forms, SHALL be governed by the
listing's column preference like any other optional column, and the listing SHALL always name each
row whatever is chosen.

#### Scenario: Controls are reachable from the listing

- **WHEN** the project listing is rendered
- **THEN** each row offers the project's settings and the controls to archive and delete it

#### Scenario: Editing from the listing preserves what it did not show

- **WHEN** a project is saved from the listing
- **THEN** every field the form carried is stored and no unshown field is blanked

#### Scenario: The controls are an optional column

- **WHEN** the column carrying the controls is switched off
- **THEN** the listing still names each row and links to the project

### Requirement: The task list shows a chosen set of lists

The task list SHALL offer a control selecting which projects it draws from, placed with the listing's
own controls rather than in the settings menu. An installation that has made no choice SHALL show
every list, and a project created after a choice was made SHALL be visible without the choice being
revisited. The choice SHALL be recorded per browser and SHALL survive later requests. A filter
naming a project explicitly SHALL override the choice, and the screen SHALL say which rule produced
what it shows rather than presenting a shorter or empty list without explanation. A recorded value
this build does not recognise SHALL fall back to showing every list.

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
trail can be followed from any row. The subject SHALL link to the record itself where the reader may
reach one: a task to its own screen, a project or workflow to its screen, and a comment or an
artifact to its place on the task it hangs off, addressed so the browser lands on that record rather
than on the top of the page. A subject the feed cannot resolve SHALL name its kind in words and
carry no link, never a bare identifier presented as a destination.

#### Scenario: An actor leads to that actor's own trail

- **WHEN** a reader follows the actor named on a row
- **THEN** the feed is shown narrowed to that actor, and says so

#### Scenario: A comment is reachable from the row about it

- **WHEN** the feed renders an entry whose subject is a comment
- **THEN** the row links to that comment's own place on the task it was left on

#### Scenario: A record inside a closed disclosure is still reached

- **WHEN** a link from the feed addresses a record that the target screen keeps behind a disclosure
- **THEN** that disclosure is opened and the record is marked, so the link lands somewhere visible

#### Scenario: An unresolvable subject is not a dead link

- **WHEN** the feed renders an entry whose subject it cannot name
- **THEN** the row says what kind of record changed and offers no link

### Requirement: The activity feed is searchable and filterable from its own address

The activity feed SHALL offer a filter over free text, the kind of record changed, the surface the
change arrived from, and the actor that made it. The whole filter SHALL live in the query string, so
a narrowed feed is a link that can be shared, bookmarked and restored, and the control SHALL be a
plain form that submits without scripting. Every term that is narrowing the feed SHALL be shown, each
removable on its own without retyping the rest. A filter that matches nothing SHALL say so and say
how much was searched, rather than reading as an empty tenant.

#### Scenario: A filter is addressable

- **WHEN** a filter is applied to the feed
- **THEN** the address bar carries it, and loading that address again restores the same feed

#### Scenario: The live update respects the filter

- **WHEN** a change arrives over the event stream while the feed is filtered
- **THEN** the re-rendered rows are still narrowed by that filter

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
from a project from a priority at a glance. The same fact SHALL take the same form on every screen
that shows it, so a state or a priority does not change shape between the list, the board and a
task's own screen.

#### Scenario: Four kinds of marking are four shapes

- **WHEN** a task row renders its state, its priority, its tags and its project
- **THEN** each kind is rendered in a form that differs from the other three by more than its colour

#### Scenario: A marking reads the same everywhere

- **WHEN** the same workflow state is rendered on the task list, on the board and on a task's screen
- **THEN** it takes the same form on all three

#### Scenario: A tag and a project lead to the rest of their own

- **WHEN** a reader follows a tag or a project on a row
- **THEN** the task list is shown narrowed to that tag or that project

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
document and resolved from the event, or re-established when the swap completes. A behaviour SHALL
NOT depend on an element that existed when the script first ran, and re-establishing one SHALL NOT
leave a second connection or a second handler behind.

#### Scenario: Dragging works on a board reached by clicking

- **WHEN** a reader navigates to a project board from another screen by following a link
- **THEN** a card dragged to a legal column is moved, exactly as it is for a board reached by its own URL

#### Scenario: The live feed follows the reader

- **WHEN** a reader navigates to the activity feed from another screen by following a link
- **THEN** the feed updates as events arrive

#### Scenario: Re-establishing opens nothing twice

- **WHEN** a reader navigates between two screens repeatedly
- **THEN** no more than one event-stream connection is held at a time

### Requirement: An emptied form field clears the value it holds

A browser form field that a reader empties and saves SHALL clear the stored value. A field the submission
does not name at all SHALL leave the stored value unchanged.

The distinction SHALL be drawn on whether the submission carries the field's key, not on whether the value
it carries is empty, so that a submission naming some of a record's fields changes only the fields it
names.

This SHALL hold for a task's custom fields and for an account's display name, which are the two values the
browser could previously set and never remove.

Where clearing a value would leave a required field empty, the save SHALL be refused with a validation
error and the stored value SHALL be left as it was. A refused save SHALL NOT report success.

#### Scenario: Emptying a custom field clears it

- **GIVEN** a task whose custom field holds a value
- **WHEN** the reader empties that field's input and saves
- **THEN** the field is shown empty on the task afterwards

#### Scenario: A save naming one field leaves the others alone

- **GIVEN** a task carrying values in two custom fields
- **WHEN** a submission names only the first of them
- **THEN** the first holds the submitted value and the second is unchanged

#### Scenario: A save carrying no custom field inputs changes none of them

- **GIVEN** a task whose custom field holds a value
- **WHEN** the reader saves the task's title from a form carrying no custom field input
- **THEN** the custom field still holds its value

#### Scenario: Emptying a required field is refused

- **GIVEN** a task whose required custom field holds a value
- **WHEN** the reader empties that field's input and saves
- **THEN** the save is refused as invalid
- **AND** the stored value is unchanged

#### Scenario: Emptying a display name removes it

- **GIVEN** an account with a display name
- **WHEN** an administrator empties the name input and saves
- **THEN** the account has no display name afterwards

#### Scenario: A save naming no display name leaves it alone

- **GIVEN** an account with a display name
- **WHEN** a submission changes the account's role and names no display name
- **THEN** the account keeps its display name

### Requirement: Putting a project away removes that project and nothing else

The task listing's visibility control SHALL remove from the listing exactly the projects the reader has
put away, whatever the number of projects in the tenant.

The listing SHALL be narrowed by excluding the projects put away, rather than by naming the projects to
show, so that the rows shown do not depend on any listing of projects having been complete.

The control SHALL offer a box for every project in the tenant, so that a project can be put away and
brought back whatever its position in the project listing.

#### Scenario: Hiding one project of many keeps the rest

- **GIVEN** a tenant with more projects than one page of the project listing returns
- **AND** a task in a project past that first page
- **WHEN** the reader puts a single other project away
- **THEN** that task is still listed

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

The choice SHALL change only how the selected tasks are drawn. The filter expression, the deadline
window, the sort, the page size, the page position and the project visibility choice SHALL be
resolved once and SHALL apply identically to every view, so that switching never changes which tasks
are on screen beyond the list's open-work default. Every view SHALL be walked by the same pager.

The list SHALL be the view an installation shows before any choice is made, and a stored value that
this build does not recognise SHALL read as the list rather than failing the screen.

#### Scenario: The choice survives the next request

- **GIVEN** a reader on the task screen
- **WHEN** they switch the view to the board or to lanes
- **AND** they request the task screen again
- **THEN** the chosen view is drawn without the switch being pressed a second time

#### Scenario: Switching keeps the filter

- **GIVEN** a task screen filtered to one tag
- **WHEN** the reader switches to the board
- **THEN** the board holds exactly the tasks the filtered list held
- **AND** the filter expression is still in the filter box

#### Scenario: Switching keeps the project visibility choice

- **GIVEN** a reader who has put one project away
- **WHEN** they switch to the board
- **THEN** no task of the project they put away appears on the board

#### Scenario: An untouched installation shows the list

- **GIVEN** a browser that has never used the switch
- **WHEN** it opens the task screen
- **THEN** the list is drawn

#### Scenario: An unrecognised stored choice shows the list

- **GIVEN** a browser carrying a view preference this build does not implement
- **WHEN** it opens the task screen
- **THEN** the list is drawn rather than an error

### Requirement: Board columns merge only across workflows that are the same state machine

The task board SHALL draw one set of columns from the workflow the selected projects share, and SHALL
treat two projects' workflows as the same workflow only when their definitions agree on every element
a board draws or a move depends on: the states in their declared order, including each state's key,
label, terminal flag and category, and the transitions in their declared order, including each
transition's source, target, required scope and whether it requires a comment.

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

#### Scenario: A differing state category prevents a merge

- **GIVEN** two projects whose workflows declare the same state keys and transitions but give one state a different category
- **WHEN** the reader draws the board over both
- **THEN** no merged board is drawn

### Requirement: A board card is never offered a move its own project's workflow forbids

Every move a board card offers SHALL be derived from the workflow of that card's own project, never
from the definition the columns were drawn from.

A control for a transition the card's own workflow does not allow SHALL NOT be rendered, so that the
reader is refused before acting rather than by the service after acting. The service refusing such a
move SHALL remain the fallback and SHALL NOT be the only protection.

A drop onto a column SHALL apply a single transition, since a drop names a destination and nothing
else. A route passing through other states SHALL be offered only by a control that names the states
it passes through before it is applied.

#### Scenario: An unreachable state is not offered

- **GIVEN** a card whose project's workflow permits no move from its current state to a given state
- **WHEN** the board is drawn
- **THEN** the card's move control offers no option for that state

#### Scenario: A card offers only its own project's routes

- **GIVEN** a merged board holding cards from two projects
- **WHEN** the board is drawn
- **THEN** each card's move control lists exactly the routes its own project's workflow allows from that card's state

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

The reader's view preference SHALL be left as it is by the refusal, so that narrowing the selection
draws the board without the switch being pressed again.

Where two named groups run workflows carrying the same name, the refusal SHALL distinguish them, so
that two entries reading alike cannot be the whole explanation.

A task on the page whose status matches none of the drawn columns SHALL be reported on the screen
rather than omitted from it.

#### Scenario: Disagreeing workflows refuse the board and say which projects disagree

- **GIVEN** three selected projects, two sharing a workflow and one differing
- **WHEN** the reader asks for the board
- **THEN** the list is drawn
- **AND** the screen states that the projects do not share a workflow
- **AND** each distinct workflow is named with the projects that run it

#### Scenario: Narrowing to one group draws its board

- **GIVEN** a refused board naming two groups of projects
- **WHEN** the reader follows the entry for one group
- **THEN** the board of that group's shared workflow is drawn
- **AND** the switch did not have to be pressed again

#### Scenario: Two workflows of the same name are named apart

- **GIVEN** a refusal naming two groups whose workflows are both called `Default`
- **WHEN** the notice is drawn
- **THEN** the two entries are distinguishable from one another

#### Scenario: A task in a state the workflow no longer has is reported

- **GIVEN** a board whose page holds a task whose status is in none of the drawn columns
- **WHEN** the board is drawn
- **THEN** the screen names that task as unplaced rather than leaving it off the board silently

### Requirement: A refused submission is answered by the form, not by an error page

Where a browser submission is refused for a reason the reader can act on by changing what they submitted —
a missing or invalid value, or a conflict with something that already exists — the screen SHALL re-render
the form it came from, carrying every value the submission held and the refusal's message beside the
control it concerns.

This SHALL be the response to the ordinary form submission, so that it holds with scripting turned off. A
client-side validation attribute on a control MAY be present as a convenience, and SHALL NOT be the thing
performing the check.

A refusal the reader cannot act on by editing the submission — an authentication or authorization failure,
or an internal fault — SHALL continue to be answered by the error screen, which is the only surface that
can explain it.

The message SHALL be associated with its control for assistive technology, and the control SHALL be marked
as invalid.

#### Scenario: A submission with a missing required value comes back

- **GIVEN** the API token screen's issue form
- **WHEN** a submission naming a token name but selecting no scope is sent
- **THEN** the token screen is rendered again rather than the error screen
- **AND** the name input still holds the submitted name
- **AND** the message explaining the refusal appears with the scope control

#### Scenario: A conflicting submission comes back

- **GIVEN** a tenant already holding a live token named `ci`
- **WHEN** a submission naming `ci` is sent
- **THEN** the token screen is rendered again with the name still in its input
- **AND** the message says the name already exists

#### Scenario: An unauthorized submission still reaches the error screen

- **WHEN** a submission is refused because the caller lacks the required scope
- **THEN** the error screen is rendered

### Requirement: A tenant administrator can see and revoke another actor's token in the browser

The API token screen SHALL list every token of the tenant to a reader holding the tenant administration
scope, and SHALL list only the reader's own tokens to any other reader.

Where the listing spans more than the reader's own tokens, each row SHALL say which actor holds it, by that
actor's handle where the directory holds one. A row that is the reader's own SHALL be distinguishable from
a row that is not.

A revocation control SHALL be offered only for a token the reader is shown, and the control for a token
held by another actor SHALL say whose it is before it is used. No control SHALL be offered for a token the
reader may not see.

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

#### Scenario: The control for somebody else's token says whose it is

- **GIVEN** an administrator reading a token held by another actor
- **WHEN** the revocation disclosure for that row is opened
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

It SHALL be presented in the same one-time secret region the token screen uses for a freshly issued token:
a region of its own rather than the interface's transient confirmation message, readable in full without
being cropped, selectable, and accompanied by a copy control. The value SHALL remain readable in full when
no scripting is available.

A secret the operator supplied SHALL NOT be presented back to them.

The field's own help SHALL state what happens in each case: when tix generates a secret, when the operator
supplies one, and when an existing endpoint is saved with the field left blank.

#### Scenario: A generated secret appears once

- **WHEN** an endpoint is registered with the signing secret field left blank
- **THEN** the webhook screen presents the generated secret in its one-time secret region
- **AND** a copy control points at it

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

#### Scenario: The field's help is true in both cases

- **WHEN** the signing secret field's help is read
- **THEN** it says a generated secret is shown once
- **AND** it says a secret the operator supplies is not shown

### Requirement: One presentation for a value the server will not disclose again

A value the server will never disclose again SHALL be presented through one shared region, used by every
screen that has such a value, rather than through markup copied per screen.

The region SHALL be distinct from the interface's transient confirmation message, SHALL be readable in full
without being cropped, SHALL be selectable, and SHALL carry a copy control driven by the interface's one
generic copy affordance. It SHALL remain readable in full when no scripting is available.

A freshly issued API token and a generated webhook signing secret SHALL both be presented through it.

#### Scenario: The issued value is not presented as a confirmation message

- **WHEN** a token has just been issued
- **THEN** its value is in its own region rather than in the transient confirmation message
- **AND** a copy control points at it

#### Scenario: Both screens use one region

- **WHEN** the token screen and the webhook screen each present a one-time secret
- **THEN** both are rendered by the same shared region

### Requirement: The settings screen owns every per-browser display preference

The settings screen SHALL carry a control for every per-browser display preference this interface
keeps, with no preference editable only from elsewhere.

Every such control SHALL be a form the browser submits on its own, with a method, an action and a
submit button, and SHALL carry no scripted handler, so that it stores its value with scripting
turned off.

Every such control SHALL be accompanied by a description of what the preference does, associated
with the control so that assistive technology announces it, and written so that a reader who does not
already know the preference's name can tell what it changes.

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

Where the stored preference holds a key that no project on the control's own listing answers to, the
control SHALL name that key and SHALL say that no project answers to it, and SHALL not offer a
checkbox for it. Submitting the control SHALL discard such keys.

A stored key that names no project SHALL exclude nothing from the task listing, and SHALL not prevent
any screen from rendering.

#### Scenario: A project is named, not only keyed

- **GIVEN** a tenant with a project whose key and name differ
- **WHEN** a reader opens the visibility control on either screen
- **THEN** the project's name and its key are both shown

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

A figure whose listing stopped before the end SHALL NOT be rendered as a number, since the number
would be a floor presented as a total; the screen SHALL print in its place that the figure is
incomplete, distinguishably from a listing that failed to be read at all.

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

#### Scenario: A figure whose walk stopped prints no total

- **GIVEN** a tenant with more projects than the project walk covers
- **WHEN** a reader opens the tenant screen
- **THEN** the Projects row of the diagram prints that the count is incomplete, in the figure's place

#### Scenario: One caveat for one control rendered twice

- **GIVEN** the project visibility control, which the task screen and the settings screen both render
- **WHEN** the walk behind its choices stops before the end
- **THEN** both screens state it, from the control's own state rather than from either screen's

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

The number SHALL be derived from what the control's own links carry. Where that memory is bounded and
has discarded the earliest of it, the number SHALL be carried alongside rather than recomputed from
what is left.

A number arriving from the address bar SHALL NOT be believed below what the link's own history
already proves, SHALL be bounded, and SHALL reach no query. The worst a value a reader typed can cost
is a wrong label on the control.

A link short of that bound SHALL carry no page number of its own, since its history already counts
it.

#### Scenario: Walking past the bound of the control's memory

- **GIVEN** a listing with more pages than the control's cursor memory holds
- **WHEN** a reader follows the control forward past that bound
- **THEN** each page states a number one higher than the page before it

#### Scenario: Walking back past the bound

- **WHEN** the reader follows the control back from a page past that bound
- **THEN** each page states a number one lower than the page before it, for every page the control
  can still reach

#### Scenario: A number below what the history proves is ignored

- **GIVEN** a page whose link carries a history of forty positions
- **WHEN** the address bar names a page number lower than that history accounts for, or a value that
  is not a number at all
- **THEN** the control states the number the history itself counts

#### Scenario: An early page's link carries no page number

- **WHEN** a reader follows the control forward or back from a page within the bound of its memory
- **THEN** the resulting address carries no page number parameter

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

A resubscription SHALL carry the sequence of the last event the screen saw, so
the server replays the gap. Where that cursor is no longer retained, the
screen SHALL subscribe afresh and re-fetch its fragment whole rather than
leaving a gap nothing fills.

Where no connection can be established at all, the screen SHALL poll its
fragment on a slow interval, and SHALL stop polling as soon as a connection
succeeds.

While a screen is not receiving events it SHALL say so on screen, naming the
consequence: that what is shown may be out of date.

#### Scenario: A dropped connection is reopened

- **GIVEN** a screen receiving events
- **WHEN** the connection drops
- **THEN** it is reopened after a bounded, growing, jittered delay, and updates resume

#### Scenario: Nothing is lost across the gap

- **GIVEN** a screen that has seen events up to a known sequence
- **WHEN** it reconnects
- **THEN** it resubscribes from that sequence and the changes made while it was down are shown

#### Scenario: A cursor the server cannot honour

- **GIVEN** a screen resuming from a sequence that has been retained out of the log
- **WHEN** the server refuses the cursor
- **THEN** the screen subscribes from the present and re-fetches its fragment, rather than failing silently

#### Scenario: A sleeping laptop does not storm the server

- **GIVEN** a page hidden while its connection is down
- **WHEN** it is hidden
- **THEN** it does not retry, and on becoming visible it reconnects at once and re-reads

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

A `<select>` whose rendered markup names no option as selected is presented by
every browser with its first option selected. That presentation SHALL NOT be
read as an edit: such a control SHALL be treated as untouched while it sits on
its first option, and as edited once it sits on any other. A `<select>` whose
markup does declare a default SHALL continue to be read against that default.

A control rendered inside a live region SHALL declare its own default, so that
what the reader is shown and what the markup asked for agree. The project
board's Move control SHALL mark its first route selected.

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

#### Scenario: The Move control names the option the browser will show

- **WHEN** the board is rendered, as a whole page or as its live fragment
- **THEN** each Move select carries exactly one selected option, and it is the first route

#### Scenario: A declared default is still what a choice is measured against

- **GIVEN** a select whose markup marks an option other than the first as selected
- **WHEN** the control sits on the first option instead
- **THEN** it is reported as edited

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
its own frame. Its columns SHALL NOT wrap onto further rows and SHALL NOT be stacked one above
another, because a board column is a fixed-height scroller and both arrangements grow past that
height, overrunning whatever the page draws beneath the board.

At phone width the board SHALL snap one column to the viewport, so that a column is read whole
rather than in parts.

A card on a board covering more than one project SHALL name the project it belongs to.

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

#### Scenario: A phone settles on one column at a time

- **GIVEN** a viewport of phone width
- **WHEN** the reader swipes the board sideways
- **THEN** it comes to rest with a single column filling the viewport

#### Scenario: The board does not draw over what follows it

- **GIVEN** a project board with a settings panel beneath it
- **WHEN** the board is drawn at phone width
- **THEN** the settings panel is visible and operable below the board rather than covered by it

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

An attribute omitted for width SHALL remain available on the task's own screen, which the row
links to, so that nothing is unreachable rather than merely unshown.

#### Scenario: A phone row keeps its title

- **GIVEN** a task with several tags, an assignee, a project and a deadline
- **WHEN** its row is drawn at phone width
- **THEN** the title is drawn without being compressed to make room for the tags

#### Scenario: The shed attributes are on the task screen

- **GIVEN** a row drawn at a width that omits its tags
- **WHEN** the reader opens the task
- **THEN** the tags are listed there

#### Scenario: Identifying attributes survive every width

- **GIVEN** any viewport width the interface supports
- **WHEN** a task row is drawn
- **THEN** its status, priority, assignee, lease state, deadline mark and reference are present

### Requirement: A failure states itself in the flow of its own screen

When a screen cannot be drawn, the reason SHALL be rendered in the flow of the page, beneath the
heading that announces the failure, and SHALL be set to a measure that keeps it readable as a
sentence.

The reason SHALL NOT be rendered as a transient notification, so that it cannot be placed apart from
the heading it explains, mistaken for an unrelated notice, or drawn outside the reader's view.

The screen SHALL offer the ways out of it as controls carrying the same appearance as the other
navigation controls in the interface.

#### Scenario: The reason sits under its heading

- **GIVEN** a request that cannot be served
- **WHEN** the failure screen is drawn
- **THEN** the reason appears beneath the heading, in the flow of the page

#### Scenario: The reason is not a toast

- **GIVEN** a failure screen
- **WHEN** it is drawn
- **THEN** the reason is not rendered as the transient notification the rest of the interface uses for flashes

#### Scenario: The ways out look like the controls they are

- **GIVEN** a failure screen offering a way back
- **WHEN** it is drawn
- **THEN** that way back is drawn as a control rather than as bare text

### Requirement: The error screen's ways out lead where they say

Every link the error screen offers SHALL be labelled with where it leads, and no link SHALL point at
an address that redirects somewhere other than its label. A failed mutation SHALL offer the screen
it was submitted from. A failed page SHALL offer the same-origin page that linked to it, and SHALL
offer the task list when there is none or when that page is the failing one. The task list SHALL
always be one of the ways out. The error screen SHALL reach the browser as the page it is, at its own
status, under the server that also serves the API, and SHALL NOT be replaced by the API's error
envelope.

#### Scenario: A missing page with no referrer

- **WHEN** a signed-in reader requests a page that does not exist, with no referring page
- **THEN** the error screen offers "Back to tasks" leading to the task list
- **AND** it offers no link to the root address

#### Scenario: A missing task under the full server

- **GIVEN** `tix serve` serving the API and the browser interface together
- **WHEN** a signed-in reader opens a task reference that does not exist
- **THEN** they get the HTML error screen with a not found status, not a JSON body

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
return to the control that was used. The chosen state SHALL be drawn before the server answers, and
a refused or failed action SHALL put the previous state back and say why. Without JavaScript the same
forms SHALL post and redirect.

#### Scenario: Starring a row halfway down the list

- **GIVEN** the task list scrolled so the twentieth row is in view
- **WHEN** the reader stars that row
- **THEN** the star fills at once, only that row is replaced, the list stays where it was, and the
  star keeps focus

#### Scenario: A move the server refuses

- **WHEN** a move chosen from a row's status menu is refused
- **THEN** the row shows its previous status and a toast says why

#### Scenario: No JavaScript

- **GIVEN** a browser with scripting off
- **WHEN** the reader stars a row
- **THEN** the form posts and the listing is drawn again on that row

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

Links into tasks and listings SHALL start on the primary mouse button's press, except a link on a card that can be dragged, which SHALL open on its click so the press can start a drag. Boosted navigations
SHALL cross-fade where the browser supports view transitions, and SHALL NOT for a reader who prefers
reduced motion. Back to the task list SHALL restore the list's scroll position. The task filter SHALL
apply as the reader types, after a pause, and when a select beside it changes, and the Apply button
SHALL remain. The list, board and lanes switch SHALL swap the page without reloading its assets.

#### Scenario: Dragging a card by its title

- **GIVEN** a board with dragging on
- **WHEN** the reader presses a card's title and drags it to another column
- **THEN** the card moves to that column and the task is not opened

#### Scenario: Dropping a starrable card

- **GIVEN** a board card that carries a star button ahead of its Move form
- **WHEN** the reader drops the card on a column it may move to
- **THEN** the move is posted to the Move form's action and the star is left as it was

#### Scenario: Back to the list

- **GIVEN** the task list scrolled to the fortieth row
- **WHEN** the reader opens a task and presses Back
- **THEN** the list is shown scrolled to where it was

#### Scenario: Reduced motion

- **GIVEN** a reader whose system prefers reduced motion
- **WHEN** they open a task from the list
- **THEN** the screen is swapped with no cross-fade

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
information disclosure beside its name. An on/off preference SHALL be drawn as a switch that exposes
its state to assistive technology and remains a plain form submission.

#### Scenario: An on/off preference is a switch

- **WHEN** the settings screen is drawn
- **THEN** each on/off preference's submit button has the switch role and reports whether it is on

#### Scenario: Rows stack on a narrow screen

- **GIVEN** a viewport 900px wide
- **WHEN** the settings screen is drawn
- **THEN** each row's control sits under its description

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
of 1440px, 900px and 390px, with any disclosure inside a table row opened. A table that cannot fit
SHALL scroll inside its own frame, a disclosure's label SHALL NOT break between its letters, and an
information note SHALL stay inside the viewport.

#### Scenario: The token table with Revoke open

- **GIVEN** a viewport 900px wide and a tenant with API tokens
- **WHEN** the reader opens a token's Revoke disclosure
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

The control SHALL be reachable by keyboard as a vertical separator that exposes its current value,
minimum and maximum, and the arrow keys SHALL change the width. Without scripting the control SHALL
be absent and the sidebar SHALL keep its default width.

#### Scenario: A dragged width survives a reload

- **GIVEN** a reader who drags the sidebar's edge to 320px
- **WHEN** they reload the page
- **THEN** the sidebar is 320px wide from the first paint

#### Scenario: A drag past the maximum stops at the maximum

- **WHEN** the reader drags the edge further right than the maximum width
- **THEN** the sidebar is drawn at the maximum width

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
collapse it. The rail SHALL keep the product logo and every navigation entry as an icon, and every
entry in the rail SHALL keep its accessible name. The collapsed state SHALL be kept per browser and
applied before the page paints, and expanding SHALL return the sidebar to the width it had.

On a viewport too narrow for the sidebar column, the collapsed state SHALL have no effect, and the
sidebar SHALL be the bar and menu it is without it.

#### Scenario: Dragging it small collapses it

- **WHEN** the reader drags the sidebar's edge below the collapse threshold and lets go
- **THEN** the sidebar is drawn as a rail holding the logo and the navigation icons
- **AND** a button to expand it is visible

#### Scenario: The button works on its own

- **GIVEN** an expanded sidebar
- **WHEN** the reader presses the collapse button and then presses it again
- **THEN** the sidebar is a rail after the first press and back at its previous width after the second

#### Scenario: A rail entry still has its name

- **GIVEN** a collapsed sidebar
- **WHEN** assistive technology reads a navigation entry in the rail
- **THEN** the entry is announced by the same name it has when the sidebar is expanded

#### Scenario: The phone layout ignores the collapsed state

- **GIVEN** a browser that collapsed the sidebar on a desktop
- **WHEN** a signed-in screen is drawn at phone width
- **THEN** the brand bar and the menu button are drawn as they are for a browser that never collapsed it

### Requirement: A browser chooses whether a task opens as a page or in a dialog

The web interface SHALL offer a per-browser preference, Task opens in, stored in the `tix_task_open`
cookie, whose positions are Page and Dialog. Absent, or any value other than `dialog`, SHALL mean
Page, which SHALL behave exactly as the interface did before the preference existed. The control
SHALL be a switch in the Listings section of Settings that posts without scripting and names the
position in force.

#### Scenario: An untouched browser opens tasks as pages

- **GIVEN** a browser that has never set Task opens in
- **WHEN** the reader opens the task list
- **THEN** the page carries no task dialog
- **AND** the Settings switch for Task opens in reads Page

#### Scenario: Choosing Dialog

- **WHEN** the reader presses the Task opens in switch on Settings
- **THEN** the browser stores `tix_task_open=dialog`
- **AND** the reader is returned to the Listings section with a toast naming the setting
- **AND** the task list carries the task dialog

### Requirement: In Dialog mode a task link opens the task screen over the page

When Task opens in is Dialog, a plain primary click on a link to a task screen, on any screen other
than a task screen, SHALL open that task in a modal dialog over the page instead of navigating. This
SHALL apply to task titles in the list, on board cards and lane cards, to the activity feed, to
search results and to the toast offered after adding a task. The address SHALL change to the task's
address, so that reloading or opening the address elsewhere draws the full task page, and Back SHALL
close the dialog and leave the page under it as it was. Escape, the dialog's Close button and a click
on the backdrop SHALL close it, and focus SHALL return to the element that opened it, or to the same
task's row when that element has been redrawn. A click with a modifier key, a middle click, and any
click in a browser without scripting SHALL open the full task page as before. Under 600px the dialog
SHALL fill the screen.

#### Scenario: Opening and closing from the list

- **GIVEN** Task opens in is Dialog and the reader is on the task list
- **WHEN** they click a task's title
- **THEN** the task screen is drawn in a modal dialog over the list
- **AND** the address is the task's
- **WHEN** they press Escape
- **THEN** the dialog closes, the address is the list's again, and focus is on the title they clicked

#### Scenario: Back closes the dialog

- **GIVEN** a task open in the dialog over a filtered list
- **WHEN** the reader presses the browser's Back
- **THEN** the dialog closes and the filtered list is still on screen, not reloaded

#### Scenario: A deep link is the full page

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader reloads while a task is open in the dialog
- **THEN** the full task page is drawn, with its back link

#### Scenario: A modified click opens a new tab

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader ctrl-clicks a task's title
- **THEN** no dialog opens and the browser opens the full task page as it would for any link

### Requirement: The server draws the task screen alone for the dialog

A request for a task screen carrying the header `X-Tix-Dialog: 1` SHALL be answered with the task
screen's content and its flash, without the layout, the sidebar or the page's live binding, inside
one element marking it as the dialog's answer. Ids the task screen shares with a listing's rows SHALL
be distinct in that answer, so a control in the dialog cannot address an element of the page behind
it. The back link SHALL be a Close button. The same request without the header SHALL be answered
with the full page. Other screens SHALL ignore the header.

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

### Requirement: A browser chooses whether clicking a title edits it

The web interface SHALL offer a per-browser preference, Clicking a title, stored in the
`tix_title_click` cookie, whose positions are Opens the task and Edits the title. Absent, or any
value other than `edit`, SHALL mean Opens the task. The control SHALL be a switch in the Listings
section of Settings that posts without scripting and names the position in force.

#### Scenario: Choosing Edits the title

- **WHEN** the reader presses the Clicking a title switch on Settings
- **THEN** the browser stores `tix_title_click=edit`
- **AND** every task title in the list, on board cards and on lane cards is marked as editable and
  has an open link beside it

### Requirement: A title can be edited where it is listed

When Clicking a title is Edits the title, a plain click on a task's title in the list, on a board
card or on a lane card SHALL turn it into a text field holding the title. Enter, or leaving the
field, SHALL save it through the task's existing edit address with the version the listing was drawn
with; Escape SHALL put the title back unchanged. An unchanged title SHALL send nothing. A refusal,
including a conflict with a newer version and an empty title, SHALL be shown beside the field with
the field kept open. Enter or F2 on a selected row SHALL edit its title. The open link beside the
title SHALL open the task, as a page or in the dialog as Task opens in says. Without scripting the
title SHALL be a link to the task.

#### Scenario: Renaming from the list

- **GIVEN** Clicking a title is Edits the title
- **WHEN** the reader clicks a title, types a new one and presses Enter
- **THEN** the task's title is the new one and the row shows it
- **AND** the task's description and assignee are unchanged

#### Scenario: Somebody else saved first

- **GIVEN** a listing drawn at a task's version 3, and the task since saved at version 4
- **WHEN** the reader renames it from the listing
- **THEN** the save is refused and the reason is shown beside the field
- **AND** the title is unchanged

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
the highest-weighted language it ships, matching an exact tag before its base language, and SHALL
draw English when the header names none it ships. Every screen SHALL carry the chosen language in
the root element's `lang` attribute and SHALL draw its text, flashes, page title, error heading and
hint, and fixed labels in that language. Text a language's catalog lacks SHALL be drawn in English.
The interface SHALL ship English and Bulgarian.

#### Scenario: A Bulgarian browser

- **WHEN** a browser sends `Accept-Language: bg-BG,bg;q=0.9,en;q=0.5` and opens the task screen
- **THEN** the root says `lang="bg"` and the navigation, the flash and the error screen are in Bulgarian

#### Scenario: A language the build does not ship

- **WHEN** a browser sends `Accept-Language: de-DE,fr;q=0.8`
- **THEN** the screen is drawn in English with `lang="en"`

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
helpers, except text inside `code`, `kbd`, `pre` and `samp`. Every language other than English SHALL
hold an entry for every key the interface uses and no entry it does not, and each entry SHALL format
the same number of arguments and carry the same markup as its key. A screen exempted from this check
SHALL be named, with its reason, in the test that makes it.

#### Scenario: A new English literal

- **WHEN** a template gains `<p class="group">Data</p>`
- **THEN** the test suite fails naming the template and the literal

#### Scenario: A missing translation

- **WHEN** a key used by a template has no Bulgarian entry
- **THEN** the test suite fails naming the key and where it is used

### Requirement: Unfamiliar concepts carry an info tip

Beside a concept a newcomer cannot decode from the screen (claims and leases, tokens and scopes,
the filter syntax, recurrence, stars, list, board and lane views, workflows, states and categories,
webhooks, connections, sync, domains, import and export, retention, SSH keys, statistics figures,
the activity feed) the interface SHALL offer an info tip: a disclosure that works without
scripting, from the keyboard and on touch, is named "About" its topic for assistive technology,
explains the concept in one or two plain sentences in the page's language, and names the `docs/`
page that says more where one exists. A tip SHALL NOT link to another origin.

#### Scenario: The view switch explains itself

- **WHEN** a reader opens the tip beside the task screen's view switch
- **THEN** it explains list, board and lanes and names `docs/web-ui.md`, and its button is announced as "About Views"
