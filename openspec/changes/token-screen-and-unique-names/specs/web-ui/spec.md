## ADDED Requirements

### Requirement: A refused submission is answered by the form, not by an error page

Where a browser submission is refused for a reason the reader can act on by changing what they submitted —
a missing or invalid value, or a conflict with something that already exists — the screen SHALL re-render
the form it came from, carrying every value the submission held and the refusal's message beside the
control it concerns.

This SHALL be the response to the ordinary form submission, so that it holds with scripting turned off. A
client-side validation attribute on a control MAY be present as a convenience, and SHALL NOT be the thing
performing the check.

A refusal the reader cannot act on by editing the submission — an authentication or authorization failure,
or an internal fault — SHALL continue to be answered by the error screen, which is the only surface that
can explain it.

The message SHALL be associated with its control for assistive technology, and the control SHALL be marked
as invalid.

#### Scenario: A submission with a missing required value comes back

- **GIVEN** the API token screen's issue form
- **WHEN** a submission naming a token name but selecting no scope is sent
- **THEN** the token screen is rendered again rather than the error screen
- **AND** the name input still holds the submitted name
- **AND** the message explaining the refusal appears with the scope control

#### Scenario: A conflicting submission comes back

- **GIVEN** a tenant already holding a live token named `ci`
- **WHEN** a submission naming `ci` is sent
- **THEN** the token screen is rendered again with the name still in its input
- **AND** the message says the name already exists

#### Scenario: An unauthorized submission still reaches the error screen

- **WHEN** a submission is refused because the caller lacks the required scope
- **THEN** the error screen is rendered

## MODIFIED Requirements

### Requirement: Tenant, domain, user, and token administration

The web UI SHALL provide administration screens for tenants and their memberships, for domain mappings, for
users, and for API tokens, restricted to callers holding the corresponding administrative scopes.

The token screen's issue form SHALL be able to set every property of a token the service accepts and the
listing displays, which SHALL include its scopes and its expiry. The expiry control SHALL offer a set of
durations and an explicit "never expires" choice, and SHALL propose an expiry rather than proposing no
expiry. A value the control does not offer SHALL be refused rather than read as no expiry.

The token listing SHALL show, as optional columns behaving like every other optional column, when each
token was created and when it expires, and SHALL show whether each token still authenticates: active,
expired, or revoked with the time it was revoked.

A revoked token SHALL NOT offer a revocation control. Revocation, being irreversible and ending a
credential something may be holding, SHALL be behind a confirming disclosure that names the token it ends
and says what revoking does.

A newly issued token's value SHALL be presented in a region of its own, distinct from the interface's
transient confirmation message, readable in full without being cropped, selectable, and accompanied by a
copy control. The value SHALL remain readable in full when no scripting is available.

#### Scenario: Administrative screens require scope

- **WHEN** a caller without administrative scope requests an administration screen
- **THEN** access is refused and no administrative data is rendered

#### Scenario: Token is issued once

- **WHEN** an authorized administrator creates an API token
- **THEN** the token value is displayed once at creation and is not retrievable afterwards

#### Scenario: The issued value is not presented as a confirmation message

- **WHEN** a token has just been issued
- **THEN** its value is in its own region rather than in the transient confirmation message
- **AND** a copy control points at it

#### Scenario: An expiry set on the form reaches the token

- **WHEN** an administrator issues a token choosing an expiry of seven days
- **THEN** the token's stored expiry is seven days ahead
- **AND** the listing's expires column shows it

#### Scenario: A token issued with no expiry says so

- **WHEN** an administrator issues a token choosing never expires
- **THEN** the listing's expires column says the token does not expire

#### Scenario: The listing says when a token was created

- **WHEN** the token listing is read
- **THEN** each row shows when that token was created

#### Scenario: A revoked token is shown as revoked and offers no revocation

- **GIVEN** a revoked token
- **WHEN** the token listing is read
- **THEN** the row is marked revoked
- **AND** the row offers no control to revoke it

#### Scenario: Revocation is confirmed before it happens

- **GIVEN** a live token
- **WHEN** the token listing is read
- **THEN** reaching the revoke control requires opening a disclosure that names the token and says that
  whatever holds it stops authenticating

#### Scenario: Domain mapping is manageable

- **WHEN** an authorized administrator adds a domain mapping for a tenant
- **THEN** the mapping appears in the domain list and its verification state is shown
