## ADDED Requirements

### Requirement: SCIM endpoints are served per tenant under /scim/v2

`tix serve` SHALL answer SCIM 2.0 under `/scim/v2` for the tenant the request resolves to, by host or, on a shared host, by the bearer token, exactly as the API resolves it. Every response SHALL carry `application/scim+json`.

#### Scenario: Discovery on a tenant's host

- **WHEN** a provider sends `GET /scim/v2/ServiceProviderConfig` with a provisioning token
- **THEN** it receives 200 with the service provider configuration, `patch.supported` true and `bulk.supported` false

#### Scenario: Resource types and schemas

- **WHEN** a provider reads `/scim/v2/ResourceTypes` and `/scim/v2/Schemas`
- **THEN** both list the User and Group resources and their schemas

### Requirement: Only a provisioning token reaches the SCIM endpoints

The SCIM endpoints SHALL accept only a bearer token carrying `scim:provision`. A request with no valid credential SHALL get 401 and one with a credential lacking the scope SHALL get 403, both as SCIM errors.

#### Scenario: A token without the scope is refused

- **WHEN** a token carrying `*` calls `GET /scim/v2/Users`
- **THEN** the answer is 403 with the SCIM error schema

#### Scenario: No credential

- **WHEN** a request without an Authorization header calls `GET /scim/v2/Users`
- **THEN** the answer is 401

### Requirement: A tenant switches SCIM on and off

A tenant SHALL be able to turn SCIM provisioning on and off, off by default. While it is off every SCIM endpoint SHALL answer 404 to an authenticated provisioning token, and the token SHALL be kept.

#### Scenario: SCIM is off

- **WHEN** a provisioning token calls `GET /scim/v2/Users` on a tenant with SCIM off
- **THEN** the answer is 404 with a SCIM error saying provisioning is off

### Requirement: Users are listed with filters and paging

`GET /scim/v2/Users` SHALL list the tenant's members as a ListResponse, filtered by `userName eq`, `externalId eq`, `id eq`, `emails.value eq` or `emails[type eq "work"].value eq`, terms joined with `and`, paged by `startIndex` and `count`. An unsupported filter SHALL be a 400 `invalidFilter`.

#### Scenario: Filter by userName

- **WHEN** a provider lists `Users?filter=userName eq "ann@example.com"`
- **THEN** the answer holds exactly the user with that userName and `totalResults` 1

#### Scenario: Page through users

- **WHEN** a provider lists `Users?startIndex=2&count=1` on a tenant with three members
- **THEN** the answer holds the second member, `startIndex` 2, `itemsPerPage` 1 and `totalResults` 3

### Requirement: Creating a user links an existing account or creates one without a password

`POST /scim/v2/Users` SHALL link an existing account whose address matches the user's email, granting a membership when it has none, and otherwise SHALL create an account with no password. A user already provisioned in the tenant SHALL be refused with 409 `uniqueness`.

#### Scenario: Create a new person

- **WHEN** a provider posts a user `bob@example.com` that no account has
- **THEN** the answer is 201, the person is a `viewer` member with no password, and the externalId is kept

#### Scenario: Link an existing account

- **WHEN** a provider posts a user whose email is an existing member's address
- **THEN** the answer is 201 with that member's user id and their role is unchanged

### Requirement: Users are replaced, patched and deleted

`PUT` SHALL replace a user's SCIM attributes, `PATCH` SHALL apply `add`, `replace` and `remove` operations with or without a path, and `DELETE` SHALL remove the person's membership and answer 204, after which the user reads as 404.

#### Scenario: Patch the display name

- **WHEN** a provider patches `replace displayName "Ann B"`
- **THEN** the person's display name in the tenant is `Ann B`

#### Scenario: Delete a user

- **WHEN** a provider deletes a user
- **THEN** the answer is 204, the membership is gone, the account and its history remain, and `GET` answers 404

### Requirement: Deactivation suspends without revoking

Setting `active` false SHALL disable the person in this tenant and delete their sessions there; their API tokens and SSH keys SHALL stop working while inactive and SHALL work again when `active` is set true. Tasks and history SHALL be kept.

#### Scenario: Deactivate and reactivate

- **WHEN** a provider patches `active` false and later true
- **THEN** while inactive the person's token is refused and sign-in fails, and after reactivation the same token works again

### Requirement: Groups are stored with their members

SCIM Groups SHALL be stored per tenant with a unique display name ignoring case. `GET`, `POST`, `PUT`, `PATCH` and `DELETE` SHALL be supported, PATCH SHALL add and remove members, and a member SHALL be a user of the tenant.

#### Scenario: Add and remove a member

- **WHEN** a provider patches a group with `add members [{value: U}]` and then `remove members[value eq "U"]`
- **THEN** U is a member after the first and not after the second

### Requirement: Group mappings decide a provisioned person's role

A group display name SHALL map to at most one tenant role. A person provisioned over SCIM SHALL hold the highest role mapped to their groups; one who leaves their last mapped group SHALL drop to `viewer`. A person never provisioned over SCIM SHALL not be changed by a mapping.

#### Scenario: Join a group mapped to admin

- **WHEN** `Engineering` is mapped to `admin` and a provider adds a provisioned person to it
- **THEN** the person's role in the tenant is `admin`

#### Scenario: Leave the last mapped group

- **WHEN** that person is removed from `Engineering`
- **THEN** their role is `viewer`

### Requirement: SCIM changes are audited with the scim channel

Every SCIM change SHALL write its audit entry and event in its own transaction with via kind `scim` and the token's name, so it shows in history and in `tix audit ls --via scim`.

#### Scenario: Audit a provisioned user

- **WHEN** a provider creates a user with the token named `scim`
- **THEN** an audit entry `scim.user.create` records via kind `scim` and token name `scim`
