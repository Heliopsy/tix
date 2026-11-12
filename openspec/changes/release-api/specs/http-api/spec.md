## ADDED Requirements

### Requirement: The release check is readable over the API

`GET /api/v1/release` SHALL answer any authenticated caller with `running`, `latest`, `status`, `newer`,
`checked` and `failed`, where `status` is one of `newer`, `current`, `unknown`, `failed` or `pending` as the
web's release screen names it. It SHALL read the same cached check as the browser interface and never wait
on the release feed. An unauthenticated request SHALL be refused with 401.

#### Scenario: A newer release

- **GIVEN** a server running 0.58.0 whose last check found 0.59.0
- **WHEN** a signed-in caller reads `GET /api/v1/release`
- **THEN** the answer has `running` 0.58.0, `latest` 0.59.0, `status` newer and `newer` true

#### Scenario: No credential

- **WHEN** a request without a credential reads `GET /api/v1/release`
- **THEN** it is refused with 401 and carries no release information

### Requirement: The release route offers an upgrade only when one exists

The answer SHALL carry `notes`, the newest release's notes as Markdown with links removed and the leading
heading naming the release dropped, and `methods`, one entry per install method with `id` `binary`,
`container` or `go`, a `name`, a `help` and the `command` that upgrades it to `latest`. Both SHALL be empty
unless `newer` is true.

#### Scenario: Current installation

- **GIVEN** a server whose running version is the newest release
- **WHEN** a signed-in caller reads `GET /api/v1/release`
- **THEN** `status` is current, `notes` is empty and `methods` is an empty list

#### Scenario: Commands carry the version on offer

- **GIVEN** a server whose last check found 0.59.0, newer than the running version
- **WHEN** a signed-in caller reads `GET /api/v1/release`
- **THEN** `methods` lists binary with `tix update`, container with `podman pull ghcr.io/heliopsy/tix:0.59.0` and go with `go install github.com/heliopsy/tix@v0.59.0`

### Requirement: The release route carries the bundled changelog

The answer's `changes` SHALL hold the changelog embedded in the binary as the release screen selects it:
`running`, the running version's entry, present only for a tagged release the changelog lists; `earlier`,
up to ten entries before it, or the eleven newest when `running` is absent; and `untagged`. Each entry SHALL
carry `version`, `date` and `notes` as Markdown with links removed.

#### Scenario: A tagged build

- **GIVEN** a server running a tagged release the changelog lists, with more than ten releases before it
- **WHEN** a signed-in caller reads `GET /api/v1/release`
- **THEN** `changes.running` is that release's entry and `changes.earlier` lists the ten releases before it

#### Scenario: A development build

- **GIVEN** a server running `dev`
- **WHEN** a signed-in caller reads `GET /api/v1/release`
- **THEN** `changes.running` is absent, `changes.untagged` is true and `changes.earlier` lists the eleven newest entries
