# A tenant can turn its trash off

## Why

Deleting a task soft deletes it, and the trash (#128) is where it waits to be restored. Some tenants do
not want deleted work kept at all: a delete there should mean gone. Today the only way to get that is
`--hard` on every delete, which the browser's undo toast and bulk bar never pass.

## What Changes

- Tenants gain a `trash_off` setting, false by default so every existing tenant keeps the trash. A tenant
  admin sets it with `tix tenant edit --trash on|off`, the Trash choice on the web Tenant screen, or
  `PATCH /api/v1/tenants/{ref}` with `trash_off`.
- With the trash off the service deletes tasks permanently whatever `hard` says, refuses `RestoreTask` and
  listings with `Trashed` set to yes, both with `precondition_failed` and the detail `reason: trash_off`.
- A new `TrashStatus` operation answers any reader with whether the trash is on and how many tasks it
  holds: `tix task trash`, `GET /api/v1/trash`, and every web screen for the sidebar.
- Turning the trash off keeps the tasks already in it, out of reach, and turning it back on returns them.
- Web: the Trash sidebar entry is hidden, the trash address shows the refusal in its filter bar, and every
  delete (single and bulk) answers with a confirmation instead of deleting and offering Undo.
- CLI: `tix task rm` keeps its flags and its no-prompt behaviour and prints a note on stderr that the
  delete is permanent.
- Docs, the agent skill and the Bulgarian catalog follow.

## Capabilities

### Modified Capabilities

- `multi-tenancy`: the trash setting.
- `task-management`: delete, restore and the trash listing obey it; `TrashStatus`.
- `cli`: `tix tenant edit --trash`, `tix task trash`, the `task rm` note.
- `http-api`: `GET /api/v1/trash`, the refusal.
- `web-tasks`: the hidden Trash, the confirmation, the Tenant screen choice.

## Impact

Migration `0026_tenant_trash.sql` adds `tenants.trash_off`. `core.Service` gains `TrashStatus`, `store.Tx`
gains `CountTrashed`. No change for a tenant that never touches the setting.
