## ADDED Requirements

### Requirement: The sidebar's groups fold

The sidebar's Saved filters, Recently viewed, Configure and Data groups SHALL each have a heading that,
with scripting, is a button folding and unfolding the group, exposing `aria-expanded` and naming the
group's body with `aria-controls`. Which groups are folded SHALL be kept per browser in the
`tix_sidebar` cookie and SHALL be drawn by the server, so a folded group is folded from the first
paint. A folded group SHALL still show the entry for the page being read. On the collapsed rail a fold
SHALL have no effect. Without scripting the heading SHALL be plain text and every group open.

#### Scenario: Folding Configure is remembered

- **GIVEN** a reader who presses the Configure heading
- **WHEN** they open another screen
- **THEN** Configure is drawn folded from the first paint, its heading reporting it collapsed

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

### Requirement: Navigating does not move the sidebar

A boosted navigation SHALL draw the sidebar in its final state in the first frame after the swap: no
entry, control or label in it SHALL change position between the swap and the scripts that run after
it. The sidebar's width and rail SHALL be kept per browser in the `tix_sidebar` cookie and drawn by the
server. A control in the sidebar that needs scripting SHALL be shown by a class set on the document
root before the body is parsed, not revealed by its script after a swap. The recently viewed list SHALL
be kept across a boosted navigation rather than redrawn from empty, and opening one of its entries SHALL
be a boosted navigation like any other sidebar entry.

#### Scenario: Going from Tasks to Projects

- **GIVEN** a reader on Tasks with a recently viewed list and the Configure group showing
- **WHEN** they follow Projects in the sidebar
- **THEN** every sidebar entry is at the position it had before the click in every frame of the navigation

#### Scenario: A stored rail survives a navigation without a flash

- **GIVEN** a reader who collapsed the sidebar
- **WHEN** they follow any sidebar entry
- **THEN** the page is drawn with the rail in its first frame

#### Scenario: Earlier browser storage is adopted

- **GIVEN** a browser that kept the sidebar's width and rail in `localStorage` under an earlier build
- **WHEN** it loads a page
- **THEN** the cookie holds the same width and rail and the old keys are removed
