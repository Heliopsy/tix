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
