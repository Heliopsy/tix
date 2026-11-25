# Info tips close on their own, invite links are absolute, `--password -` reads stdin

## Why

An open (i) tip only closed from its own circle, so a reader who opened one and moved on found it still
open several tips later, on a phone as much as on a desktop.

Over SSH, `tix user invite` printed only `/invite/tix_inv_...` and told the admin "this is a local
database", even on a server with `server.public_url` set. The HTTP API and the browser built the full
link from the request, but the service never did, so SSH, a local database configured with the key and
the terminal interface's people view had only the path.

`tix user create --password -` took the dash as the password and failed on its length, although its own
example and `docs/tenancy.md` say `-` reads standard input. `user edit`, `tenant user create` and
`tenant user edit` did the same.

## What Changes

- An open info tip is light-dismissed: a press outside it and its circle, Escape (focus goes back to the
  circle), or opening another tip closes it. A press inside it leaves it open. The circle's
  `aria-expanded` follows it. Without scripting it is the same disclosure as before.
- The service sets an issued or re-issued invite's `url` under the same address as a task's `web_url`:
  the tenant's own domain, else `server.public_url`. `path` stays. The CLI's local database hint shows
  only when there is no `url`.
- `--password -` on `user create`, `user edit`, `tenant user create` and `tenant user edit` reads the
  password from standard input, less one trailing newline, and refuses an empty one. `login` already did.

The share menu and the other ⋯ menus are native `popover` (auto) elements, which already light-dismiss;
they are unchanged.

## Capabilities

### Modified Capabilities

- `web-ui`: an open info tip closes when the reader moves on.
- `cli`: invite links are absolute when an address is known; `--password -` reads standard input.
- `ssh-access`: `tix user invite` over SSH prints the absolute link.

## Impact

`internal/web/assets/tipplace.js`, `internal/service/weburl.go`, `internal/service/invite.go`,
`internal/core/invite.go` (doc comment), `api/openapi.yaml` (regenerated), `cmd/io.go`, `cmd/auth.go`,
`cmd/tenantuser.go`, `docs/web-ui.md`, `docs/configuration.md`, `docs/tenancy.md`.
