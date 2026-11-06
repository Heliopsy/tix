# Design

## One source for the snippets

`internal/onboard` builds every command and configuration snippet from a `Target`: server URL,
tenant key, handle, token, SSH address and running version. It imports only the standard library.
The web handler renders it; `internal/onboard`'s own test renders it with `URL` and `TOKEN` and
asserts each MCP snippet appears verbatim under its client's heading in docs/mcp.md and that every
JSON snippet parses. `cmd/connect_test.go` walks every snippet, finds each `tix` at a command
position (start of a line, after `--`, inside a JSON or TOML `command`), and resolves it against the
real Cobra tree with the helpers the skill guard uses.

## Filling in the token

The server renders each token position as a `<span data-token-slot>` holding `$TIX_TOKEN`. After a
token is issued, the screen draws it once in the usual one-time secret block, read from a
short-lived HttpOnly cookie the POST set, exactly as `/admin/tokens` does. `connect.js` copies that
value into every slot. The copy buttons copy `textContent`, so they copy the filled command. Nothing
is persisted, and the value appears in the HTML once. Without scripting the commands keep
`$TIX_TOKEN` and the value is copied from the secret block by hand.

## Tenant key

`GetTenant` needs `tenant:admin`, so the key is known only to a reader who may read the tenant. Every
command works without it: a token names its tenant, the context is then named `tix` and has no
`--tenant`, and the SSH user is `tix`, which connects a key enrolled in one tenant.

## Scope picker

One partial, `scope-picker`, draws every token form's scopes: a checkbox per scope in groups, each
with a note, and a disabled box with the reason for a scope the reader lacks. Presets, Select all,
Clear, the tri-state group boxes and the summary come from `scopes.js` and are not drawn without
scripting. Select all ticks each held scope and never `*`, which an administrator ticks as its own
choice. The one-time cookie carries the granted scopes beside the value, so the confirmation lists
what the service granted.

## Server address

The URL is the scheme the request arrived with, honouring `server.trusted_proxies`, and its `Host`,
the same derivation the calendar feed uses. The SSH address is `ssh.public_address` when set,
otherwise the request's host name and the port `--ssh-listen` binds. The section is drawn only when
`tix serve` runs an SSH listener.

## Tabs without folds

Sections are headed and always drawn. The MCP clients, install methods and skill targets are tab
sets: buttons with `role="tab"` that `connect.js` wires up. Without scripting every panel is drawn
one after another under its own heading, so nothing is hidden behind a control that does not work.

## Docs links

The (i) tips name a docs file as text and link nowhere, and `TestPagesReferenceNoExternalOrigin`
forbids an external origin in any page. The menu entry and the section links therefore point at
`/docs/{page}`, which answers a redirect to the file on GitHub at the running release's tag, or at
`main` for a development build. Only pages in a fixed list are redirected, and a test checks that
list against `docs/`.
