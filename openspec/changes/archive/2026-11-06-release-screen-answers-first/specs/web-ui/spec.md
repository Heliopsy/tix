## ADDED Requirements

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
