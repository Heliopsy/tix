# Housekeeping cleans up finished tasks

## Why

Done and cancelled tasks pile up on boards and in listings forever, and the trash (#128, #142) never
empties: retention only ever touched events, audit entries and webhook deliveries. People asked for
cleanup "as a setting per project, per tenant". Doing it by hand means a bulk delete every few weeks.

## What Changes

- A housekeeping policy with three rules: move tasks finished in a done-category state to the trash after N
  days, the same for cancelled-category states with its own N, and empty the trash of tasks deleted more than
  N days ago.
- Each rule is set per tenant as the default and overridable per project as inherit, off, or a number of days.
  Every rule is off by default, so nothing changes for existing data.
- The retention pruner `tix serve` already runs applies the policy on its tick, in bounded batches, as the
  system actor, writing the usual `task.delete` audit entry and `task.deleted` event per task.
- Finished tasks only ever go to the trash, never straight to permanent deletion. With the tenant's trash off,
  enabling the two finished-task rules is refused and every rule pauses.
- `tix cleanup show|set|preview|run`, `GET|PUT /api/v1/housekeeping`, `POST /api/v1/housekeeping/run`, a
  Housekeeping section on the Tenant screen and a Housekeeping panel in each project's settings, each showing
  how many tasks the policy would touch now.

## Capabilities

### Modified Capabilities

- `retention`: the housekeeping policy, its resolution and the pruner tick.
- `cli`: `tix cleanup`.
- `http-api`: the housekeeping routes.
- `web-ui`: the Tenant screen section and the project panel.

## Impact

Migration `0027_housekeeping.sql` adds `housekeeping_policies`. `core.Service` gains `GetHousekeeping`,
`PutHousekeeping` and `RunHousekeeping`; `store.Tx` gains the policy reads and writes and the conditional
trash and purge statements. No behaviour change for a tenant that sets nothing.
