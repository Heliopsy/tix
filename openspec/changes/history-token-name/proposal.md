# Name the token behind an import or sync in history

## Why

A change made with an API token already records the token's ID and its name at the time, and the Activity feed,
a task's History and the raw view print it: "via token ci-agent", "via MCP (token scout-runner)". An import or a
sync started with a token records it the same way, but its channel words had no slot for the name, so the row
read `via sync%!(EXTRA string=ci-runner)`. The Sync runs screen named who ran each import and never the token.

## What Changes

- An import or sync made with a token reads "via import (token ci-runner)" and "via sync (token ci-runner)" in
  Activity, a task's History and the raw view, in English and Bulgarian.
- Sync runs' Ran as column names the token beside the actor: "deploy-bot via token ci-runner".
- Nothing new is stored and no API changes: audit entries and events already carry `via.token_id` and
  `via.token_name`, the name as it was when the change was made, so a renamed or deleted token still reads as it
  did. Who sees it is unchanged: the screens that show it already need the audit log.
