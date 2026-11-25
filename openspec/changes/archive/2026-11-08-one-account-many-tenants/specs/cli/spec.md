## ADDED Requirements

### Requirement: A context holds one tenant's credential

A context SHALL keep one server, one tenant and one credential for that tenant's actor. `tix login --tenant KEY` SHALL sign in to that tenant, sending it as the tenant header when the server's host is not claimed by a tenant, and SHALL store the credential on the named or current context.

#### Scenario: Two contexts for one server

- **WHEN** a person runs `tix login --server URL --tenant ho --context ho` and `tix login --server URL --tenant test --context test`
- **THEN** each context holds a credential for its own tenant, and commands under each act there

#### Scenario: Tenant flag disagrees with a bound host

- **WHEN** `tix login --tenant test` targets a host claimed by tenant HO
- **THEN** the login is refused and nothing is written to the configuration

### Requirement: A tenant login stores an API token

`tix login --tenant KEY` against a server SHALL exchange the session for an API token of the person's actor in that tenant, carrying `*` and `account:read`, SHALL end the session, and SHALL write the token to the context. When the person may not mint a token there, it SHALL write nothing and say how to add a token instead.

#### Scenario: Member without token authority

- **WHEN** a person whose role in `test` may not mint tokens runs `tix login --tenant test`
- **THEN** the command is refused, the sign-in session is ended and the configuration is unchanged

### Requirement: Tenant listing and selection read the caller's memberships

`tix tenant ls` SHALL list the caller's memberships for a caller who is not a global admin. `tix tenant use KEY` SHALL be checked against those memberships, and SHALL name the login command to run when the current context holds no credential for KEY.

#### Scenario: Selecting a tenant the caller belongs to without a credential

- **WHEN** a caller whose context holds an HO credential runs `tix tenant use test` and is a member of `test`
- **THEN** nothing is written and the message names `tix login --tenant test`

#### Scenario: Listing

- **WHEN** a member of HO and `test` who is not a global admin runs `tix tenant ls`
- **THEN** both tenants are listed with the role held in each, and no other tenant

#### Scenario: Selecting a tenant another context holds a credential for

- **WHEN** a caller runs `tix tenant use test` and another context on the same server holds a credential for `test`
- **THEN** that context becomes the current one

#### Scenario: Selecting a tenant the caller is not a member of

- **WHEN** a caller who is not a global admin runs `tix tenant use` with a tenant they are not a member of
- **THEN** it is refused as not found and nothing is written

### Requirement: whoami prints the caller's memberships

`tix whoami` SHALL list the caller's memberships, when the credential lists them and there is more than one, with the role held in each and the context holding a credential for it.

#### Scenario: Person in two tenants

- **WHEN** a member of HO and `test` whose token carries `account:read` runs `tix whoami`
- **THEN** both tenants are printed with the role in each, and the context holding each one's token

### Requirement: Global admins add existing accounts from the command line

`tix tenant user add TENANT EMAIL --role ROLE` SHALL add an existing account to a tenant for a global admin, and SHALL be refused for anybody else.

#### Scenario: Added

- **WHEN** a global admin runs `tix tenant user add test villy@example.com --role admin`
- **THEN** the account becomes an admin of `test` and the command prints the new actor's handle

### Requirement: A global admin marks a domain shared from the command line

`tix domain edit HOSTNAME --shared` SHALL mark a domain of any tenant shared and `--shared=false` SHALL clear it, for a global admin. `tix tenant domain ls KEY` SHALL list a tenant's domains with the flag. Either SHALL exit 5 for anybody else.

#### Scenario: Shared from the shell

- **WHEN** a global admin runs `tix domain edit tix.z09.org --shared`
- **THEN** the domain is printed with `shared` true

### Requirement: The account log is reachable from the command line

`tix account log` SHALL print the caller's account log, and `tix global-admin account-log EMAIL` SHALL print another account's log for a global admin.

#### Scenario: Own log

- **WHEN** a person runs `tix account log`
- **THEN** their account's changes are printed, newest first
