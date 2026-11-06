## ADDED Requirements

### Requirement: A screen for connecting an agent

The browser interface SHALL serve a Connect an agent screen at `/connect` to every signed-in reader,
reachable from the main menu and from the API tokens screen. It SHALL show commands for installing the
CLI, issuing a token, configuring a CLI context, registering `tix mcp` with each client docs/mcp.md
names, installing the agent skill and calling the REST API, each with a copy button.

#### Scenario: Commands name this server

- **GIVEN** a reader signed in to tenant `acme` at `https://tix.example.com`
- **WHEN** they open `/connect`
- **THEN** the CLI context command reads `tix ctx add acme --server https://tix.example.com` with the
  reader's tenant key, and no command carries the placeholder `URL`

### Requirement: Issuing a token on the connect screen

The connect screen SHALL issue an API token for the reader from a preset scope bundle or a custom set,
offering only scopes the reader holds, and SHALL show the value once. Its script SHALL write the value
into every command on the screen without storing it or placing it in a URL. Before a token is issued
the commands SHALL read `$TIX_TOKEN`.

#### Scenario: The token fills every command

- **GIVEN** a reader holding `token:admin`
- **WHEN** they issue a token with the agent preset
- **THEN** the screen shows the value once, and every token position in the commands holds it

#### Scenario: A reader who cannot issue tokens

- **GIVEN** a reader without `token:admin`
- **WHEN** they open `/connect`
- **THEN** no issuing form is drawn and the commands read `$TIX_TOKEN`

### Requirement: The SSH section follows the listener

The connect screen SHALL show the SSH commands only when the serving process runs an SSH listener.
The address SHALL be `ssh.public_address` when it is set, otherwise the request's host name and the
port the listener binds.

#### Scenario: A container publishes another port

- **GIVEN** `tix serve --ssh-listen 0.0.0.0:2222` with `ssh.public_address` set to `tix.example.com:2020`
- **WHEN** a reader in tenant `acme` opens `/connect`
- **THEN** the screen shows `ssh -p 2020 acme@tix.example.com`

### Requirement: Documentation is a menu entry

The main menu SHALL carry a Docs entry, opening in a new tab, and each section of the connect screen
SHALL link to its documentation page. Both SHALL go through `/docs/{page}`, which redirects to that
page for the running version and SHALL refuse a page that is not in the documentation.

#### Scenario: An unknown page is not redirected

- **WHEN** a client asks for `/docs/evil.md`
- **THEN** the answer is 404 and no redirect is issued

### Requirement: One scope picker for every token form

Every browser token form SHALL draw each scope as a labelled checkbox in a group with a note on what it
allows. A scope the reader does not hold SHALL be drawn disabled with the reason. Select all SHALL tick
each scope the reader holds and never `*`, which SHALL be a separate choice offered only to a reader
holding it. The issued token's confirmation SHALL list the scopes it was granted.

#### Scenario: A scope the reader lacks

- **GIVEN** a reader holding `token:admin` and `task:read` only
- **WHEN** they open the token form and submit `task:write` anyway
- **THEN** the `task:write` box is disabled with its reason, and the submission is refused with 403
