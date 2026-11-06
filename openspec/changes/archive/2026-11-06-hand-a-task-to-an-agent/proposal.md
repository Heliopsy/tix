# Hand a task to a coding agent from its screen

## Why

Giving a task to Claude Code, Codex or OpenCode meant writing the instruction by hand every time:
which task, which server, which commands claim it and close it. The owner asked for something on
the task screen that hands over a ready snippet instead.

## What Changes

- The task screen's header, as a page and in the task dialog, carries a "Hand to an agent" button
  that opens a dialog of copy-ready text for this task and this server.
- A prompt for an agent that drives the tix CLI, naming the real commands that claim the task, read
  it, comment on it and release it as done.
- One tab per client documented in `docs/mcp.md`: the command that starts that client on the task
  where it has one, with the prompt quoted safely for a POSIX shell, and the prompt to paste where
  it has none.
- The same prompt phrased for an agent that has the tix MCP tools.
- Each block has a Copy button; the last tab chosen is remembered per browser. Without scripting
  every client's text is drawn as plain blocks one under another.
- No token, ever, in any of it.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-tasks`: the task screen's hand to an agent dialog.

## Impact

- New `internal/handoff`: the prompt, the client list and the shell quoting, with no Cobra.
- `internal/web/taskdetail.go`, `internal/web/templates/task.html` (one include),
  `internal/web/templates/partials.html`, `internal/web/assets/handoff.js`, `app.css`, `icons.svg`,
  `templates/layout.html` (the script tag).
- `internal/i18n/bg_handoff.go`, `docs/web-ui.md`, `docs/agents.md`.
- No service, store or API change.
