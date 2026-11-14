## ADDED Requirements

### Requirement: A token can be minted to require a claim

An API token SHALL carry `require_claim`, false unless set when the token is minted. Every surface
that mints a token SHALL offer it, and every listing and the issued token SHALL report it. A token
created before the setting existed SHALL read as false.

#### Scenario: Mint a token that requires a claim

- **WHEN** an admin runs `tix token create agent-1 --scope task:write --require-claim`
- **THEN** the issued token and `tix token ls` report `require_claim` true

#### Scenario: A token minted without it does not require a claim

- **WHEN** a token is created over the HTTP API with no `require_claim` field
- **THEN** the token reports `require_claim` false and behaves exactly as tokens did before

### Requirement: Rotating a token keeps its claim requirement

Rotating a token SHALL give the replacement the same `require_claim` as the token it replaces.

#### Scenario: A rotated token still requires a claim

- **WHEN** a token minted with `require_claim` is rotated
- **THEN** the new token reports `require_claim` true
