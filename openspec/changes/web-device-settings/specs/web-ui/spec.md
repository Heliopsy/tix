## ADDED Requirements

### Requirement: Settings can be searched

With scripting, Settings SHALL offer a search box that keeps only the rows whose name, help or explanation
holds every word typed, and the sections left with a row, saying so when nothing matches.

#### Scenario: Searching for motion

- **WHEN** a reader types `motion` into the search box
- **THEN** the Reduce motion row is shown and sections with no matching row are hidden

### Requirement: Each setting says where it is kept

Every setting on Settings SHALL carry a label: On this device for a browser cookie or storage, Synced to
your account for what the server keeps with the account. The Tenant screen SHALL say its settings apply to
the whole tenant.

#### Scenario: A cookie preference

- **WHEN** a reader opens Settings
- **THEN** the Colour rows by row is labelled On this device and the Timezone row Synced to your account

### Requirement: Start screen

Settings SHALL offer a Start screen choice of Projects, Tasks, My work or Inbox, kept in `tix_start`.
`/` SHALL redirect to the chosen screen, and a sign-in that follows no link SHALL land there.

#### Scenario: Opening on the inbox

- **GIVEN** a browser that chose Inbox
- **WHEN** it opens `/`
- **THEN** it is redirected to `/inbox`

### Requirement: Reduce motion override

Settings SHALL offer a Reduce motion switch, kept in `tix_motion`, which puts `data-motion="reduce"` on
`<html>`; the interface SHALL then drop animation, transitions, cross-fades and smooth scrolling as it does
under `prefers-reduced-motion: reduce`.

#### Scenario: Turned on

- **WHEN** a reader turns Reduce motion on
- **THEN** every page's root element carries `data-motion="reduce"`

### Requirement: Browser notifications for new inbox items

With scripting, Settings SHALL offer a switch, kept in browser storage, that asks the browser for
permission and then shows a notification for each new inbox item while a tix tab is open and not being
looked at, each item once. A second switch SHALL keep the notification to saying something arrived.

#### Scenario: Hidden content

- **GIVEN** a reader with notifications on and Hide what they say on
- **WHEN** somebody mentions them while their tab is in the background
- **THEN** a notification says only that something new reached the inbox
