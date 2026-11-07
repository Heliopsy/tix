## ADDED Requirements

### Requirement: whoami lists the caller's memberships

`GET /whoami` SHALL report the caller's account identifier and, for a session or a token carrying the `account:read` scope, every tenant the account is a member of with its key, name, role, actor and host. The `*` scope SHALL NOT imply `account:read`.

#### Scenario: Session caller in two tenants

- **WHEN** a person who is a member of HO and `test` calls whoami with a session
- **THEN** the answer lists both tenants with the role held in each

#### Scenario: Token without the scope

- **WHEN** a token carrying `*` but not `account:read` calls whoami
- **THEN** the answer describes the token's actor and lists no memberships

### Requirement: A host no tenant domain claims serves the credential's tenant

On a host that no tenant domain claims, the tenant SHALL be taken from the `X-Tix-Tenant` header or the presented credential, and SHALL default to the configured default tenant. On a host a tenant domain claims, a header naming another tenant SHALL be refused before authentication.

#### Scenario: Sign-in to a named tenant on the shared host

- **WHEN** a sign-in on the shared host carries `X-Tix-Tenant: test`
- **THEN** the sign-in is checked against tenant `test` and the session minted belongs to it

#### Scenario: Header on a bound host

- **WHEN** a request to a host claimed by tenant HO carries `X-Tix-Tenant: test`
- **THEN** it is refused and nothing is read from either tenant

#### Scenario: Header and credential disagree

- **WHEN** a request on the shared host names tenant A in the header and presents a credential of tenant B
- **THEN** it is refused as a credential not valid for the request's tenant

### Requirement: Tenant switch routes

The API SHALL serve `POST /auth/switch`, taking a target tenant key and answering with a session for the same host or a switch ticket and target address for another host, and `POST /auth/switch/redeem`, taking a ticket.

#### Scenario: Switch on the same host

- **WHEN** a session on the shared host posts a switch to a tenant the person belongs to
- **THEN** the answer carries a new session for that tenant and the old session no longer authenticates
