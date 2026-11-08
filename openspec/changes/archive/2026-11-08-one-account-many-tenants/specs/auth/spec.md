## ADDED Requirements

### Requirement: Account credentials of a person in several tenants belong to that person

The password, email, required password change and second factor of an account that holds a membership in more than one tenant SHALL be changed only by the person or by a global admin. A tenant admin's attempt SHALL be refused as forbidden with reason `account_in_other_tenants`, naming no other tenant.

#### Scenario: Tenant admin of test resets a password

- **WHEN** an admin of tenant `test` who is not a global admin sets the password of a person who is also a member of HO
- **THEN** it is refused as forbidden, the password is unchanged, and the refusal names no tenant

#### Scenario: Person in one tenant only

- **WHEN** a tenant admin sets the password of a person who is a member of that tenant only
- **THEN** the password is changed, as it is today

#### Scenario: Global admin resets it

- **WHEN** a global admin sets the password of a person who is a member of several tenants
- **THEN** the password is changed and the change is recorded in the tenant the global admin acted in

### Requirement: An invite to an existing account is accepted with that account's own credentials

Accepting an invite addressed to an existing account SHALL require that account's password and, when one is enrolled, its second factor. Holding the invite link SHALL NOT be enough to accept it.

#### Scenario: The issuing admin opens the link

- **WHEN** the admin who issued an invite to an existing account opens the link and submits a wrong password
- **THEN** acceptance is refused and no membership is granted

#### Scenario: The person accepts

- **WHEN** the person opens the link and gives their password and a valid second factor code
- **THEN** the membership is granted and their password and enrolment are unchanged

### Requirement: A switch satisfies the target tenant's second factor policy

A tenant switch SHALL require a second factor code when the target tenant requires one on the surface and the current session was minted without one. Each session SHALL record whether its sign-in passed a second factor.

#### Scenario: Target requires a code the session did not give

- **WHEN** a person whose HO session was minted with a password only switches to a tenant that requires a second factor in the browser
- **THEN** they are asked for a code and no session is minted until a valid one is given

#### Scenario: No second factor set up

- **WHEN** a person whose account has no second factor switches to a tenant that requires one in the browser
- **THEN** the switch is refused as forbidden with reason `second_factor_required` and no session is minted

#### Scenario: Session already passed one

- **WHEN** a person whose session was minted with a second factor switches to such a tenant
- **THEN** the switch completes without asking again

### Requirement: Disabled state is read on every request

The session and token verifiers SHALL refuse a credential whose account or actor is disabled, read on each request, so that disabling an account stops every tenant at once.

#### Scenario: Account disabled while signed in to two tenants

- **WHEN** a global admin disables an account that holds live sessions in two tenants
- **THEN** the next request on either session is refused

## MODIFIED Requirements

### Requirement: Administrative reset

A tenant administrator SHALL be able to clear another user's TOTP enrolment, which removes the secret and every outstanding recovery code and returns the user to an unenrolled state, when that user is a member of the administrator's tenant only. For a user who is also a member of another tenant, only a global admin SHALL be able to clear it.

The reset SHALL be recorded in the audit log naming the administrator who performed it and the user affected.

#### Scenario: Reset returns the user to unenrolled

- **WHEN** an administrator clears the enrolment of a user who is a member of that tenant only
- **THEN** that user's next sign-in asks for no code, and is sent through enrolment if the tenant requires a factor

#### Scenario: Reset of a person in several tenants

- **WHEN** a tenant administrator who is not a global admin clears the enrolment of a user who is also a member of another tenant
- **THEN** it is refused as forbidden and the enrolment is unchanged

#### Scenario: The reset is attributed

- **WHEN** an administrator clears a user's enrolment
- **THEN** an audit entry records the acting administrator, the affected user, and the time
