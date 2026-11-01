## ADDED Requirements

### Requirement: The project settings change the key after a confirmation

The project settings form SHALL offer the project's key alongside its name and description, with an
explanation that task references change and old links redirect. Submitting a different key SHALL NOT
change anything at once: the reader SHALL be shown a confirmation naming the old and new key with
example task references before and after, and SHALL change the key only by confirming. The
confirmation SHALL carry every other submitted field so confirming also saves them, and SHALL work
without scripting. An icon image uploaded in the same save cannot be carried to a second screen, so
the confirmation SHALL ask for it again rather than drop it. Submitting the current key SHALL save
without a confirmation.

#### Scenario: A changed key asks first

- **GIVEN** project `infra` holding task `infra-1`
- **WHEN** the reader submits the settings with key `ops`
- **THEN** a confirmation shows `infra-1` becoming `ops-1` and the project is unchanged

#### Scenario: Confirming renames and lands on the new board

- **WHEN** the reader confirms the change of `infra` to `ops`
- **THEN** the key is `ops` and the reader is taken to `/projects/ops`

#### Scenario: The icon chosen with a new key is kept

- **WHEN** the reader picks a built-in icon and a new key in one save and confirms
- **THEN** the project carries both the new key and the icon
- **AND** had the reader uploaded an image instead, the confirmation asks for the file again

### Requirement: Old project and task URLs redirect to the new ones

A GET request for a project screen or a task screen addressed by a kept alias SHALL be answered with
a permanent redirect to the same screen addressed by the current key, keeping the rest of the path
and the query. A form post addressed by an alias SHALL be served without a redirect.

#### Scenario: An old project URL redirects

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** the reader opens `/projects/infra`
- **THEN** the response is a 301 to `/projects/ops`

#### Scenario: An old task URL redirects

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** the reader opens `/tasks/infra-8`
- **THEN** the response is a 301 to `/tasks/ops-8`
