# Tasks

## 1. Spec

- [x] 1.1 Proposal, design, delta spec and tasks for the people view

## 2. Terminal view

- [x] 2.1 `people` view opened by `@`, gated on the registry reads, listing users and pending invites
- [x] 2.2 Read the selected user in full on `enter`
- [x] 2.3 Create form with password and invite modes, a masked password and the forced change
- [x] 2.4 Edit form sending only the changed fields
- [x] 2.5 Delete a user, revoke and re-issue an invite, clear a second factor, each confirmation naming its subject
- [x] 2.6 Show an issued invite link once
- [x] 2.7 Explain an `account_in_other_tenants` refusal

## 3. Registry and docs

- [x] 3.1 Replace the ten `GAP:` exemptions with `TUI: "people"` bindings and lower the gap count
- [x] 3.2 docs/tui.md: the view, its keys, the gate and the gap count; docs/README.md view count

## 4. Tests

- [x] 4.1 Model tests for listing, gating, each action, the one-time link and the cross-tenant refusal
- [x] 4.2 Watch the gating and the changed-fields guards fail
