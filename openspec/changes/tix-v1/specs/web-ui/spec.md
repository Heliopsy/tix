## ADDED Requirements

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

#### Scenario: Invalid workflow is refused

- **WHEN** a workflow edit would leave tasks in a state the workflow no longer defines
- **THEN** the change is refused with an explanation and the workflow is unchanged

#### Scenario: Field definition is created

- **WHEN** an authorized user defines a typed custom field
- **THEN** the field appears on task detail screens for that project and values are validated against its type

### Requirement: Tenant, domain, user, and token administration

The web UI SHALL provide administration screens for tenants and their memberships, for domain mappings, for users, and for API tokens and their scopes, restricted to callers holding the corresponding administrative scopes.

#### Scenario: Administrative screens require scope

- **WHEN** a caller without administrative scope requests an administration screen
- **THEN** access is refused and no administrative data is rendered

#### Scenario: Token is issued once

- **WHEN** an authorized administrator creates an API token
- **THEN** the token value is displayed once at creation and is not retrievable afterwards

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

The web UI SHALL provide screens for exporting a snapshot, importing a snapshot with mode selection and dry run, and running or refreshing an external import with its mapping and dry run.

#### Scenario: Export is downloadable

- **WHEN** an authorized user exports a tenant from the web UI
- **THEN** a snapshot document is produced in the selected format

#### Scenario: Dry run is shown before import

- **WHEN** a user uploads a snapshot and requests a dry run
- **THEN** the planned creations, updates, and skips are displayed and nothing is written

#### Scenario: External import reports its result

- **WHEN** an external import is run from the web UI
- **THEN** the created, updated, skipped, and lossy results are displayed

### Requirement: Live activity feed

The web UI SHALL provide an activity feed showing recent events for the tenant, updating as new events arrive when JavaScript is available.

#### Scenario: Feed shows recent activity

- **WHEN** a user opens the activity feed
- **THEN** recent events for the tenant are listed in reverse chronological order

#### Scenario: Feed updates live

- **WHEN** another user changes a task while the feed is open with JavaScript enabled
- **THEN** the change appears in the feed without the user reloading the page

#### Scenario: Feed is readable without JavaScript

- **WHEN** the feed is opened with JavaScript disabled
- **THEN** recent events are still listed, refreshed on page load

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

Every web UI screen SHALL be usable on a narrow viewport typical of a phone, with no horizontal page scrolling and with all controls reachable.

#### Scenario: Board at phone width

- **WHEN** the project board is viewed at a phone-width viewport
- **THEN** the columns remain navigable and the page does not scroll horizontally

#### Scenario: Forms at phone width

- **WHEN** a task detail form is viewed at a phone-width viewport
- **THEN** every field and the submit control are reachable and operable

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

### Requirement: Display preferences live in one always-visible settings menu

The browser interface SHALL gather the preferences that apply to every screen -- the colour scheme
and whether the administrative screens are shown -- into a single settings control that is present
in the sidebar on every signed-in screen. Every control within it SHALL be a plain form that works
with scripting disabled, and the menu SHALL render under every colour scheme on offer, including the
low contrast one. Preferences scoped to one listing, such as which columns it shows, SHALL stay on
that listing rather than moving into the settings menu.

#### Scenario: The menu is on every screen

- **WHEN** any signed-in screen is rendered
- **THEN** the sidebar carries one settings control gathering the theme and the advanced toggle

#### Scenario: Every scheme renders it

- **WHEN** the interface is rendered under the system, light, dark, and low contrast schemes
- **THEN** the settings menu renders in each, showing the current scheme as the chosen one

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
