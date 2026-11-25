---
name: tix-getting-started
description: Get a person started with tix, the task tracker shared by humans and AI agents. Use this when somebody is new to tix, asks "how do I start", "how do I install tix", "what is a tenant", "what is an instance", "which app should I use", or wants to connect a desktop app, the web browser, the Android app, the CLI, the terminal UI, SSH or an AI agent (MCP or CLI) for the first time. Covers asking how they want to use tix, the concepts (instance, tenant, project, task, workflow, people, agents and credentials, roles) with one running example, setup per interface, a first task walked through its workflow, and where to go next. For day-to-day work once tix is set up, use the main `tix` skill instead.
version: 1
verified-against: tix e3f99f9e (2026-11-09)
---

# Getting started with tix

This skill is for the first hour: somebody who has not used tix, or is not
sure what it is, and wants to get from nothing to a first task. Once they are
set up, hand over to the main `tix` skill
([tix.red/skill.md](https://tix.red/skill.md)), which covers the everyday loop.

The commit this file was checked against is the `verified-against` field above,
and nowhere else. `cmd/skill_getting_started_test.go` reads it and fails if it
names a commit that is not an ancestor of HEAD, or if this file names a command
or flag the CLI does not have. Run `tix version` against the build you are
driving; if it is much newer than that commit, trust `tix <command> --help`
where the two disagree.

## Rules for you, the agent

- **Never touch a real store without asking.** A `tix` command with no `--db`
  and no context can resolve to the person's own database at
  `~/.local/share/tix/tix.db`, which may hold real work. Before running
  anything that writes, run `tix whoami` and tell the person what it says.
- **Try things on a scratch database.** For a demonstration, or when the
  person just wants to see tix work, pass `--db` with a path in a temporary
  directory on every command, and say that it is a throwaway copy.
- **Never print secrets.** Passwords, API tokens, invite links, recovery codes
  and lease tokens are shown once by tix. Do not echo them back, paste them
  into chat, write them into files the person did not ask for, or put them on
  a command line that lands in shell history when a `-` (standard input)
  option exists. Say "paste your token here" rather than inventing one.
- **Ask before you assume.** Step 1 below comes first, every time.

## Step 1: ask how they want to use tix

Ask both questions before explaining or installing anything. Offer the
choices as a short list.

**How do you want to use tix?**

1. Desktop app, on Linux, macOS or Windows.
2. A web browser.
3. The mobile app, on Android. (iOS is not available yet.)
4. The command line (CLI).
5. The terminal UI (TUI), a board in the terminal.
6. Over SSH, with nothing installed but an SSH client.
7. Connecting an AI agent, through MCP or the CLI.

Several answers are fine; most people pair one of 1 to 3 with 4.

**Where does your tix live?**

- **I already have a server and an account.** Ask for the server address
  (for example `https://tix.z09.org`), the tenant key if they know it, and
  whether they sign in with a password or were given an API token.
- **I want to try it on my own machine.** A local database for one person, no
  server.
- **I want to run a server for a team** (self-host). Point them at the
  deployment guide; setting up a server is more than a first hour.

## Step 2: explain the concepts

Use plain language and one example the whole way through. The example below
is the one to use unless the person gives you their own names.

> Ada works at a design studio. The studio runs a tix server at
> **tix.z09.org**. On it, the studio has a tenant called **studio**. Inside
> it there is a project **FIDARO** for a client website, and its twelfth task
> is **FIDARO-12, "Fix the contact form"**. A coding agent works some of
> those tasks next to Ada.

```text
instance  tix.z09.org                    (a tix server, or a local database)
 └─ tenant  studio                       (a workspace: people, projects, settings, tokens)
     ├─ people and agents  ada (admin), raj (member), fidaro-bot (agent token)
     └─ project  FIDARO                  (a set of tasks, with a workflow)
         └─ task  FIDARO-12 "Fix the contact form"
             todo ──> doing ──> done     (the workflow's states and moves)
```

- **Instance.** A tix server, such as `tix.z09.org`, that everyone reaches
  over the network. Or a local database file on one machine for a single
  user, with no server at all. The apps call a server they know about an
  instance, and can hold several.
- **Tenant.** A separate workspace on an instance, with its own people,
  projects, workflows, settings and tokens. Nothing crosses between tenants.
  One person can belong to several tenants on the same instance, with the same
  email and password, and switch between them; they may be an admin in one
  and a member in another. A single-user setup has one tenant, `default`, and
  never needs to think about it.
- **Project.** A set of tasks with a short key, like `FIDARO`. Task references
  are the key plus a number: `FIDARO-12`. tix shows keys in upper case and
  accepts them in any case.
- **Task.** One piece of work: a title, a description, a status, a priority,
  an assignee, a due date, tags, comments and subtasks. A task can also depend
  on another, which keeps it out of the queue until the other is finished.
- **Workflow.** The states a task moves through and which moves are allowed.
  Every project has one. The built-in `default` workflow is `todo`, `doing`,
  `blocked`, `done` and `cancelled`, and there is no direct move from `todo` to
  `done`: work goes through `doing`. A team can write its own. Newer builds
  also ship ready-made templates whose keys start with `tix-` (such as
  `tix-kanban`, `tix-gtd`, `tix-dev-review`, `tix-bugs`, `tix-releases`,
  `tix-agent-review`, `tix-incident`, `tix-support`, `tix-approval`,
  `tix-sales`, `tix-hiring` and `tix-content`). Mention them only if the
  person's version has them: check whether `tix workflow --help` lists a
  `template` subcommand, and if it does, list them with its `ls`.
- **People, agents and credentials.** A person signs in with an email and a
  password, plus a second factor (an authenticator app code) when they or the
  tenant turn it on. A person can also enrol an SSH public key and use it for
  SSH and for signing in from the command line. Agents and scripts never use a
  person's password: they get an **API token** with only the scopes they need
  (for example read tasks, claim tasks, move tasks, comment). A token belongs
  to one tenant, expires after 90 days by default, and can be rotated, which
  issues a new value and can keep the old one working for a few minutes.
- **Roles.** In each tenant a person is a `viewer` (reads), a `member` (also
  creates, edits, moves and claims tasks and comments) or an `admin`
  (everything in that tenant, including people and tokens). Separately, a
  **global admin** runs the instance itself: creates tenants and manages
  anyone's account. Roles are per tenant; global admin is per instance.

Stop here and check the person is with you before the setup. Keep this part
to a minute of reading.

## Step 3: set up the interface they chose

Use the person's server address and tenant in place of `tix.z09.org` and
`studio`. Do only the sections for what they picked.

### Desktop app (Linux, macOS, Windows)

Linux and macOS:

```sh
curl -fsSL https://tix.red/install-app.sh | sh
```

Windows, in PowerShell:

```powershell
irm https://tix.red/install-app.ps1 | iex
```

The script downloads the latest release, checks it against the release
checksums, installs it and starts it. Running it again updates. The macOS
build is not signed and notarised yet, so the first launch takes an extra
step, and Windows SmartScreen warns on first launch. Downloads and the same
notes are at <https://tix.red/en/apps/>.

In the app, add an instance: the server address and either a sign-in (email,
password, and a code when the account has a second factor) or an API token.
Or choose a local database to keep everything on this computer. The current
instance and tenant show as **Instance › Tenant**; tap it to switch.

### Web browser

Open the server's address, such as `https://tix.z09.org`, and sign in at
`/login` with email and password, then the code if asked. A new person usually
gets an invite link from an admin instead, which opens a page to choose a
password. Somebody in several tenants on that server picks one on the Choose a
tenant page, and switches later from the tenant name at the top of the
sidebar.

To try the browser without a server, start one on this machine over a scratch
database (see "Try it locally" below) and open `http://127.0.0.1:8080`.

### Mobile app (Android)

iOS is not available yet; offer the web browser on an iPhone instead.

1. On the phone, open <https://tix.red/en/apps/#android> and download the
   **arm64-v8a** APK (right for almost any phone from the last several years;
   the page lists the others).
2. Open the file. If Android asks, turn on **Allow from this source** for the
   browser or file manager, and go back.
3. Tap **Install**. Play Protect may warn that it does not know the app,
   because it is not from Google Play; for the official release that is
   expected.
4. Open tix and add the server with a sign-in or a token, or create a local
   database on the phone.

From a phone, `127.0.0.1` is the phone itself, so a server on the person's
computer is not reachable that way; use the server's real address.

### Command line (CLI)

Linux and macOS:

```sh
curl -fsSL https://tix.red/install.sh | sh
```

Windows, in PowerShell:

```powershell
irm https://tix.red/install.ps1 | iex
```

Both verify the checksum. Then check where commands will go:

```sh
tix version
tix whoami
```

**With a server and an account.** If the person is an admin of their tenant,
`tix login --tenant` signs in and stores a token for that tenant in a named
context:

```sh
tix --server https://tix.z09.org login --tenant studio --context studio --email ada@example.com --password -
tix whoami
```

`--password -` reads the password from standard input so it stays out of shell
history. Minting that token needs the admin role in the tenant. Anyone else
asks an admin for a token (the web UI's Connect an agent page issues one) and
adds it to a context. Have the person paste it themselves rather than through
you:

```sh
tix ctx add studio --server https://tix.z09.org --token "$TIX_TOKEN" --use
tix whoami
```

Carry on only when `tix whoami` names the server and says a token is present.
Contexts are how one person keeps several servers or tenants apart:

```sh
tix ctx ls
tix ctx use studio
tix tenant ls
```

`tix tenant ls` lists the tenants the person belongs to, with the role in each
and the context holding a token for it, and `tix tenant use KEY` switches to
another one.

**Local, for one person.** `tix init` sets up a local database on this machine
and makes it the default. It is meant for the person's real work, so ask first.
For trying things out, use a scratch database instead (below).

### Terminal UI (TUI)

The TUI is part of the CLI, so install the CLI first and set it up as above.
Then:

```sh
tix tui
```

It opens on whatever `tix whoami` names. `ctrl+k` lists every action with its
key, and `?` lists the keys of the current view. `tix tui --tenant studio`
starts in another tenant, and `T` opens the tenant view inside it.

### SSH

Nothing to install, if the server runs tix's SSH listener. Ask the person, or
their admin, for the SSH host and port: the web UI's Connect an agent page
shows them on its SSH tab when the listener is on.

1. Enrol a public key. In the browser it is the SSH keys screen under the
   account (`/admin/ssh-keys`); from a CLI already set up for the tenant:

   ```sh
   tix user key add --file ~/.ssh/id_ed25519.pub --label laptop
   ```

   Enrol the `.pub` file, never the private key.

2. Connect. The SSH username is the **tenant key**, not a login name:

   ```sh
   ssh -p 2222 studio@tix.z09.org
   ```

   A key enrolled in only one tenant can also use the neutral user `tix`
   (`ssh -p 2222 tix@tix.z09.org`). With no command it opens a start menu;
   `ssh -t -p 2222 studio@tix.z09.org FIDARO` opens that project's board, and
   `ssh -p 2222 studio@tix.z09.org task ls -o json` runs one tix command.

### Connecting an AI agent

An agent always gets its own API token, never the person's password or
browser session. Ask the person to open **Connect an agent** in the web UI
(`/connect`, in the sidebar and on the API tokens screen). It issues a token
with a scope picker (the Agent preset is the usual start) and fills it into
ready-to-copy commands, one tab per way in: CLI, MCP (with a tab per client:
Claude Code, Codex, OpenCode, Gemini CLI, Claude Desktop, Cursor, VS Code,
Zed), SSH, API, and Skill.

An admin can also mint one from the CLI, with an expiry:

```sh
tix token create fidaro-bot --scope task:read --scope task:write --scope task:transition --scope task:claim --scope comment:write --expires 30d
```

The value prints once. The person stores it where the agent reads it; you do
not repeat it.

- **CLI agents.** Install the CLI, then add the token to a context as in the
  CLI section. Install the main `tix` skill for the agent from the Skill tab,
  or from <https://tix.red/skill.md>.
- **MCP.** `tix mcp` serves the server's API as MCP tools. It talks to a
  running server and never opens a database, so it needs a server address and
  a token, from a context or the environment. For a project's `.mcp.json`:

  ```json
  {
    "mcpServers": {
      "tix": {
        "command": "tix",
        "args": ["mcp"],
        "env": { "TIX_SERVER_URL": "https://tix.z09.org", "TIX_TOKEN": "PASTE_TOKEN_HERE" }
      }
    }
  }
  ```

  The Connect an agent page has the exact snippet for each client, and
  `tix mcp --list` prints every operation with its tool state.
- **Rotation.** When the token has to change,
  `tix token rotate fidaro-bot --grace 10m` issues a new value and keeps the
  old one working for ten minutes so a running agent is not cut off.
  `tix token ls` shows every token and when it expires; `tix token rm ID`
  revokes one at once.

### Try it locally, on a scratch database

This is the safe way to show tix to somebody, or to follow the first-task
walkthrough without a server. Every command names the scratch file, so
nothing touches the person's real data:

```sh
export TIXDB="$(mktemp -d)/tix-try.db"
tix --db "$TIXDB" whoami
tix --db "$TIXDB" tui
```

To see the browser on it, invite an admin account and start a server bound
to `127.0.0.1`:

```sh
tix --db "$TIXDB" user invite ada@example.com --role admin
tix --db "$TIXDB" serve
```

Against a database the invite prints only a path, `/invite/tix_inv_...`. The
person opens `http://127.0.0.1:8080` followed by that path, chooses a password
and is signed in. The link is a secret: let them copy it from their own
terminal rather than repeating it. Stop the server with ctrl-c when done, and
delete the temporary directory.

## Step 4: a first task, walked through

Use the same example. On a scratch database, add `--db "$TIXDB"` after `tix`
on every line; on a server, the context already says where to go, so confirm
with `tix whoami` first and ask before creating anything there.

```sh
tix project create fidaro "Fidaro website"
tix task add "Fix the contact form" -p fidaro --priority high --due 2026-12-01 --tag web
tix task ls -p fidaro
```

The first task in a new project is `FIDARO-1`. In a real project it might be
`FIDARO-12`; use whatever reference `tix task add` printed. Then add a
subtask, a comment, and move it through the workflow:

```sh
tix task add "Check the spam filter" -p fidaro --parent fidaro-1
tix comment add fidaro-1 "The form posts, but the email never arrives"
tix task mv fidaro-1 doing
tix task mv fidaro-1 done
tix task show fidaro-1
```

Pass `-p` on a subtask too: without it the subtask goes to the default
project. `tix task mv fidaro-1 done` straight from `todo` is refused (exit `6`,
naming the allowed moves), because the default workflow has no such move; that
is the workflow doing its job. The same steps
in the browser: open the project, use the add button in the header, open the
task, and use its move menu. In the TUI, `ctrl+k` finds the same actions by
name.

`tix task add` also reads a one-line shorthand, where `!` is the priority,
`#` a tag, `@` an assignee's handle (who must exist in the tenant), `due:` the
due date and `+` the project:
`tix task add --parse "Fix the contact form !high #web due:fri +fidaro"`.

## Step 5: where to go next

- Day-to-day work, and anything an agent does in a queue: the main `tix`
  skill, <https://tix.red/skill.md>, or `skills/tix/SKILL.md` in the
  repository.
- The guides, starting from the index:
  <https://github.com/heliopsy/tix/blob/main/docs/README.md>.
- Tenants, roles and switching:
  <https://github.com/heliopsy/tix/blob/main/docs/tenancy.md>.
- Workflows and custom fields:
  <https://github.com/heliopsy/tix/blob/main/docs/workflows.md>.
- Agents, leases and tokens:
  <https://github.com/heliopsy/tix/blob/main/docs/agents.md>, and MCP clients:
  <https://github.com/heliopsy/tix/blob/main/docs/mcp.md>.
- The browser: <https://github.com/heliopsy/tix/blob/main/docs/web-ui.md>.
  The terminal: <https://github.com/heliopsy/tix/blob/main/docs/tui.md>.
- Running a server, including SSH:
  <https://github.com/heliopsy/tix/blob/main/docs/deployment.md>.
- The apps: <https://tix.red/en/apps/>.

`tix <command> --help` documents every flag of the build in front of you.
