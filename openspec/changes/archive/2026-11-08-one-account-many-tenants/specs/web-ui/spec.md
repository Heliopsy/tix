## ADDED Requirements

### Requirement: The header switches between the reader's tenants

For a reader who is a member of more than one tenant, the header SHALL show the current tenant as a button opening a dialog that lists each of the reader's tenants with its role and a Switch button. It SHALL use no disclosure element, and SHALL work without scripting through a dialog link.

#### Scenario: Two memberships

- **WHEN** a person who is a member of HO and `test` is signed in to HO
- **THEN** the header's tenant button opens a dialog listing HO as current and `test` with a Switch button

#### Scenario: One membership

- **WHEN** a person who is a member of one tenant only is signed in
- **THEN** the header shows the tenant's name as text and offers no switcher

#### Scenario: Without scripting

- **WHEN** scripting is off and the tenant button is followed
- **THEN** the same list is drawn in place and each Switch button submits a plain form

### Requirement: A tenant at its own address is a link

The switcher SHALL offer a Switch button only for a tenant the current host serves. A tenant served at a domain of its own SHALL be listed as a link to that address's sign-in page.

#### Scenario: Tenant with its own domain

- **WHEN** a person in HO and in `away`, which has its own domain, opens the switcher on HO's shared host
- **THEN** `away` is listed with a link to its sign-in page and no Switch button

### Requirement: Sign-in on a shared host goes by the account

A browser sign-in on a shared host that no header or credential pointed at a tenant SHALL accept an account that is no member of the host's own tenant, signing it in to the first tenant by key it belongs to that the host serves. After the password and any second factor, a person with more than one such tenant SHALL land on a Choose a tenant screen.

#### Scenario: Member of test only

- **WHEN** a member of `test` alone signs in on tix.z09.org, a shared domain of HO
- **THEN** the session belongs to `test`

#### Scenario: Member of both

- **WHEN** a member of HO and `test` signs in on tix.z09.org
- **THEN** after the password and any code the Choose a tenant screen lists both

### Requirement: The switcher lists only the reader's own memberships

The switcher SHALL list only tenants the reader is a member of. A global admin's reach into other tenants SHALL stay in the Global admin area and SHALL NOT appear as switcher entries.

#### Scenario: Global admin without membership

- **WHEN** a global admin who is a member of HO only opens the switcher
- **THEN** it lists HO alone and links to the Global admin area

### Requirement: Users screen shows a pending invite of an existing account

A pending invite addressed to an existing account SHALL appear on the inviting tenant's Users screen with its email and state.

#### Scenario: Pending

- **WHEN** a tenant admin has invited an address that has an account and it is not accepted yet
- **THEN** the Users screen lists the address as pending

### Requirement: The Global admin area shares a tenant's domains

A tenant's screen in the Global admin area SHALL list its domains, each marked shared or bound to the tenant, with a control that sets or clears the flag. A tenant's own Domains screen SHALL show the flag and SHALL NOT offer to change it.

#### Scenario: Global admin shares a host

- **WHEN** a global admin opens tenant HO in the Global admin area and shares tix.z09.org
- **THEN** the row shows the domain as shared and offers to stop sharing it

### Requirement: An Account screen shows the reader's account log

The web UI SHALL offer an Account screen, reached from the settings screen, listing the reader's account log. The Global admin area SHALL link each user to their account log.

#### Scenario: Reader opens Account

- **WHEN** a signed-in person opens the Account screen
- **THEN** it lists the changes to their account with who made each and the tenant it came from
