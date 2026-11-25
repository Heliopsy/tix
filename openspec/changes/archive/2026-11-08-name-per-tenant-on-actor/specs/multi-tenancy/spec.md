## ADDED Requirements

### Requirement: A person's display name is per tenant

The display name shown for a person as a user of a tenant SHALL be the one on their actor in that
tenant, on every surface. A rename SHALL write that actor, and SHALL also write the account when the
person belongs to that tenant only. A rename in one tenant SHALL NOT change the name shown in another.

#### Scenario: Renaming a two-tenant person

- **GIVEN** a tenant admin who is not a global admin, and a person in that tenant and another
- **WHEN** the admin renames the person on the Users screen
- **THEN** the Users screen, GetUser and ListUsers in that tenant show the new name, and the other tenant still shows the old one

#### Scenario: Renaming a one-tenant person

- **WHEN** a tenant admin renames a person who belongs to that tenant only
- **THEN** the actor and the account both carry the new name, and a membership added later starts with it
