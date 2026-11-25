# The web palette offers what the app's does

## Why

The app's command palette (tix-mobile, `quick_actions_host.dart` and the task detail commands) creates a
task from any template, switches tenant, picks a colour scheme by name and acts on the open task: move,
assign, star, snooze, copy the reference, hand to an agent and the rest. The web palette could only go
somewhere, cycle the colour scheme one step and press a few listing controls, so a reader moving between
the two lost half of what ctrl+k did.

## What Changes

- **New task from a template**: one entry per template the reader can read, found by its name and by the
  title it gives. Choosing one posts the form the Use link on Templates opens. A tenant-wide template's
  task goes in the project on screen (a project's screens, or the task on screen's project); with none,
  the palette opens that form so the reader picks one. Read from a new `GET /palette/templates`.
- **Switch to**: one entry per tenant the header's tenant switcher has a Switch button for, sending that
  button's form.
- **Colour scheme**: one entry per scheme, System, Light, Dark, OLED, Low contrast and System, OLED when
  dark, sending the sidebar switch with that scheme. Switch the colour scheme stays.
- **Task**: on a task screen, on its own page or in the task dialog, the things the screen lets the
  reader do to it: change the status or priority, assign, change the due date, add a tag or a comment,
  star or unstar, snooze or unsnooze, copy the reference, hand to an agent, rename, write the
  description, copy to, move to, save as template and reclaim the lease. Each drives the control the
  screen draws, and is offered only when that control is drawn, so a reader who may not change the task
  is offered nothing that would.
- Templates, tenants and colour schemes are listed once something is typed; the task's actions are listed
  with nothing typed too, under Task.

## Capabilities

### Modified Capabilities

- `web-ui`: the command palette gains Task, Templates, Switch to and Colour scheme groups.

## Impact

- `internal/web/palette.go`: `/palette/templates`, the embedded colour schemes and task actions.
- `internal/web/assets/palette.js`: the four groups and what choosing each does.
- `internal/i18n/bg_palette.go`, `internal/web/i18n.go`: Bulgarian and the script's new words.
- `docs/web-ui.md`: the palette section.
- No new capability: the template route reads `ListTemplates`, the same call `GET /api/v1/templates`
  makes, and every action posts a form the screens already post.
