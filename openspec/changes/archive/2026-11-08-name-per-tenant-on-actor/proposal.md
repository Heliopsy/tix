# A person's name in a tenant lives on their actor

## Why

A tenant admin renaming somebody who also belongs to another tenant saved the name on the person's
actor in this tenant, but GetUser and ListUsers showed the account's name, so the Users screen kept the
old name and the rename looked lost. The opposite held for a person in one tenant: a rename wrote only
the account, while the account menu, assignees and `whoami` read the actor and kept the old name.

## What Changes

- The design already makes the actor display name a per-tenant thing a tenant admin may change. Every
  read of a user of a tenant (GetUser, ListUsers, the Users screen, `tix user ls/show`, the API, the TUI)
  shows the actor's name.
- A rename always writes the actor, and the account too when the person belongs to this tenant only, so
  a membership added later starts from it.
- Accepting an invite with a chosen name sets the actor's name as well as the account's.
- A migration copies the account's name onto the actor of each person who has actors in one tenant
  only. People in several tenants keep each tenant's name.

## Capabilities

### Modified Capabilities

- `multi-tenancy`: the display name shown for a person in a tenant is the one on their actor there.

## Impact

`internal/service/user.go`, `internal/service/invite.go`, a migration under `internal/store/migrations`,
`docs/tenancy.md`.
