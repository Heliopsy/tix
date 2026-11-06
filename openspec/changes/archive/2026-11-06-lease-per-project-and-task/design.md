## Context

Lease length resolved claim > authored workflow > tenant > deployment > 30m, with the tenant's maximum a clamp
(see `lease.Policy.Narrow`). Housekeeping (#154) added a tenant default with project overrides in a policy table.

## Decisions

- **Columns, not a policy table.** The tenant level already lives on `tenants.lease_default/lease_max` (0012), so
  the project level is `projects.lease_default/lease_max` and the task level `tasks.lease`, duration text with
  `''` inheriting, as on tenants. Setting them rides `UpdateProject` / `UpdateTask`, which already write the row,
  the audit entry and the event in one transaction; no new mutation method.
- **Project above workflow.** The project setting is the more specific choice for the work in that project, and
  a workflow is shared between projects.
- **Project maximum.** It fell out of `Policy.Cap`, so it is in. It may only lower the ceiling: a value above the
  tenant's current ceiling is refused, and resolution takes the minimum anyway, so a tenant maximum lowered later
  still wins.
- **Refuse on set, clamp on claim.** A task or project length typed above the ceiling is refused when it is set,
  so the screen never shows a figure claims do not get. One that drifts above a ceiling lowered afterwards is
  clamped at claim time, and the grant says `clamped`, so lowering a maximum never stops claims.
- **Explaining the lease.** `TaskLease` returns a `LeaseGrant` (length, source, maximum, maximum's source,
  clamped). Claims and renewals carry the same object, so a worker sees why it got what it got.
- **Carrying.** The lease describes the work, so copy, template, next occurrence and export/import carry it. A
  copy into a project with a lower ceiling is clamped at claim time rather than refused, as the copy wrote no
  figure anyone typed.
- **Not on create.** `task add` and `project create` take no lease; set it with edit. Keeps the add forms as
  they are.
