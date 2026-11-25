# Server administration and global admin in the terminal

## Why

Two kinds of administration still have no terminal screen. A tenant admin can read the server's status
and list or end the live connections it holds on the web and the command line, and the registry records
all three as terminal gaps. A global admin can create and delete tenants, manage any tenant's users, add
an existing account to a tenant, grant and revoke global admin and read any account's log from the web
Global admin screens, and the terminal records nine of those as exemptions and two as gaps on the grounds
that a terminal session is pinned to one tenant.

That reason does not hold. Every global admin operation takes the tenant it acts on as an argument, and
the web Global admin screens are served to a session that is pinned to one tenant exactly as a terminal
session is. Nothing about a terminal stops it from doing the same.

## What Changes

- A server view (`%`) for readers who may read the status report: the installation, the answering server,
  every registered server with its heartbeat, the tenant's work counts and second factor state, and the
  tenant's live connections on the answering server with a cursor and a confirmed end.
- A global admin view (`!`), offered only to a session whose actor is a global admin right now: every
  tenant, with create (key, name, first admins) and delete; the users of a tenant, with create, add an
  existing account, edit (name, role, disabled) and delete; the global admins, with grant and revoke; and
  the account log of any account by email.
- The terminal resolves whether its actor is a global admin by asking `WhoAmI`, as the web sidebar does,
  and `capability.Operation.Permits` honours that report for operations needing `global:admin`.
- Generated first-admin passwords are shown once on the global admin view and are gone when it is read
  again or left.
- The form's shared `Secret` flag on a new user's password, masked as it is typed and in the form's
  summary.
- Registry: `status.read`, `connection.list` and `connection.end` bound to the `server` view;
  `tenant.create`, `tenant.delete`, `tenant.user.list`, `tenant.user.create`, `tenant.user.update`,
  `tenant.user.delete`, `tenant.member.add`, `global_admin.list`, `global_admin.grant`,
  `global_admin.revoke` and `account.log.any` bound to the `globaladmin` view. `tenant.delete` records a
  terminal limitation: the session's own tenant is not deleted from here.
- The terminal gap count falls by five and the non-gap absences by nine.

## Impact

- `internal/tui`: two new views in their own files, the shared `Secret` flag on a password field, hooks in the shared dispatch.
- `internal/capability`: registry bindings, `Permits`, gap count, recorded shortfalls, view list.
- `cmd/tui.go`, `internal/sshd/session.go`: resolve the actor's global admin standing before computing
  access.
- `docs/tui.md`: the two views, their keys and the new figures.
- No HTTP, CLI, web or store change.
