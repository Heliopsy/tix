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

Claude Code, in `.mcp.json` at the root of a project or through `claude mcp add`:

```json
{
  "mcpServers": {
    "tix": {
      "command": "tix",
      "args": ["mcp"],
      "env": {
        "TIX_SERVER_URL": "https://tix.example.com",
        "TIX_TOKEN": "tix_..."
      }
    }
  }
}
```

```sh
claude mcp add tix --env TIX_SERVER_URL=https://tix.example.com --env TIX_TOKEN=tix_... -- tix mcp
```

Any other client that launches a stdio server takes the same command and environment. Startup asks the server
who the token belongs to, so a wrong URL or a refused token ends the process with a message on standard error
before the client sees any tools.

Only stdio is served. An HTTP transport would hand your token's authority to whoever reaches the port, and
`tix serve` is already the network surface, with its own authentication.

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
