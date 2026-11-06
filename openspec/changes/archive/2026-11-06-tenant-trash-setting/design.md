# Design

## The setting is a column, named for its default

`tenants.trash_off INTEGER NOT NULL DEFAULT 0`, following `require_totp_*`. Named `trash_off` rather than
`trash` so that the zero value, in the column, in `core.Tenant` and in a JSON body that omits it, is
today's behaviour. A `trash bool` whose zero value meant "off" would have turned every tenant's deletes
permanent on upgrade and on every `CreateTenant` that did not name it.

## Enforced in the service

`DeleteTask` reads the tenant inside its own transaction and sets `Hard` when the trash is off, so every
caller, bulk deletes and the TUI included, deletes permanently without knowing about the setting.
`RestoreTask` and `ListTasks` with `Trashed: Yes` refuse with `core.TrashIsOff`, a `precondition_failed`
carrying `reason: trash_off` so a client can tell it from other preconditions.

`IncludeDeleted` (`is:deleted`) is not refused. It is the export-shaped "everything" listing, and a tenant
that turned the trash off still holds the rows deleted before; hiding them there would make an export
silently incomplete.

## Turning it off keeps the trash

The safer of the two flows offered. Turning the trash off changes what future deletes do and puts the
existing trash out of reach (no listing, no restore), and nothing is destroyed. Turning it back on
returns everything. The Tenant screen and `tix task trash` count what is held so the admin can see it.
Asking "empty it now?" on the same form would put an irreversible action behind the control that looks
like a preference.

## No "empty after N days"

The retention pruner removes events, audit entries and webhook deliveries by age and never touches tasks.
Purging the trash would need a new class with a subtree-ordered hard delete and an audit entry and event
per task, plus the setting on every surface. Not cheap, so left out.

## TrashStatus is its own operation

Every surface that draws a Delete needs to know whether it can be undone, and `GetTenant` is an admin
operation. A member-readable read of one flag and one `COUNT(*)` is cheaper than widening `GetTenant`.

## Web: the confirmation is drawn by the handlers

`askBeforePermanentDelete` runs in the single and bulk delete handlers. An unconfirmed delete on a tenant
with no trash renders `taskdelete.html`, which re-posts every submitted field plus `confirm=1` to the
same route. Because the question lives in the handler, any Delete drawn anywhere, including on the task
page being redesigned in parallel, asks. A request from an htmx control that swaps one card is retargeted
to `main` so the confirmation draws as a screen. "Delete permanently" on the task screen is already an
explicit permanent delete and does not ask again.

The trash address with the trash off answers the task list with the refusal in its filter bar, rather
than a 404, because it is a filter a reader may have bookmarked.

## CLI keeps its semantics

`tix task rm` has never prompted; `--hard` is the explicit permanent delete and nothing asks. That stays.
With the trash off it prints one note on stderr and its dry-run plan reports `hard: true`.

## Audit

The setting goes through `UpdateTenant`, which already records `tenant.update` and `tenant.updated` with
the before and after record in the same transaction; its metadata gains `trash: on|off` when it changed.
