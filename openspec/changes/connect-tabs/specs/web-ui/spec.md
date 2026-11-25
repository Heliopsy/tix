## MODIFIED Requirements

### Requirement: A screen for connecting an agent

The browser interface SHALL serve a Connect an agent screen at `/connect` to every signed-in reader,
reachable from the main menu and from the API tokens screen. It SHALL read as three numbered steps: get a
credential, pick how you connect from tabs named CLI, MCP, SSH, API and Skill, and copy the commands in the
chosen tab, each with a copy button.

#### Scenario: Commands name this server

- **GIVEN** a reader signed in to tenant `acme` at `https://tix.example.com`
- **WHEN** they open `/connect?tab=cli`
- **THEN** the CLI context command reads `tix ctx add acme --server https://tix.example.com` with the
  reader's tenant key, and no command carries the placeholder `URL`

#### Scenario: Each tab carries its commands

- **WHEN** a reader opens `/connect` with `tab` set to each of `cli`, `mcp`, `ssh`, `api` and `skill`
- **THEN** that tab is selected and its panel holds the install and context commands, the `tix mcp`
  registration for each client docs/mcp.md names, the ssh commands, the curl calls, or the skill install

### Requirement: The SSH section follows the listener

The connect screen's SSH tab SHALL show the SSH commands, the terminal interface and tix commands run over
ssh, only when the serving process runs an SSH listener, and otherwise SHALL say the server runs none. The
address SHALL be `ssh.public_address` when it is set, otherwise the request's host name and the port the
listener binds.

#### Scenario: A container publishes another port

- **GIVEN** `tix serve --ssh-listen 0.0.0.0:2222` with `ssh.public_address` set to `tix.example.com:2020`
- **WHEN** a reader in tenant `acme` opens `/connect?tab=ssh`
- **THEN** the tab shows `ssh -p 2020 acme@tix.example.com` and `ssh -p 2020 acme@tix.example.com task ls -o json`

#### Scenario: No listener

- **GIVEN** a server without an SSH listener
- **WHEN** a reader opens `/connect?tab=ssh`
- **THEN** the tab draws no ssh command and says the server runs no SSH listener

### Requirement: Documentation is a menu entry

The main menu SHALL carry a Docs entry, opening in a new tab, and each tab of the connect screen SHALL carry
a help link to its documentation section. Both SHALL go through `/docs/{page}`, which redirects to that
page for the running version and SHALL refuse a page that is not in the documentation.

#### Scenario: An unknown page is not redirected

- **WHEN** a client asks for `/docs/evil.md`
- **THEN** the answer is 404 and no redirect is issued

#### Scenario: A tab's help link

- **WHEN** a reader opens the MCP tab's help link
- **THEN** it redirects to docs/mcp.md

## ADDED Requirements

### Requirement: Connect screen tabs are links

Each tab on the connect screen, and each nested tab for an install method, an MCP client or a skill
location, SHALL be a link naming it in the query (`tab`, `install`, `client`, `skill`) and keeping the other
parameters. The server SHALL draw the named tab selected and every other panel hidden, so the screen works
without scripting. An unknown value SHALL select the first tab.

#### Scenario: Codex without scripting

- **WHEN** a browser without scripting follows the MCP tab and then the Codex tab
- **THEN** it lands on `/connect?tab=mcp&client=codex` with the Codex commands shown and the other clients
  hidden

#### Scenario: An unknown tab

- **WHEN** a reader opens `/connect?tab=nope`
- **THEN** the CLI tab is selected

### Requirement: Connect screen tabs switch in place

With scripting the connect screen's tab strips SHALL behave as ARIA tabs, with the task screen's tab
script: a click or Enter switches the panel without a reload, the arrow keys, Home and End move focus and
selection, and the address SHALL be rewritten to name the chosen tab. The choice SHALL NOT be kept in
browser storage.

#### Scenario: Arrow to the next tab

- **GIVEN** the CLI tab focused with scripting
- **WHEN** the reader presses the right arrow
- **THEN** the MCP tab is selected and focused, its panel shows, and the address reads `?tab=mcp`

### Requirement: One credential step above the tabs

The connect screen SHALL draw the token step once, above the tabs, for every tab that reads `$TIX_TOKEN`,
with the command to mint the token and to rotate it. It SHALL say that the SSH tab uses an enrolled SSH key
instead.

#### Scenario: The token step is shared

- **WHEN** a reader holding `token:admin` opens `/connect?tab=api`
- **THEN** the token form is drawn once, before the tab strip, and the API panel holds no form of its own
