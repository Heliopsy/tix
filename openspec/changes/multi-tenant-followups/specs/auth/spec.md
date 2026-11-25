## ADDED Requirements

### Requirement: A cross-host switch uses a single-use ticket

A switch to a tenant served on another host SHALL hand over a ticket that is single use, expires within 60 seconds, is stored only as a hash, and is bound to the account, the target tenant and the target host. It SHALL be carried in a request body, never a URL.

#### Scenario: Ticket replayed

- **WHEN** a redeemed switch ticket is presented again
- **THEN** it is refused and no session is minted

#### Scenario: Ticket presented to another host

- **WHEN** a ticket minted for one host is presented to a different host
- **THEN** it is refused and no session is minted

#### Scenario: Redeeming asks before signing in

- **WHEN** a valid ticket reaches the target host
- **THEN** a page naming the account and the tenant asks for confirmation, and only its submission mints the session
