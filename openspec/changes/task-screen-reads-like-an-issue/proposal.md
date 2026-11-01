# The task screen reads like an issue

## Why

The task screen grew one card at a time and it showed. The description was drawn twice, once as
text and once in an always-open Edit card under it, so a task with a real description was two
screens tall before its subtasks. The title, priority, assignee and repeat rule were each shown in
one place and edited in another. Copy to, Save as template and Snooze were ghost links at the far
end of the title row, Delete sat in a Danger zone at the foot of the page, and the identifier's
Copy lived in an About card. The aside was three separate cards with labels that wrapped their
values onto four lines, the history was the only full-width panel, and on a phone every property
came after the last comment.

## What Changes

- A header: the reference, the title in large type with Rename beside it, the star, and one
  actions menu (three dots) holding Copy link, Copy the task id, Copy to, Save as template,
  Snooze or Unsnooze, Reclaim the lease when offered, and Delete with its two options.
- Two columns on a wide screen. The main column reads description, subtasks, dependencies,
  artifacts, comments and history. The right column is one properties panel: status, priority,
  snooze and claim state, assignee, due date, project, tags, repeat rule and each custom field,
  every one a label and a value a writer edits where it is read, then the record's facts
  (created by, created, updated, started, finished, version, id).
- The description is read as text with its checklist, and an Edit switch over it turns it into
  the editor in place. There is no separate Edit card and no separate custom fields form.
- On a phone the properties come first, as a summary under the title, and the record's facts last.
- The due date can be set and cleared from the browser: the task form posts `due_at`, and a form
  that does not name it leaves the date alone.
- A shortcut whose target sits inside a closed disclosure opens it, so `e` still reaches the title.
- A stylesheet block left open by an earlier merge is closed, which brings back every rule after
  it (the inbox, templates and this screen) on screens wider than a phone.

## Impact

- `internal/web/templates/task.html`, `internal/web/assets/app.css`, `assets/shortcuts.js`,
  `assets/busy.js`, `internal/web/taskdetail.go`, Bulgarian strings, `docs/web-ui.md`.
- No route, capability, service method or CLI flag changes. Every route the screen posted to
  before it still posts to.
