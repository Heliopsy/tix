# The people screen in the terminal

## Why

The registry records ten operations a tenant administrator has in the browser's Admin > Users screen and on
the command line, and not in the terminal: listing, reading, creating, editing and deleting users, issuing,
listing, re-issuing and revoking invites, and clearing somebody's second factor. Each is a `GAP:` exemption on
the terminal surface. An administrator working over SSH has to leave the session to let a colleague in.

## What Changes

- A new `people` view, opened by `@` from anywhere, lists the tenant's users with their role, state and
  second factor, and the pending invites under them. One cursor runs over both lists.
- `enter` reads the selected user in full. `n` creates a user with a password (optionally requiring a change
  at first sign-in) or issues an invite link, on one form. `e` edits a user's display name, role, disabled
  state, forced password change and password. `X` deletes the selected user or revokes the selected invite,
  behind a confirmation naming them. `R` re-issues the selected invite. `F` clears the selected user's second
  factor, behind a confirmation.
- An issued invite link is shown once, on the screen, until the reader leaves it or acts again.
- A refusal carrying `account_in_other_tenants` is reported in its own words, naming the person, so the
  administrator learns why their change stopped rather than reading a bare permission error.
- The view is offered only to a reader who may list users (`user:admin`, held by the admin role), and each
  key only to a reader whose authority reaches the operation it calls.
- The ten `GAP:` exemptions become `TUI: "people"` bindings, so the parity test holds the view to them.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `tui`: a people administration view.

## Impact

- `internal/tui`: a new view and its forms, confirmations and messages; a masked text field for passwords.
- `internal/capability`: ten bindings, the view in the parity test's view list, the gap count down by ten.
- `docs/tui.md`, `docs/README.md`: the view, its keys and the new gap count.
