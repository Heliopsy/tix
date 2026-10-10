# auth Specification

## Purpose

Establishes who an actor is across no-auth, local accounts and API tokens, and issues, scopes and revokes the credentials each mode needs.

## Requirements

### Requirement: Authentication modes

The system SHALL support three authentication modes over the same data: no-auth, local accounts, and API tokens. No-auth SHALL be the default for a freshly created local database. API tokens SHALL be usable whether or not local accounts are enabled.

#### Scenario: Fresh local database defaults to no-auth

- **WHEN** a user runs any operation against a database file that was just created and never configured for authentication
- **THEN** the operation succeeds without any credential being supplied

#### Scenario: Local accounts enabled

- **WHEN** authentication is configured for local accounts and a request arrives without a credential
- **THEN** the request is rejected as unauthenticated and no domain data is returned

#### Scenario: Tokens work in no-auth mode

- **WHEN** an API token is presented while the deployment is otherwise in no-auth mode
- **THEN** the token is validated and the resulting actor carries only the token's scopes rather than full scope

### Requirement: Synthesized actor in no-auth mode

In no-auth mode the system SHALL synthesize a local actor holding the full scope set, and SHALL still evaluate the authorization policy for every operation. The policy in this mode SHALL always allow, so that no authorization branch exists that is only exercised when authentication is enabled.

#### Scenario: Policy runs in no-auth mode

- **WHEN** an operation is performed in no-auth mode
- **THEN** the authorization policy is consulted for that operation and returns an allow decision

#### Scenario: Actor is attributed

- **WHEN** a mutation is performed in no-auth mode
- **THEN** the resulting audit entry and event record the synthesized local actor rather than an empty actor

#### Scenario: Enabling authentication changes no code path

- **WHEN** a deployment switches from no-auth to local accounts
- **THEN** the same authorization policy decides every operation, with only the actor's scopes differing

### Requirement: Password storage

The system SHALL hash user passwords with argon2id using a per-user random salt, and SHALL NOT store or be able to recover the plaintext password. Password material SHALL NOT appear in API responses, exports, logs, or audit entries.

#### Scenario: Password is hashed on set

- **WHEN** a user account is created or its password is changed
- **THEN** only an argon2id hash and its parameters are persisted, and the plaintext is not retrievable by any operation

#### Scenario: Verification

- **WHEN** a correct password is presented for an existing account
- **THEN** authentication succeeds, and when an incorrect password is presented authentication fails with an authentication error that does not reveal whether the account exists

#### Scenario: Password never exported

- **WHEN** a user record is read through any transport or included in an export
- **THEN** the response contains no password hash and no password field

### Requirement: Session tokens

Successful password authentication SHALL mint an opaque session token. Only a hash of the token SHALL be persisted. Every session SHALL carry an expiry after which it is rejected, and a logout operation SHALL invalidate the session immediately.

#### Scenario: Session issued once

- **WHEN** a user authenticates with a correct password
- **THEN** an opaque session token is returned to the caller and only its hash is stored

#### Scenario: Expired session rejected

- **WHEN** a session token is presented after its expiry time has passed
- **THEN** the request is rejected as unauthenticated

#### Scenario: Logout

- **WHEN** a user logs out
- **THEN** the session is invalidated and any subsequent request presenting the same token is rejected as unauthenticated

### Requirement: Session cookies

When a session is delivered as a browser cookie, the cookie SHALL be set HttpOnly and SHALL carry a SameSite attribute. The cookie SHALL be set Secure whenever the session was established over TLS.

#### Scenario: Cookie attributes over plain HTTP on loopback

- **WHEN** a session cookie is issued over a non-TLS loopback connection
- **THEN** the cookie carries HttpOnly and SameSite and omits Secure

#### Scenario: Cookie attributes over TLS

- **WHEN** a session cookie is issued over a TLS connection
- **THEN** the cookie carries HttpOnly, SameSite, and Secure

### Requirement: API token issuance

An API token's secret value SHALL be displayed exactly once, at creation time, and SHALL be stored only as a hash. Subsequent reads of the token record SHALL return metadata only.

#### Scenario: Secret shown once

- **WHEN** an API token is created
- **THEN** the response contains the full token value and the stored record contains only a hash plus metadata

#### Scenario: Secret not retrievable later

- **WHEN** a previously created token is listed or read
- **THEN** the response contains the token identifier, name, scopes, tenant, optional project, expiry, and last use time, and does not contain the token value

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

### Requirement: API token lifecycle

The system SHALL allow tokens to be revoked, SHALL reject a revoked token immediately, and SHALL record the time a token was last used for authentication.

#### Scenario: Revocation

- **WHEN** a token is revoked
- **THEN** the next request presenting that token is rejected as unauthenticated

#### Scenario: Last use recorded

- **WHEN** a token successfully authenticates a request
- **THEN** the token's last use time is updated to reflect that request

#### Scenario: Revocation is auditable

- **WHEN** a token is created or revoked
- **THEN** an audit entry is written recording the actor and the token identifier, and no token value appears in it

### Requirement: Scope vocabulary

The system SHALL define a coarse, closed scope vocabulary covering task read, task write, task transition, task claim, task delete, project read, project write, workflow read, workflow write, comment write, artifact write, event subscribe, user admin, token admin, webhook admin, audit read, export, import, and sync admin. Every authorized operation SHALL map to one of these scopes.

#### Scenario: Unknown scope refused

- **WHEN** a token is created requesting a scope name outside the vocabulary
- **THEN** the request is rejected as a validation error and no token is created

#### Scenario: Every operation has a scope

- **WHEN** any operation is invoked through any transport
- **THEN** the authorization policy resolves it to exactly one scope from the vocabulary

### Requirement: Human roles

