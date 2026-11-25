## ADDED Requirements

### Requirement: Users screens leave out removed people

The Users screen and the global admin's users screen of a tenant SHALL NOT draw a row or a manage dialog for a
person removed from that tenant. The activity screen SHALL still name such a person in history.

#### Scenario: Removed while still a member elsewhere

- **GIVEN** a person who is a member of the reader's tenant and of another
- **WHEN** the reader removes them and the Users screen draws again
- **THEN** the screen has no row for them, and the other tenant's users screen still has one

## MODIFIED Requirements

### Requirement: The user listing shows and preserves each account's role

The user administration screen SHALL show, for every account it lists, the role that account's
membership of this tenant grants, and the edit control SHALL open with that role already selected.
Saving any other change to an account SHALL NOT alter its role unless the role was itself changed.
An account whose role the screen cannot read, because the reader may not list memberships, SHALL be
shown as having none, and its control SHALL offer to leave the role unchanged rather than proposing one.

#### Scenario: The role is on the row

- **WHEN** the user listing renders an account whose membership grants it a role
- **THEN** that role is shown on the row

#### Scenario: Editing another field does not change the role

- **WHEN** an administrator opens an account, changes only whether it is disabled, and saves
- **THEN** the account keeps the role it had

#### Scenario: An account with no membership proposes no role

- **WHEN** a reader holding user:admin but not tenant:admin opens the listing
- **THEN** each account's control offers to leave the role unchanged and selects no role
