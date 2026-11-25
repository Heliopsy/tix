# Tabs on the Connect an agent screen

## Why

The Connect an agent screen draws every way to connect one under another: install, token, CLI context, eight
MCP clients, the skill, SSH and the REST API. The owner called it really confusing. A reader who wants one
thing, Codex over MCP or a curl call, scrolls past everything else and cannot tell which steps their way
needs. The SSH section also predates the ssh command surface (0.48.0) and only offers the terminal
interface, and the CLI section does not mention `tix login --tenant` or `tix token rotate`.

## What Changes

- The screen reads as three steps: 1. get a credential (the token form, shown once above the tabs),
  2. pick how you connect (a tab strip), 3. copy the commands (the tab's panel).
- Five tabs: CLI, MCP, SSH, API and Skill. Each is a link to `/connect?tab=NAME`, so it can be shared and
  works without scripting; the server draws the named tab selected and the others hidden.
- The nested choices (install method, MCP client, skill location) are tabs of the same kind, linkable as
  `?install=`, `?client=` and `?skill=`, so `/connect?tab=mcp&client=codex` opens Codex.
- With scripting the strips are ARIA tabs: arrow keys, Home and End move between them, a click switches
  without a reload, and the address is rewritten so a copied URL names the tab on screen. The task screen's
  tab script (`assets/tasktabs.js`) drives both screens; the connect screen's own tab code goes.
- Each tab carries a help link (?) to the documentation section for it.
- The CLI tab gains `tix login --tenant` for a reader who can mint their own token, and the credential step
  gains `tix token rotate`. The SSH tab gains the ssh command surface (`ssh acme@host task ls -o json`,
  export over ssh) and is drawn on every server, saying how to turn the listener on when there is none.
- The last tab is remembered in the URL only: there is no per-person preference store to keep it in.

## Capabilities

### Modified Capabilities

- `web-ui`: the Connect an agent screen's layout, tabs, SSH commands and help links.

## Impact

`internal/web` (connect.go, templates/connect.html, assets/tasktabs.js, assets/connect.js,
assets/css/610-connect-agent.css), `internal/onboard` (login and rotate snippets, ssh commands),
`internal/i18n/bg_connect.go`, docs/web-ui.md, docs/agents.md, and their tests. No HTTP, CLI or storage
change.
