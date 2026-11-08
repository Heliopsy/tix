# Share a task

## Why

The owner asked for a button that shares a task easily. Copying a link to a task meant finding
Copy link inside the actions menu, and anything richer, a Markdown link for a pull request or the
key and title for a chat message, meant typing it out. A phone's own share sheet was not offered at
all, and nothing outside the browser printed a task's web address.

## What Changes

- The task screen's header, as a page and in the task dialog, carries a Share button that opens a
  small menu: Share… (the system share sheet, where the browser has one), Copy link, Copy as
  Markdown and Copy key and title. Each copy reports itself in a toast that is announced.
- Copy link moves from the actions menu into the Share menu, so each action is still drawn once.
- A lane card's more actions menu gains Copy as Markdown, Copy key and title and Share….
- The menu shows the task's absolute address as a plain link, so with scripting off there is still
  something to select and copy.
- The link is the task's screen on the host the reader is on. It names no tenant: the tenant is
  taken from the reader's session, and the interface refuses a tenant named in the request.
- `tix task link REF` prints the task's web address, or the Markdown or the key and title, from the
  command line. The address is the server the context talks to, or `--base`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-tasks`: the Share menu on the task screen and on a lane card; Copy link moves out of the
  actions menu.
- `cli`: `tix task link`.

## Impact

- New `internal/tasklink`: the address, the Markdown and the plain forms, with no Cobra.
- `internal/web/share.go`, `internal/web/templates/share.html` (parsed into every screen),
  `templates/task.html`, `templates/tasks.html` (one include in the lane card menu),
  `templates/layout.html` (the script tag), `assets/share.js`, `assets/copy.js`,
  `assets/css/697-task-share.css`, `icons.svg`.
- `cmd/task.go`, `internal/i18n/bg_share.go`, `docs/web-ui.md`, `docs/tenancy.md`, `skills/tix`.
- No service, store, registry or API change.
