## ADDED Requirements

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
