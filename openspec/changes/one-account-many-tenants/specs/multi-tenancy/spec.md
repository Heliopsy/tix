## ADDED Requirements

### Requirement: A person has one actor per tenant, linked to one account

A user account SHALL be linked to each of its actors through the actor's account reference, with at most one actor per account per tenant. No operation SHALL find an account from an actor, or an actor from an account, by assuming the two share an identifier.

#### Scenario: Second tenant gets a second actor

- **WHEN** a person who is a member of tenant A is added to tenant B
- **THEN** tenant B holds a new actor for that person, linked to the same account, and tenant A's actor is unchanged

#### Scenario: Identifiers that differ still resolve

- **WHEN** a person whose actor identifier differs from their account identifier signs in, passes a second factor, signs in over SSH and mints a token
- **THEN** each step finds the same account and the actor of the tenant it runs in

#### Scenario: No second actor in one tenant

- **WHEN** an account that already has an actor in a tenant is added to that tenant again
- **THEN** it is refused as a conflict and nothing is written

### Requirement: A global admin adds an existing account to a tenant directly

A global admin SHALL be able to add an existing account to any tenant with a role and a handle, without the person's acceptance. The addition SHALL be recorded in the target tenant naming the global admin.

#### Scenario: Two people of HO become admins of test

- **WHEN** a global admin adds villy@ and icek@ to tenant `test` as admins
- **THEN** each holds an admin membership in `test`, keeps their membership in HO, and signs in to either with the same password and second factor

### Requirement: A tenant admin reaches an existing account only by invitation

A tenant admin SHALL NOT attach an existing account to their tenant without the person's acceptance. Creating a user with an address that already has an account SHALL be refused as a conflict. An invite to such an address SHALL grant the membership only when accepted.

#### Scenario: Create with an existing address

- **WHEN** a tenant admin creates a user with an address that has an account in another tenant
- **THEN** it is refused as a conflict and no actor or membership is written

#### Scenario: Invite stays pending until accepted

- **WHEN** a tenant admin invites an address that has an account and the person has not accepted
- **THEN** the person holds no membership in that tenant and cannot be assigned work there

#### Scenario: Invite to an account with no password

- **WHEN** a tenant admin invites an address whose account belongs to another tenant and has no password yet
- **THEN** it is refused as a conflict and nothing is written

### Requirement: A first admin of a new tenant may be an existing account

An admin email of a tenant being created that already has an account SHALL make that account an admin of the new tenant, with its own password and second factor. No password SHALL be generated or returned for it, and supplying one for it SHALL be refused as invalid. A key already in use SHALL be refused as a conflict, and nothing SHALL be created.

#### Scenario: Naming somebody who already exists

- **WHEN** a global admin creates a tenant naming as admin an address that has an account in another tenant
- **THEN** the account becomes an admin of the new tenant, keeps its other memberships, and no initial credential is returned for it

### Requirement: Removing a person from a tenant leaves their other tenants alone

Removing or disabling a person in a tenant SHALL affect only that tenant's actor, membership and credentials, unless the person holds no membership elsewhere, in which case the account is removed or disabled as before. The answer SHALL NOT say which case applied.

#### Scenario: Removed from one of two tenants

- **WHEN** a tenant admin of B removes a person who is also a member of A
- **THEN** the person's sessions, tokens and keys in B stop working, and sign-in to A still succeeds

#### Scenario: Removed from the only tenant

- **WHEN** a tenant admin removes a person who is a member of that tenant only
- **THEN** the account is deleted, as it is today

### Requirement: A session acts in one tenant at a time

A session SHALL belong to one tenant and one actor. Moving to another tenant SHALL mint a new session for the person's actor there, after checking that the membership exists and that neither the account nor that actor is disabled.

#### Scenario: Switch to a tenant the person belongs to

- **WHEN** a person signed in to HO switches to `test`, where they are a member
- **THEN** a session for their `test` actor is minted and later requests act in `test` with their `test` role

#### Scenario: Switch to a tenant the person does not belong to

- **WHEN** a person switches to a tenant where they hold no membership
- **THEN** it is refused as not found, the same answer as for a tenant that does not exist

### Requirement: Only a global admin marks a domain shared

A tenant domain SHALL carry a shared flag, off by default. Only a global admin SHALL set or clear it, for a domain of any tenant. A tenant admin or a token without `global:admin` SHALL be refused as forbidden, whether or not the hostname is mapped, and nothing SHALL change.

#### Scenario: Global admin shares HO's host

- **WHEN** a global admin marks tix.z09.org, a domain of tenant HO, shared
- **THEN** the domain is shared and still belongs to HO

#### Scenario: Tenant admin tries

- **WHEN** an admin of HO who is not a global admin marks HO's own domain shared, by session or by a token carrying `*`
- **THEN** it is refused as forbidden and the domain stays bound to HO alone

### Requirement: Sharing a domain is audited in its tenant

Setting or clearing a domain's shared flag SHALL record an audit entry and an event in the tenant owning the domain, naming the global admin and the value before and after. Setting the value the domain already holds SHALL record nothing.

#### Scenario: Shared and unshared

- **WHEN** a global admin marks a domain of HO shared and later clears it
- **THEN** HO's audit log holds two domain updates, each with the shared value before and after

## REMOVED Requirements

### Requirement: An existing user is not made an admin of another tenant

**Reason**: An account may now belong to several tenants, so an existing address no longer blocks tenant
creation.

**Migration**: See "A first admin of a new tenant may be an existing account".

## MODIFIED Requirements

### Requirement: Tenant selection is reachable from the terminal interface

Switching the tenant a session works in SHALL be reachable from inside the terminal interface, and SHALL NOT require leaving it and relaunching. The interface SHALL offer the tenants the caller is a member of, as listed by whoami, and SHALL still accept a typed tenant key.

#### Scenario: Switching without leaving the interface

- **WHEN** a tenant the caller is a member of is chosen in the terminal interface's tenant view and a credential for it is available
- **THEN** the session works in that tenant from then on, without the process being restarted

#### Scenario: The caller's tenants are listed

- **WHEN** the terminal interface asks which tenant to switch to
- **THEN** it lists the tenants the caller is a member of, with the role held in each, and no other tenant

#### Scenario: No list is offered

- **WHEN** the caller is a member of one tenant only, or is a local caller holding the database
- **THEN** no membership list is offered beyond the current tenant and a typed key is taken instead

#### Scenario: No credential for the chosen tenant

- **WHEN** a remote caller chooses a tenant for which no context holds a credential
- **THEN** the interface stays in its current tenant and says which login command would provide one

#### Scenario: The switch is not written down

- **WHEN** a tenant is switched to from inside the terminal interface
- **THEN** the choice applies to that session only, and the persisted selection is unchanged
