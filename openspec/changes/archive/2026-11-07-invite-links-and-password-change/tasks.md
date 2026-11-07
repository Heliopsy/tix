# Tasks

## 1. Model and storage

- [x] 1.1 Migration 0031: `users.require_password_change`, `user_invites`
- [x] 1.2 Store methods on SQLite and PostgreSQL
- [x] 1.3 `internal/core`: the flag, `new_password`, the `password_change` challenge, invite types

## 2. Service

- [x] 2.1 `Login` answers the challenge and takes the new password; audit `user.password_change`
- [x] 2.2 `CreateUser`/`UpdateUser` set the flag
- [x] 2.3 `CreateInvite`, `ListInvites`, `RevokeInvite`, `ReissueInvite`, `PreviewInvite`, `AcceptInvite`

## 3. Surfaces

- [x] 3.1 HTTP routes, client, registry entries, `just openapi`
- [x] 3.2 CLI: `--require-password-change`, `tix user invite`, `tix login` asks for the new password
- [x] 3.3 Web: create dialog modes, link dialog, Pending invites, Manage checkbox, invite page,
      password change page, Bulgarian strings

## 4. Documentation

- [x] 4.1 `docs/`, README, `skills/tix`
