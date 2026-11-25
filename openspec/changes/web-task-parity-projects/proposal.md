# Open task counts on the projects screen

## Why

The projects screen names each project and says nothing about the work in it. The app shows how many
open tasks each project holds beside its name, so a reader picks the project that needs attention without
opening each one. The web had no such count.

## What Changes

- Each row on `/projects` shows how many open tasks the project holds, or "no open tasks".
- "Open" is what the task list opens on: a state its workflow does not finish, snoozed tasks aside.
- The counts come from one tallied task listing for the whole screen, not a read per project.

## Impact

- `internal/web/projects.go`, `templates/projects.html`, a stylesheet part, Bulgarian strings,
  `docs/web-ui.md`.
