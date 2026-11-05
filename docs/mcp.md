# MCP tools

`tix mcp` serves the tix HTTP API to a [Model Context Protocol](https://modelcontextprotocol.io) client over
standard input and output. Every JSON operation of the API is one tool, generated at startup from the OpenAPI
document the binary carries ([api.md](api.md#openapi)), and every tool call is one request to a tix server with
your token. Nothing about a tool is written by hand except the few decisions in the overlay below, so a route
added to the API is a tool in the next release without anybody touching this command.

The CLI remains the complete interface. An agent that can run commands loses nothing by using it instead; see
[agents.md](agents.md) and the skill under `skills/tix`.

## Wiring it into a client

`tix mcp` talks to a running server and never opens a database. It reads the server and token the way every
other command does: `--server` and `--token`, `TIX_SERVER_URL` and `TIX_TOKEN`, or a context
([configuration.md](configuration.md)). Mint a token with only the scopes the agent needs:

```sh
tix token create claude --scope task:read --scope task:write --scope task:claim --scope comment:write -o json
```

Every client registers it the same way: `tix` with the argument `mcp` as a stdio server, and the two variables
in its environment. Below, URL stands for your server's address, such as `https://tix.example.com`, and TOKEN
for the token you just minted. Any client not listed here that launches a stdio server takes the same command
and environment.

Startup asks the server who the token belongs to, so a wrong URL or a refused token ends the process with a
message on standard error before the client sees any tools. A client started from a desktop launcher may not
see your shell's `PATH`; give it the absolute path to `tix` as the command if it cannot start the server.

Only stdio is served. An HTTP transport would hand your token's authority to whoever reaches the port, and
`tix serve` is already the network surface, with its own authentication.

### Claude Code

From the command line ([docs](https://code.claude.com/docs/en/mcp)):

```sh
claude mcp add tix --env TIX_SERVER_URL=URL --env TIX_TOKEN=TOKEN -- tix mcp
```

Or in `.mcp.json` at the root of a project:

```json
{
  "mcpServers": {
    "tix": {
      "command": "tix",
      "args": ["mcp"],
      "env": {
        "TIX_SERVER_URL": "URL",
        "TIX_TOKEN": "TOKEN"
      }
    }
  }
}
```

### Codex

From the command line ([docs](https://developers.openai.com/codex/mcp)):

```sh
codex mcp add tix --env TIX_SERVER_URL=URL --env TIX_TOKEN=TOKEN -- tix mcp
```

Or in `~/.codex/config.toml`, which the Codex CLI and IDE extension share:

```toml
[mcp_servers.tix]
command = "tix"
args = ["mcp"]

[mcp_servers.tix.env]
TIX_SERVER_URL = "URL"
TIX_TOKEN = "TOKEN"
```

### OpenCode

In `opencode.json` ([docs](https://opencode.ai/docs/mcp-servers/)). The command and its arguments are one
array, and the variables go under `environment`, not `env`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "tix": {
      "type": "local",
      "command": ["tix", "mcp"],
      "enabled": true,
      "environment": {
        "TIX_SERVER_URL": "URL",
        "TIX_TOKEN": "TOKEN"
      }
    }
  }
}
```

### Gemini CLI

In `.gemini/settings.json` in a project, or `~/.gemini/settings.json` for every project
([docs](https://geminicli.com/docs/tools/mcp-server/)). Gemini CLI starts stdio servers only in a folder you
have trusted:

```json
{
  "mcpServers": {
    "tix": {
      "command": "tix",
      "args": ["mcp"],
      "env": {
        "TIX_SERVER_URL": "URL",
        "TIX_TOKEN": "TOKEN"
      }
    }
  }
}
```

### Claude Desktop

In `claude_desktop_config.json`, which is `~/Library/Application Support/Claude/` on macOS and `%APPDATA%\Claude\`
on Windows ([docs](https://modelcontextprotocol.io/docs/develop/connect-local-servers)). Quit and reopen the app
after editing it:

```json
{
  "mcpServers": {
    "tix": {
      "command": "tix",
      "args": ["mcp"],
      "env": {
        "TIX_SERVER_URL": "URL",
        "TIX_TOKEN": "TOKEN"
      }
    }
  }
}
```

### Cursor

In `.cursor/mcp.json` in a project, or `~/.cursor/mcp.json` for every project
([docs](https://cursor.com/docs/context/mcp)):

```json
{
  "mcpServers": {
    "tix": {
      "type": "stdio",
      "command": "tix",
      "args": ["mcp"],
      "env": {
        "TIX_SERVER_URL": "URL",
        "TIX_TOKEN": "TOKEN"
      }
    }
  }
}
```

### VS Code

In `.vscode/mcp.json` in a workspace, for GitHub Copilot's agent mode
([docs](https://code.visualstudio.com/docs/agents/reference/mcp-configuration)). The top-level key is `servers`,
not `mcpServers`:

```json
{
  "servers": {
    "tix": {
      "type": "stdio",
      "command": "tix",
      "args": ["mcp"],
      "env": {
        "TIX_SERVER_URL": "URL",
        "TIX_TOKEN": "TOKEN"
      }
    }
  }
}
```

`code --add-mcp` writes the same server to your user profile instead:

```sh
code --add-mcp '{"name":"tix","type":"stdio","command":"tix","args":["mcp"],"env":{"TIX_SERVER_URL":"URL","TIX_TOKEN":"TOKEN"}}'
```

### Zed

In Zed's `settings.json`, opened with `zed: open settings file` ([docs](https://zed.dev/docs/ai/mcp)):

```json
{
  "context_servers": {
    "tix": {
      "command": "tix",
      "args": ["mcp"],
      "env": {
        "TIX_SERVER_URL": "URL",
        "TIX_TOKEN": "TOKEN"
      }
    }
  }
}
```

## What becomes a tool

| Rule | Effect |
| --- | --- |
| Name | The operationId with dots as underscores: `task.create` is `task_create` |
| Description | The operation's description from the document, then its method and path |
| Input | One object: path parameters, query parameters and the JSON body's properties side by side; when a body property collides with a parameter, or the body is not an object, the body goes whole under `body` |
| Result | The JSON response as text, and as structured content when it is an object; `204` is `{"status":"ok"}` |
| Error | A tool error carrying the API's envelope, `{"error":{"code":"not_found","message":"..."}}`, so a model reads the same kinds a script does ([scripting.md](scripting.md)) |
| Annotations | `GET` is read-only; `PUT` and `DELETE` are idempotent; destructive tools carry the destructive hint |

Operations that do not speak JSON are left out, whatever the overlay says. Use the CLI for those.

| Left out | Because it |
| --- | --- |
| `bundle_export` | answers a bundle stream (`tix bundle export`) |
| `bundle_import` | reads a bundle stream (`tix bundle import`) |
| `calendar_read` | answers an iCalendar feed |
| `event_subscribe` | upgrades to the WebSocket event stream (`tix watch`) |
| `project_icon` | answers an image |
| `snapshot_export` | answers NDJSON (`tix export`) |
| `snapshot_import` | reads NDJSON (`tix import`) |

`tix mcp --list` prints every operation, its tool name, the scope it needs and what became of it, without
contacting a server. `-o json` makes it a table a script can read.

```text
$ tix mcp --list -o json | jq -r '.[] | [.tool, .state] | @tsv' | head -4
actor_list      exposed
actor_show      exposed
api_spec        hidden
artifact_list   exposed
```

## Scopes and destructive tools

Two rules decide which tools a session is offered, on top of the overlay.

- **Scope.** A tool whose scope the token does not hold is not listed. The server would refuse the call anyway;
  listing it would only spend the model's attention on a tool it cannot use. The scopes are read once, at
  startup, from `GET /api/v1/whoami`.
- **Destructive.** A tool that deletes or cannot be undone is left out unless `--allow-destructive` is given or
  the overlay enables it. Every `DELETE` is destructive unless the overlay says otherwise, and the overlay marks
  the few that are not deletes but lose data all the same: `task_bulk`, `claim_reclaim`, `retention_prune` and
  `housekeeping_run`. A model that is wrong about one task should cost one task, not a project.

A token without `task:delete` is never offered `task_delete`, with or without the flag.

## The overlay

`internal/mcp/overlay.yaml` ships in the binary and holds the decisions the document cannot make. `--overlay
PATH` lays a file of your own over it, field by field, so one entry can change one thing:

```yaml
operations:
  task.delete:
    enable: true          # offer it without --allow-destructive
  task.show:
    name: show_task       # rename the tool
    description: Show one task by its ref, such as infra-42.
  audit.list:
    hide: true
    reason: the agent has no business reading the audit log
```

| Key | Meaning |
| --- | --- |
| `hide` | Leave the operation off the tool list. Needs a `reason`. |
| `reason` | Why it is hidden, or why its destructive marking departs from its method. |
| `name` | The tool name, instead of the one derived from the operationId. |
| `description` | The tool description, instead of the document's. |
| `destructive` | Mark the tool destructive or not. A departure from the method's default needs a `reason`. |
| `enable` | Offer a destructive tool without `--allow-destructive`. |
| `hide_params` | Optional parameters to leave out of the input, such as `format` on `task.list`, which would ask for CSV. |

An overlay naming an operation the API does not have, an unknown key, a hidden operation without a reason, a
tool name two operations share, or a name outside `[A-Za-z0-9_-]` stops `tix mcp` with exit 2 before it serves
anything.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | the client closed the session |
| 2 | no server is configured, or the overlay is invalid |
| 5 | the server refused the token |
