## RENAMED Requirements

- FROM: `### Requirement: The administration screens fold under one Admin group`
- TO: `### Requirement: The administration screens fold under one Manage group`

## MODIFIED Requirements

### Requirement: The sidebar is ordered by use

The sidebar SHALL draw, from the top: the tenant's name with the rail toggle, Search, then Inbox, My
work, Tasks, Projects, the reader's starred projects under Projects, Templates, Activity and Statistics
as top-level entries in that order, then Saved filters and Recently viewed, then the Manage group, then
the account menu and the version.

#### Scenario: Entries in order of use

- **GIVEN** a signed-in reader
- **WHEN** any screen is drawn
- **THEN** the navigation lists Inbox, My work, Tasks, Projects, Templates, Activity and Statistics in that order, each at the top level

### Requirement: The administration screens fold under one Manage group

Trash and the configuration screens SHALL be one group headed Manage, folded by default, keeping the
id `admin` in the `tix_sidebar` cookie. Trash SHALL be listed only while the tenant's trash is on; the
configuration screens only with Advanced screens on; with neither, no Manage group SHALL be drawn. A
folded group SHALL still show the entry for the page being read. A cookie folding both Configure and
Data SHALL fold Manage, and one leaving either open SHALL open it.

#### Scenario: Admin starts folded

- **GIVEN** a browser with no sidebar cookie
- **WHEN** any screen is drawn
- **THEN** the group, headed Manage and still folded as `admin`, is drawn folded

#### Scenario: A fold saved while the group was called Admin

- **GIVEN** a sidebar cookie holding `fold=admin`
- **WHEN** any screen is drawn
- **THEN** Manage is drawn folded

#### Scenario: A browser that had Configure open

- **GIVEN** a sidebar cookie from an older build folding Data but not Configure
- **WHEN** any screen is drawn
- **THEN** Manage is drawn open

#### Scenario: A browser that had both folded

- **GIVEN** a sidebar cookie from an older build folding Configure and Data
- **WHEN** any screen is drawn
- **THEN** Manage is drawn folded

#### Scenario: Trash off

- **GIVEN** a tenant whose trash is turned off
- **WHEN** any screen is drawn
- **THEN** Manage has no Trash entry

## ADDED Requirements

### Requirement: Templates is a main sidebar entry for every reader of tasks

The sidebar SHALL draw Templates as a top-level entry, outside the Manage group, for every reader who
can read tasks, whether or not Advanced screens is on. The command palette SHALL offer it in both modes
without a group hint, and SHALL hint every configuration screen as Manage.

#### Scenario: A member with Advanced screens off

- **GIVEN** a member who turned Advanced screens off
- **WHEN** any screen is drawn
- **THEN** Templates is listed right after Projects and its starred projects, and opens the Templates screen

#### Scenario: Not inside Manage

- **GIVEN** a tenant administrator with Advanced screens on
- **WHEN** any screen is drawn
- **THEN** Templates is listed once, outside the Manage group, and the palette hints Workflows as Manage
