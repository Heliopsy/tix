# Connect an agent

## Why

Pointing an agent at tix takes five pieces of information from five places: how to install the CLI,
a token with the right scopes, the server's address, the tenant key, and the MCP snippet for whichever
client the person uses. docs/mcp.md has the snippets, but with `URL` and `TOKEN` left for the reader
to fill in, and a wrong guess shows up as a client that silently lists no tools.

The server already knows its own address, the tenant, the reader and the version it runs. It can
hand out every command filled in.

## What Changes

- A **Connect an agent** screen at `/connect`, in the main menu for everyone signed in and linked
  from the API tokens screen. It shows, filled in for this server and this reader: installing the CLI,
  issuing a token, a CLI context, MCP setup for eight clients, the agent skill, SSH (only when this
  server runs an SSH listener) and a REST example.
- An inline form issues a token for the reader from preset scope bundles or a custom set, offering
  only scopes the reader holds. The value is shown once, and the page's script writes it into every
  command on the screen. It is never stored, put in a URL, or rendered into the commands by the server.
- A new configuration key, `ssh.public_address`, names the host and port people reach the SSH
  listener at when that differs from where `tix serve --ssh-listen` binds.
- A **Docs** entry in the main menu, and a docs link on each section of the screen. Both go through
  `/docs/{page}`, which redirects to that page of the documentation for the running version, so no
  rendered page names an external origin.
- The MCP snippets come from one Go source, `internal/onboard`, which a test checks against
  docs/mcp.md. Another test checks that every `tix` command on the screen exists with those flags.

## Impact

- New package `internal/onboard`; new routes `GET /connect`, `POST /connect/token`, `GET /docs`
  and `GET /docs/{page}`; new asset `connect.js`; new `connect` icon.
- `config.SSH.PublicAddress`; `tix serve` passes it and `--ssh-listen` to the web handler.
- docs/web-ui.md, docs/agents.md, docs/mcp.md, docs/configuration.md.
