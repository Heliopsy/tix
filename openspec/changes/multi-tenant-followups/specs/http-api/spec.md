## MODIFIED Requirements

### Requirement: Tenant switch routes

The API SHALL serve `POST /auth/switch`, taking a target tenant key and an optional code, and answering with a session for the same host, a code challenge, or a switch ticket and target address for a tenant served at another host. It SHALL serve `POST /auth/switch/redeem`, taking a ticket.

#### Scenario: Switch on the same host

- **WHEN** a session on the shared host posts a switch to a tenant the person belongs to
- **THEN** the answer carries a new session for that tenant and the old session no longer authenticates

#### Scenario: A token switches

- **WHEN** a request authenticated by an API token posts a switch
- **THEN** it is refused as invalid and no session is minted

#### Scenario: Switch to a tenant at its own address

- **WHEN** a session on HO's shared host posts a switch to `away`, which has its own domain
- **THEN** the answer carries a switch ticket and `away`'s address, no session is minted, and the HO session still authenticates
