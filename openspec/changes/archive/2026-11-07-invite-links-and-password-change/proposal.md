# Invite links and a required password change

## Why

An administrator who creates an account types a password for somebody else and then has to pass it
on, and nothing makes that person replace it. There is also no way to bring somebody in without
choosing a password for them at all: the administrator has to invent one, send it, and hope.

## What Changes

- A user carries a **require password change** flag. It can be set when the account is created (the
  browser form ticks it by default when a password is typed) and on any later edit. A password
  sign-in on an account with the flag set is answered with a `password_change` challenge instead
  of a session, on every surface; the sign-in completes only by sending a new password that meets
  the policy and differs from the current one. Only that clears the flag. API tokens and SSH keys
  are not passwords and are unaffected.
- An administrator can **invite by email** instead of setting a password. The invite creates the
  account with no password, or grants a membership to an account of this tenant that holds none,
  and returns a single-use link `/invite/TOKEN` that expires (7 days by default, at most 30). The
  token is random, stored only as its hash, listed while pending, revocable and re-issuable.
- Opening the link shows a page to choose a password and a display name, then signs the person in.
  An account that already has a password confirms it instead and keeps it. An unknown, expired,
  used or revoked link answers one identical message.
- tix sends no email. The link is shown once to the administrator with a Copy button and they pass
  it on.
- Surfaces: `tix user create --require-password-change`, `tix user edit
  --require-password-change`, `tix user invite EMAIL`, `tix user invite ls|revoke|reissue`; the
  `require_password_change` field on the user routes, `new_password` on `POST /api/v1/auth/login`,
  `/api/v1/invites` and the public `/api/v1/auth/invites/{token}`; the Users screen's create
  dialog, its Pending invites section, the Manage dialog's checkbox, and the public invite and
  password change pages.

## Impact

- Migration 0031 adds `users.require_password_change` and the `user_invites` table.
- `core.User`, `core.CreateUserInput`, `core.UpdateUserInput` and `core.LoginInput` gain fields; new
  `core.Invite`, `core.IssuedInvite`, `core.InvitePreview` and their inputs; six service methods and
  their registry entries.
- Nothing changes for an account without the flag or a tenant that never invites anybody.
