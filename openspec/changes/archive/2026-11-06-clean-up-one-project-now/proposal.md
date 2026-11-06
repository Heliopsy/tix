# Clean up one project now from its board

## Why

A board's Housekeeping panel shows a project's cleanup overrides and how many of its tasks each rule
would act on now, but running that pass meant the Tenant screen's Clean up now, which runs every
project, or `tix cleanup run --project KEY` on the command line. An admin looking at one project's
pending count had no way to act on that project alone from the browser.

## What Changes

- The board's Housekeeping panel gets Clean up now, posting to `/projects/{key}/housekeeping/run`, which
  runs `RunHousekeeping` with that project and nothing else.
- It opens a confirmation in place first, naming how many tasks go to the trash and how many are
  deleted for good. The run refuses a post that did not come through it.
- It is greyed out, with the reason, while nothing is due or the trash is off.
- The toast after a run says how many tasks were moved to the trash and how many deleted for good.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: a project's Housekeeping panel runs cleanup for that project.

## Impact

`internal/web` (route, handler, board template, CSS), the Bulgarian catalogue, `docs/web-ui.md` and
`docs/configuration.md`. No new service method: the route is a second web binding of
`RunHousekeeping`, which already takes a project. No migration.
