# Tasks

## 1. Standing

- [x] 1.1 `capability.Operation.Permits` answers `Actor.GlobalAdmin` for `global:admin` operations
- [x] 1.2 `tui.WithStanding` sets the report from `WhoAmI`; `cmd/tui.go` and `internal/sshd` use it

## 2. Server view

- [x] 2.1 `internal/tui/serveradmin.go`: read status and connections, render, cursor
- [x] 2.2 End a connection behind a named confirmation
- [x] 2.3 Model tests in `serveradmin_test.go`

## 3. Global admin view

- [x] 3.1 `internal/tui/globaladmin.go`: tenants and admins, a tenant's users, an account log
- [x] 3.2 Create and delete a tenant, own tenant refused, generated passwords shown once
- [x] 3.3 Create, add, edit and delete a tenant's users
- [x] 3.4 Grant and revoke global admin, account log by email
- [x] 3.5 `FieldSecret` form field kind, masked in the input and the summary
- [x] 3.6 Model tests in `globaladmin_test.go`

## 4. Registry and docs

- [x] 4.1 Bind the fourteen operations, limitation on `tenant.delete`, gap count, view list
- [x] 4.2 `docs/tui.md`: views table, keys, offered-when table, gap figures
