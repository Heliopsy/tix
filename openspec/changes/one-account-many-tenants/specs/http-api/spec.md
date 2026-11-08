## ADDED Requirements

### Requirement: whoami lists the caller's memberships

`GET /whoami` SHALL report the caller's account identifier and, for a session or a token carrying the `account:read` scope, every tenant the account is a member of with its key, name, role, actor and host. The `*` scope SHALL NOT imply `account:read`.

#### Scenario: Session caller in two tenants

- **WHEN** a person who is a member of HO and `test` calls whoami with a session
- **THEN** the answer lists both tenants with the role held in each

#### Scenario: Token without the scope

- **WHEN** a token carrying `*` but not `account:read` calls whoami
- **THEN** the answer describes the token's actor and lists no memberships

### Requirement: A shared host serves the tenant the header or credential names

A host no tenant domain claims, and a host whose domain is marked shared, SHALL be resolved alike, once per request before authentication: the tenant the `X-Tix-Tenant` header names, else the tenant of the session or token presented, else the domain's tenant or, on an unclaimed host, the default tenant.

#### Scenario: Sign-in to a named tenant on the shared host

- **WHEN** a sign-in on the shared host carries `X-Tix-Tenant: test`
- **THEN** the sign-in is checked against tenant `test` and the session minted belongs to it

#### Scenario: Credential of another tenant on a shared domain

- **WHEN** a session of tenant `test` is presented, with no header, on tix.z09.org, a shared domain of HO
- **THEN** the request is served as tenant `test`, and reads nothing of HO

#### Scenario: Nothing names a tenant

- **WHEN** a request with no header and no credential reaches a shared domain of HO
- **THEN** it is served as HO

#### Scenario: Header and credential disagree

- **WHEN** a request on the shared host names tenant A in the header and presents a credential of tenant B
- **THEN** it is refused as a credential not valid for the request's tenant

### Requirement: A bound host serves its own tenant alone

On a host claimed by a domain not marked shared, a header naming another tenant SHALL be refused before authentication, and a credential of another tenant SHALL be refused as not valid for the host.

#### Scenario: Header on a bound host

- **WHEN** a request to a host claimed by tenant HO, not marked shared, carries `X-Tix-Tenant: test`
- **THEN** it is refused and nothing is read from either tenant

#### Scenario: Another tenant's session on a bound host

- **WHEN** a session of tenant `test` is presented on a host bound to HO
- **THEN** it is refused as unauthenticated

### Requirement: A tenant with its own address is not served on a shared host

A shared host SHALL refuse a header naming a tenant that has a domain of its own not marked shared, other than the host's own tenant, and SHALL ignore a credential of such a tenant, so that tenant's sessions live at its own address.

#### Scenario: Tenant at its own address

- **WHEN** tenant `test` has its own domain and a request on a shared host names it in the header
- **THEN** it is refused as forbidden

### Requirement: Tenant switch routes

The API SHALL serve `POST /auth/switch`, taking a target tenant key and an optional code, and answering with a session for the same host or a code challenge. In the later phase it SHALL also answer with a switch ticket and target address for another host, and serve `POST /auth/switch/redeem`, taking a ticket.

#### Scenario: Switch on the same host

- **WHEN** a session on the shared host posts a switch to a tenant the person belongs to
- **THEN** the answer carries a new session for that tenant and the old session no longer authenticates

#### Scenario: A token switches

- **WHEN** a request authenticated by an API token posts a switch
- **THEN** it is refused as invalid and no session is minted

### Requirement: Domain update routes

The API SHALL serve `PATCH /domains/{hostname}`, taking `shared`, and `GET /tenants/{ref}/domains`, both for a global admin only, and SHALL answer anybody else as forbidden.

#### Scenario: Tenant admin token

- **WHEN** a token of a tenant admin carrying `*` patches a domain's `shared`
- **THEN** it is answered 403 and the domain is unchanged

### Requirement: Account log routes

The API SHALL serve `GET /account/log` for the caller's own account and `GET /accounts/{email}/log` for a global admin, both keyset paginated.

#### Scenario: Global admin reads a person's log

- **WHEN** a global admin requests the log of villy@
- **THEN** the answer lists villy@'s account changes across every tenant
