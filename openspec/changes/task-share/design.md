# Design

## No registry operation

The capability registry binds `core.Service` methods, and a share link is not one. Everything it
holds the reader already has: the reference and title come from `GetTask`, which is registered, and
the address is the host the reader reached the server at, or the server's configured public address
(`public-url-and-tui-share`). A service method would have to be handed the address by its caller,
which makes it formatting with a round trip.

So the share is client-side formatting in `internal/tasklink`, used by the browser interface and the
CLI. The CLI command reads the task with `GetTask` so an unknown or unreadable reference fails with
exit 3 or 5, and a reference under a project's old key prints the current one.

`public-url-and-tui-share` hosts `tix task link` over SSH beside `task show`, and adds the terminal
interface's share key.

## The address

The browser builds it from `window.location.origin` and the task's path, so it is exactly the host
the reader is on, behind any proxy. The server draws the same address, from the request's scheme
and host, as the plain link the menu shows without scripting. The CLI uses the server URL of the
context it talks to, or `--base`; a local database has no web address and is refused with exit 2
unless `--base` names one. Where `server.public_url` is set, every surface prefers it on a shared host
(`public-url-and-tui-share`).

The path is `/tasks/REF` with the stored, lower-case reference, the same path every link in the
interface uses. A project's old key answers with a redirect, so an old link keeps working.

## Shared hosts

The link carries no tenant. On a shared host the tenant is resolved by the credential, so a member
of the same tenant whose session is in that tenant opens the task. The interface refuses a request
that names a tenant (`rejectTenantOverride`), so a `?tenant=` would only break the link. A member of
several tenants whose session is in another one switches tenant first; a reader who is not signed
in signs in and is returned to the task. This is written down in `docs/tenancy.md` and
`docs/web-ui.md`.

## The browser

- The menu is a popover like the actions menu, so it opens, closes on Escape and works from the
  keyboard without a script; with one, the arrow keys move along it (`lanepop`, `assets/lanes.js`).
- Share… is shown only where `navigator.share` exists (`:root.can-share`, set by `assets/share.js`).
  A cancelled sheet does nothing; a refused one falls back to copying the link.
- Copies go through `tix.copyText`, with the legacy fallback `assets/copy.js` already has. The
  outcome is a toast, inserted with `role="status"` into the open dialog or `#toast`, and focus
  returns to the Share button.
- The Markdown escapes `\`, `[` and `]` in the title and folds it onto one line, in Go and in
  JavaScript alike, with the same cases tested on both sides.
