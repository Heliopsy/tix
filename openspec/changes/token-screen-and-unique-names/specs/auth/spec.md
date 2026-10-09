## ADDED Requirements

### Requirement: A live API token's name identifies exactly one token

Within one tenant, no two API tokens that are neither revoked nor deleted SHALL carry the same name. A
creation naming a name a live token of that tenant already holds SHALL be refused as a conflict, and no
token SHALL be created.

The rule SHALL be enforced in the service layer, so every surface that mints a token — the command line,
the HTTP API and the browser — is held to it, and SHALL additionally be enforced by the store, so two
creations that race cannot both pass the check and both insert.

A revoked token SHALL keep its name, so a listing can still say which credential stopped working, and
SHALL release it, so that revoking a token and issuing its replacement under the same name is permitted.
This is the ordinary way a credential is rotated.

Uniqueness SHALL be scoped to the tenant and no wider. One tenant SHALL NOT be able to learn, or
constrain, what another has named its credentials.

The refusal SHALL name the submitted field it concerns, so a surface rendering a form can present it
against the control that caused it.

#### Scenario: A second live token cannot take a name

- **GIVEN** a tenant holding a live token named `ci`
- **WHEN** a second token named `ci` is created in that tenant
- **THEN** the creation is refused as a conflict
- **AND** the tenant still holds exactly one token named `ci`

#### Scenario: Revoking a token frees its name

- **GIVEN** a tenant holding a live token named `ci`
- **WHEN** that token is revoked and a new token named `ci` is created
- **THEN** the new token is created
- **AND** it is a different token from the revoked one

#### Scenario: Two tenants may each name a token the same

- **GIVEN** a tenant holding a live token named `ci`
- **WHEN** a token named `ci` is created in a different tenant
- **THEN** the creation succeeds

#### Scenario: Upgrading a database that already holds duplicates

- **GIVEN** a database holding two live tokens of one tenant both named `ci`
- **WHEN** the schema is migrated to the version that requires uniqueness
- **THEN** both tokens still authenticate
- **AND** the two names differ
- **AND** the older of the two still carries the name `ci`

#### Scenario: The refusal names the field

- **WHEN** a creation is refused because the name is taken
- **THEN** the refusal names the `name` field

### Requirement: A refused token creation names the field it refused

A validation failure on token creation SHALL name the submitted field it concerns: the name where no name
was given, and the scope list where no scope was given.

This exists so that the attribution lives with the rule rather than being reconstructed by matching
message text at a presentation layer.

#### Scenario: A creation with no name

- **WHEN** a token is created with no name
- **THEN** the refusal is a validation error naming the `name` field

#### Scenario: A creation with no scope

- **WHEN** a token is created with no scope
- **THEN** the refusal is a validation error naming the `scopes` field

## MODIFIED Requirements

### Requirement: API token scoping

Every API token SHALL be bound to exactly one tenant, MAY be further restricted to a single project, and
MAY carry an expiry. A token SHALL grant no more than its declared scopes, and a token restricted to a
project SHALL NOT authorize operations on resources outside that project.

An expiry SHALL be expressible on every surface that can mint a token, since an expiry is the only control
that bounds the damage of a token that leaks unnoticed, and a surface that cannot set one can only mint
credentials that never expire.

A token carrying no expiry SHALL authenticate until it is revoked, and every surface that displays a
token's expiry SHALL say that it has none rather than leaving the value blank, so that "no expiry" is
distinguishable from an expiry the display failed to render.

Confinement SHALL be the default rather than a property of the operation being asked for. The authorization
policy SHALL name the operations a project-restricted token may perform, being those whose subject belongs
to a single project, and SHALL refuse a project-restricted token every other operation outright. An
operation that reads or writes state belonging to the tenant rather than to one project — exporting,
importing, reading the audit log, subscribing to the tenant event stream, administering tenants, users,
tokens, webhooks, sync sources or retention, and rewriting workflow definitions — SHALL therefore be
refused to a project-restricted token, whether or not the call names a project.

#### Scenario: Project-restricted token

- **WHEN** a token restricted to project A is used to read a task in project B of the same tenant
- **THEN** the request is rejected as unauthorized

#### Scenario: Project-restricted token cannot export the tenant

- **WHEN** a token restricted to project A and carrying the export scope requests an export
- **THEN** the request is rejected as unauthorized and no tenant data is produced

#### Scenario: Project-restricted token cannot read the audit log

- **WHEN** a token restricted to project A and carrying the audit read scope lists audit entries
- **THEN** the request is rejected as unauthorized, because an audit entry names no project and so cannot be confined to one

#### Scenario: Project-restricted token cannot widen itself

- **WHEN** a token restricted to project A and carrying the token admin scope mints a new token naming no project
- **THEN** the request is rejected as unauthorized and no token is created

#### Scenario: Expired token

- **WHEN** a token is presented after its expiry
- **THEN** the request is rejected as unauthenticated

#### Scenario: An expiry can be set wherever a token can be minted

- **WHEN** a token is minted from the command line, the HTTP API or the browser
- **THEN** an expiry can be given as part of that request

#### Scenario: A token with no expiry is shown as having none

- **GIVEN** a token carrying no expiry
- **WHEN** its expiry is displayed
- **THEN** the display says the token does not expire

#### Scenario: Scope not granted

- **WHEN** a token without the task delete scope attempts to delete a task
- **THEN** the request is rejected as unauthorized and the task is unchanged
