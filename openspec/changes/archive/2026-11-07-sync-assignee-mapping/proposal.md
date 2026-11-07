# Imported assignees land on tix people

## Why

Someone copied their OpenProject tasks into tix and none of the assignments came across. The importer only
looked an assignee up by tix actor handle, and OpenProject names the assignee by display name, so every
assignment fell back to a custom field. A dry run did not look at assignees at all, so nothing warned them
before the real run. There was also no starting mapping for OpenProject: every field, status and link had to
be worked out from the API by hand.

## What Changes

- The mapping gains a `users` section for the `assignee` target: an explicit `map` from the source's value
  to a tix handle or email, then automatic `match` strategies (`email`, `handle`, `name` by default), an
  optional `email_field` to read the assignee's email from, and an `unknown` policy.
- `unknown: unassigned` (the default) leaves the task unassigned and keeps the value in the
  `<prefix>assignee` custom field, now under the mapping's real prefix. `unknown: placeholder` creates one
  actor per unmatched person, with no account, membership or scopes, and reuses it on later runs.
- A dry run resolves assignees the same way, read-only, and never creates a placeholder.
- The sync result gains `unmatched_users`, sorted and deduplicated, and one warning lists them.
- A parent reference given as a link (`/api/v3/work_packages/42`) also matches the record imported as `42`.
- The OpenProject adapter ships a default mapping, `openproject.DefaultMapping()`, covering subject,
  description, the stock statuses and priorities, type, assignee, dates, parent and project.

## Capabilities

### Modified Capabilities

- `external-sync`: assignee resolution, the unmatched-user report, parent links and the OpenProject default
  mapping.
