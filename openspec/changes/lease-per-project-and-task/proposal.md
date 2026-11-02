# Lease length per project and per task

## Why

A lease length could be set per claim, per authored workflow and per tenant. Work inside one tenant is not
uniform: a project of long builds needs hours, a triage project minutes, and one slow task in an otherwise quick
project needs its own figure. Today that means a workflow per project or a `--ttl` on every claim.

## What Changes

- A task carries a `lease`; a project carries `lease_default` and `lease_max`. Migration `0028_lease_scope.sql`.
- Order, most specific first: claim `--ttl`, task, project, authored workflow, tenant, deployment, 30m.
- The ceiling is the lowest of the deployment, tenant and project maximums; a project can only lower it.
- `TaskLease` reports the lease a claim would get and which level decided it; claims and renewals carry the same.
- Surfaces: `tix task edit --lease`, `tix project edit --lease-default/--lease-max`, `tix task show` and claim
  output, `GET /api/v1/tasks/REF/lease`, the task screen's Lease row and the board's Claims panel, the TUI detail.
- Copy, templates, the next occurrence of a repeating task, export and import carry the task's lease.

## Impact

- `internal/lease`, `internal/service` (claim, task, project), both stores, core types, every surface.
- No change for existing data: every new column is empty, which inherits.
