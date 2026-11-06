# Design

## Where the text is built

`internal/handoff` builds the prompts and the per-client commands. It imports nothing from Cobra or
the web layer, so `cmd/` tests can feed every `tix ...` span the prompt names through the same
resolver the skill guard uses (`checkInvocation`) and fail on a command or flag the CLI does not
have, and `internal/mcp` tests can check every tool the MCP prompt names is one `tix mcp` exposes.

## Quoting

The prompt reaches a shell as one argument in single quotes, each `'` written as `'\''`. Inside
single quotes a POSIX shell expands nothing, so `$()`, backticks, `$VAR` and `"` in a title stay
literal. The title is folded to one line first (every run of whitespace and every control character
becomes one space), so the command is one line and nothing in it can end the line early. A test runs
the generated commands through `/bin/sh` and compares the argument it receives with the prompt.

Windows shells are not covered; the dialog says the commands are for a POSIX shell.

## Which prompt each client gets

Clients that run in a terminal (Claude Code, Codex, OpenCode, Gemini CLI, the Cursor CLI, VS Code's
`code chat`) get the CLI prompt, because the agent has a shell and the tix CLI reaches everything.
Claude Desktop and Zed get the MCP prompt to paste, since that is how `docs/mcp.md` connects them.
Each client starts interactive, not headless, so the reader can watch and approve tool calls:
`opencode --prompt` rather than `opencode run`, `gemini -i` rather than `gemini -p`.

## No scripting

Tabs are decoration. Without scripting every client's panel is drawn under its own heading and the
tab strip is not drawn (`data-needs-js`). With scripting, `assets/handoff.js` shows one panel, and
keeps the last chosen client in `localStorage` inside `try`/`catch`, so a browser that refuses
storage still works and only forgets.

## Prompt language

The prompt is English in every interface language. It is read by a model and quotes English command
names; the dialog's own words are translated.
