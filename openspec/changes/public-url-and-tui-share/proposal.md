# Public web address and sharing from the terminal

## Why

Sharing a task (`task-share`) left two gaps. The server never knew its own public web address: it
derived one from each browser request, so nothing outside a request could build a link, `tix task
link` did not run over SSH, and an agent reading a task over the API or MCP had no link to hand a
person. And the terminal interface had no way to copy anything, so sharing a task from it meant
retyping the reference.

## What Changes

- New `server.public_url` key (`TIX_SERVER_PUBLIC_URL`): the absolute http or https address people
  reach the browser interface at, with an optional base path. Validated at load. Unset, nothing
  changes.
- With it set, every task the service returns carries `web_url`, its screen in the browser
  interface, so the API, MCP, the CLI's structured output and the terminal interface all have it.
- `tix task link` runs over SSH. There it uses the task's `web_url`; `--base` stays refused like
  the other inputs that name something on the serving machine, and without `server.public_url` the
  command exits 2 saying so.
- The links the server draws from a request (the Share menu, the hand-off dialog, calendar feed
  addresses, invite links, the Connect screen) use `server.public_url` on a host that serves several
  tenants, and the request's own host otherwise. Notification channel messages link under it too.
- A tenant on a domain of its own keeps that domain in every link: the public address's scheme,
  port and path with the tenant's host.
- The terminal interface's share key (`^`) on a selected task or the task screen offers the link,
  the Markdown link and the key and title, copies the choice to the terminal's clipboard with OSC 52
  locally and over SSH, and always shows the copied text too.

## Capabilities

### Modified Capabilities

- `configuration`: `server.public_url`.
- `http-api`: `web_url` on task responses.
- `ssh-access`: `tix task link` is hosted.
- `cli`: `tix task link` prefers the task's `web_url`.
- `web-ui`: links drawn from a request prefer the public address on a shared host.
- `multi-tenancy`: a tenant on its own domain links under it.
- `tui`: sharing a task through the terminal clipboard.

## Impact

- `internal/config` (key, validation), `internal/service` (`WithPublicURL`, `web_url`),
  `internal/core` (`Task.WebURL`), `internal/tasklink` (`Rehost`, `Label`), `internal/httpapi`
  (`Origin`), `internal/web`, `internal/webhook`, `internal/server`, `internal/serving`,
  `internal/connect`.
- `internal/capability` (`Operation.Formats`, hosting `tix task link` with `task.show`), `cmd/tasklink.go`.
- `internal/tui` (share key, picker, reveal), `cmd/tui.go`.
- `api/openapi.yaml`, docs, the tix skill.
