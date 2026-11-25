# Design: SSH command surface

## Context

`internal/sshd` owns the listener, identity, capacity and the second factor. The CLI command tree lives
in `cmd/`, and no package under `internal/` may import Cobra, so the listener cannot build the tree
itself. Every CLI command reaches the service through `globals.dial`, which returns the connection
cached on `globals` when one is already set.

## Decisions

### The runner is injected

`sshd.Options.Commands` is a `CommandRunner`, a function taking an `Invocation`: the words of the
command, the session's streams, the service, the actor, the via record and whether a terminal is
attached. `cmd.RunHosted` implements it, and `tix ssh` and `tix serve --ssh` pass it in. Without a
runner the listener keeps refusing commands, which is also what a demo listener does: a sandbox key
is a stranger, and commands such as `webhook put` make the server reach out to addresses the caller
picks.

### In-process, on a pre-set connection

`RunHosted` builds a fresh command tree per invocation, the same `newRoot` the binary runs, and sets
`globals.conn` to a `connect.Conn` wrapping the server's service and the session's actor before
anything executes. `dial` returns that connection, so no command can open a database, dial a URL or
read a token. The connection is not marked direct, so no call stands as the global admin a local
store grants. The configuration is a defaults-only resolution that reads no file, no `.env` and no
environment of the server process; the environment the tree sees is empty, not the server's.

### The allowlist comes from the registry, enforced on the tree

`capability.SSHCommands()` lists the CLI path of every operation that binds the CLI and carries no SSH
exemption. `RunHosted` prunes every command from the fresh tree that is neither on that list nor an
ancestor of an entry, disables cobra's `help` and `completion` commands, and refuses any invocation
whose first word is not a top-level command of the pruned tree. What cobra resolves is then checked
again: the resolved command path must be on the list, or be a group that only prints its help. A
command the registry does not bind (`config`, `ctx`, `serve`, `init`, `tui`, `mcp`, `docs`,
`completion`, `update`, `version`, `doctor`, `demo`, `ssh`) is not in the pruned tree at all.

### Local-only flags are refused after parsing

`--db`, `--server`, `--token`, `--ctx`, `--config`, `--tenant`, `--no-discovery`, `--allow-network-fs`
and every `--log-*` flag are hidden in a hosted tree, and a persistent pre-run refuses the invocation
when any of them was set. The check reads cobra's own `Changed`, after parsing, so `--db=x`, a flag
after the positional words and a value after `--` are all judged the way cobra reads them, never by a
second hand-written parser.

### Server files are refused at the point of use

Every place a command opens, reads, writes or stats a path goes through `localFile(cmd, path)`, which
refuses any path other than `-` when the command runs hosted. That covers `--out`, `--mapping`,
`--save-mapping`, `--lines PATH`, `--icon-file`, `--file`, `--mapping-file`, the positional files of
`import tasks`, `bundle import` and `artifact put`, and anything added later: a test fails when a file
call in `cmd/` has no `localFile` guard in the same function and its file is not on the short list of
commands that are never hosted.

### Second factor

The owed rule is the service's `TOTPStatus`, as for a board session: owed when the tenant requires a
factor for SSH or the person holds one. The command and the call run with source `cli`, so the
service's sign-in surface is decided from the via record instead: any call that arrived over the SSH
listener is on the `ssh` surface. Without that, `user totp rm` over SSH would test the CLI
requirement.

A command session with a terminal is asked at the same prompt a board session uses. Without one, it
may send `TIX_CODE` as an SSH environment variable (`ssh -o SetEnv=TIX_CODE=123456 host ...`), which
is checked once; otherwise it is refused before the command is read. Enrolment needs a terminal.

### Words and routing

`sess.Command()` is the client's command line split by shell rules on the server; nothing executes it.
The first word picks the path: a registered interface name, compared without regard to case, opens the
interface; the first word of a hosted command, compared exactly, runs the command; a single other word
is a project key. Interface names win, so `watch` stays the event stream and `board KEY` still opens a
project spelled like anything. A project keyed `TASK` is opened with `ssh host TASK` or
`ssh host board task`.

## Security model

- Identity: the key's actor, resolved by the existing lookup, in the tenant its username selects. The
  actor carries no scopes of its own; authority is the membership's role and the user's global admin
  standing, decided by the service on every call.
- No shell, no exec, no environment: words go to an in-process command tree; the server's environment
  and configuration are not visible to it.
- Allowlist: the registry decides which commands exist; the pruned tree is the enforcement.
- Cross-tenant: `--tenant` is refused. Commands that name another tenant by key (`tenant show`,
  `tenant user ls`) are judged by the service, which requires global admin standing for them.
- Server files: no path but `-` is opened.
- Argument shape: at most 128 words, 4 KiB each and 32 KiB in all; a word carrying invalid UTF-8 or a
  control character other than tab and newline is refused.
- Limits: a command holds a session slot like any session, counts against the connection rate limit,
  is registered as a live connection an administrator can end, reads at most
  `server.max_import_bytes` of standard input and is cancelled after `sshd.DefaultCommandTimeout`.
- Audit: calls carry `via=ssh` with the key fingerprint and the client address.
- Stalls: exports spool before writing, so a slow reader holds no store connection; a tenant runs two
  exports at most; a command whose output nobody reads for two minutes is stopped.
- Standing: a running command's key is resolved again every 30 seconds, without recording a use, and
  the command stops with exit 5 when actor, tenant or role changed.
- Channels: per-channel state (actor, registration, watchdog) lives in a value owned by the channel,
  never on the connection's shared context, and the watchdog ends with the channel.
- Panics: the command runs under a recover, and every goroutine a command starts goes through
  `goGuarded`, which a test enforces.
- Global admins: a global admin's key reaches cross-tenant administration, gated by the key and the
  second factor where one is owed. That is the same standing the person has elsewhere.

## Risks and trade-offs

- The connection rate limit (60 an hour by default per address) applies to every command, so a busy
  script needs `ssh.rate_per_hour` raised. It is kept: it is what stops key guessing.
- Interactive confirmations that need a terminal see one only with `ssh -t`; scripts pass the flags
  that skip them.
- The command timeout is fixed for now rather than a configuration key.
