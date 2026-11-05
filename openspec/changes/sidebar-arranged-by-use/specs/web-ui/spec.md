## ADDED Requirements

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