The system SHALL provide the roles viewer, member, and admin, each defined as a fixed composition of scopes. Viewer SHALL hold read scopes only, member SHALL additionally hold the everyday task, comment, and artifact scopes, and admin SHALL additionally hold the administrative scopes.

#### Scenario: Viewer cannot write

- **WHEN** a user whose membership role is viewer attempts to create a task
- **THEN** the request is rejected as unauthorized

#### Scenario: Member cannot administer tokens

- **WHEN** a user whose membership role is member attempts to create an API token for another user
- **THEN** the request is rejected as unauthorized

#### Scenario: Admin can administer

- **WHEN** a user whose membership role is admin manages users, tokens, or webhooks within their tenant
- **THEN** the operations succeed

### Requirement: Least privilege for agent tokens

A token holding only the scopes needed to work a queue, namely task read, task claim, task transition, task write, comment write, artifact write, and event subscribe, SHALL NOT be able to delete a project, delete a task, administer users or tokens, or read the audit log.

#### Scenario: Agent token cannot delete a project

- **WHEN** a token carrying only the queue-working scopes attempts to delete a project
- **THEN** the request is rejected as unauthorized and the project still exists

#### Scenario: Agent token can work the queue

- **WHEN** a token carrying only the queue-working scopes claims the next unblocked task, transitions it, comments on it, and releases it
- **THEN** every one of those operations succeeds

#### Scenario: Agent token cannot read audit

- **WHEN** a token carrying only the queue-working scopes requests the audit log
- **THEN** the request is rejected as unauthorized

### Requirement: Single enforcement point

Authorization SHALL be evaluated in exactly one place in the codebase, reached by every transport. The HTTP layer SHALL only authenticate, resolving identity, tenant, and scopes into the request context, and SHALL NOT make authorization decisions of its own.

#### Scenario: Identical decision across transports

- **WHEN** the same operation is attempted with the same actor once through a direct database call and once through the HTTP API
- **THEN** both attempts produce the same allow or deny outcome and the same error code when denied

#### Scenario: Denial originates in the policy

- **WHEN** an unauthorized operation is attempted over HTTP
- **THEN** the denial is produced by the authorization policy rather than by a route-level check

### Requirement: Authenticator chain

The system SHALL resolve credentials through an ordered chain of authenticators so that additional identity providers can be added without changing callers. SSO and OIDC providers SHALL NOT ship in this version, while the user record SHALL carry the columns an external identity would populate.

#### Scenario: Chain order

- **WHEN** a request carries both a bearer token and a session cookie
- **THEN** exactly one authenticator resolves the actor according to a deterministic, documented order

#### Scenario: No provider configured

- **WHEN** an operator attempts to configure an external identity provider in this version
- **THEN** the configuration is rejected as unsupported and the server continues with the supported modes

#### Scenario: External identity columns present

- **WHEN** a user record is inspected
- **THEN** it carries fields for an external identity provider and subject that are unset in this version

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

### Requirement: A token minted without a stated expiry expires

Where a surface is driven by a person composing a command or a form, minting an API token without stating
an expiry SHALL produce a token that expires, and the window SHALL be the same on every such surface.

The command line SHALL default to ninety days, which SHALL be the window the browser form proposes. The
command line SHALL accept an explicit instruction to mint a non-expiring token, and that instruction SHALL
be the only way to obtain one: an omitted value and an empty value SHALL both take the default.

The default SHALL be stated in the command's own flag help, so that a reader is told what typing nothing
does without consulting a changelog.

The HTTP API SHALL mint a non-expiring token when the request carries no expiry, because an API client
composes that field rather than typing it, and SHALL NOT substitute a default of its own.

#### Scenario: A token minted with no stated expiry expires in ninety days

- **WHEN** a token is minted from the command line with no expiry given
- **THEN** the stored token expires ninety days from the invocation

#### Scenario: A non-expiring token must be asked for

- **WHEN** a token is minted from the command line with the expiry given as `never`
- **THEN** the stored token has no expiry

#### Scenario: An empty expiry is not a request for a non-expiring token

- **WHEN** a token is minted from the command line with the expiry given as an empty value
- **THEN** the stored token expires ninety days from the invocation

#### Scenario: An absolute expiry is still accepted

- **WHEN** a token is minted from the command line with the expiry given as a date
- **THEN** the stored token expires on that date

#### Scenario: The flag help states the default

- **WHEN** the help for the token creation command is read
- **THEN** the expiry flag's own entry states that it defaults to ninety days
- **AND** names the value that mints a non-expiring token

#### Scenario: The API mints a non-expiring token when none is requested

- **WHEN** a token creation request carries no expiry
- **THEN** the minted token has no expiry

### Requirement: Revoking another actor's token is recorded as a different act

Revoking an API token SHALL be recorded in the audit trail against the actor who performed it, and a
revocation of a token held by another actor SHALL be recorded under a different action from a revocation of
the performer's own.

The distinction SHALL be carried by the action, so that it is reachable by the filters the trail is read
through, and SHALL NOT be carried only by a field of the entry's payload.

The entry SHALL name the token and the actor the token was held by.

#### Scenario: Revoking your own token is recorded as your own

- **GIVEN** an actor holding a token of their own
- **WHEN** they revoke it
- **THEN** the trail records the ordinary revocation action against them

#### Scenario: Revoking another actor's token is recorded distinguishably

- **GIVEN** an administrator and a token held by another actor of the same tenant
- **WHEN** the administrator revokes it
- **THEN** the trail records an action different from the one a self-revocation records
- **AND** the entry names the administrator as the actor who performed it
- **AND** the entry names the actor the token was held by

#### Scenario: The distinction can be filtered for

- **WHEN** the trail is filtered by the action recording another actor's revocation
- **THEN** only those revocations are returned
