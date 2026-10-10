## ADDED Requirements

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
