# Tasks

## 1. Contract and stores

- [x] 1.1 Migration 0026 `tenants.trash_off`, both stores read and write it
- [x] 1.2 `core.Tenant.TrashOff`, `UpdateTenantInput.TrashOff`, `core.TrashStatus`, `core.TrashIsOff`
- [x] 1.3 `store.Tx.CountTrashed` on both engines

## 2. Service

- [x] 2.1 `DeleteTask` is permanent with the trash off; `RestoreTask` and `Trashed: Yes` refused
- [x] 2.2 `UpdateTenant` applies the setting, one audit entry and event
- [x] 2.3 `TrashStatus` and its registry entry

## 3. Surfaces

- [x] 3.1 HTTP `GET /api/v1/trash`, client
- [x] 3.2 CLI `tix tenant edit --trash`, `tix task trash`, `task rm` note
- [x] 3.3 Web Tenant screen choice with tip and held count, hidden sidebar entry, filter-bar refusal
- [x] 3.4 Web confirmation for single and bulk delete, no undo
- [x] 3.5 TUI exemption recorded: it shows the service's refusal
- [x] 3.6 Bulgarian catalog entries

## 4. Docs

- [x] 4.1 `docs/tenancy.md`, `docs/web-ui.md`, `docs/api.md`, `docs/scripting.md`, `docs/tui.md`
- [x] 4.2 `skills/tix`
