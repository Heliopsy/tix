# Configuration

tix runs with no configuration file. `tix task add "buy milk"` works on a machine that has never seen tix: it
creates the database, the tenant and the default project on first use.

Everything below is for when the defaults are not what you want.

## The five layers

A value is resolved from the first layer that supplies it:

```text
flags  >  environment  >  .env  >  config file  >  defaults
```

`tix config show --sources` reports the effective value of every key and which layer produced it:

```console
$ tix config show --sources -o json
[
  {"key":"database.dsn","env":"TIX_DATABASE_DSN","value":"sqlite:///home/you/.local/share/tix/tix.db",
   "source":"default","secret":true},
  ...
]
```

Secrets are redacted in that output. `-v` on any command reports how the target was resolved, which is the
quicker answer when the question is only "which database am I talking to".

## Keys

Every key has a generated `TIX_*` variable: uppercase the path, replace `.` and `-` with `_`, prefix `TIX_`.

| Key | Environment variable | Default |
| --- | --- | --- |
| `tenant` | `TIX_TENANT` | `default` |
| `project` | `TIX_PROJECT` | (unset) |
| `current_context` | `TIX_CURRENT_CONTEXT` | (unset) |
| `database.dsn` | `TIX_DATABASE_DSN` | `sqlite://~/.local/share/tix/tix.db` |
| `database.allow_network_fs` | `TIX_DATABASE_ALLOW_NETWORK_FS` | `false` |
| `database.connect_timeout` | `TIX_DATABASE_CONNECT_TIMEOUT` | `15s` |
| `server.url` | `TIX_SERVER_URL` | (unset) |
| `server.listen` | `TIX_SERVER_LISTEN` | `127.0.0.1:8080` |
| `server.token` | `TIX_SERVER_TOKEN` | (unset) |
| `server.trusted_proxies` | `TIX_SERVER_TRUSTED_PROXIES` | (unset) |
| `server.cookie_security` | `TIX_SERVER_COOKIE_SECURITY` | `auto` (`auto`, `always`, `never`) |
| `auth.mode` | `TIX_AUTH_MODE` | `token` |
| `hooks.mode` | `TIX_HOOKS_MODE` | `off` |
| `webhooks.drain_mode` | `TIX_WEBHOOKS_DRAIN_MODE` | `inline` (`inline`, `server`, `off`) |
| `webhooks.allow_private_targets` | `TIX_WEBHOOKS_ALLOW_PRIVATE_TARGETS` | `false` |
| `discovery.enabled` | `TIX_DISCOVERY_ENABLED` | `true` |
| `discovery.filenames` | `TIX_DISCOVERY_FILENAMES` | `.tix.yaml,.tix/config.yaml` |
| `retention.audit` | `TIX_RETENTION_AUDIT` | `8760h` |
| `retention.events` | `TIX_RETENTION_EVENTS` | `720h` |
| `retention.webhook_deliveries` | `TIX_RETENTION_WEBHOOK_DELIVERIES` | `720h` |
| `log.level` | `TIX_LOG_LEVEL` | `info` (`debug`, `info`, `warn`, `error`) |
| `log.format` | `TIX_LOG_FORMAT` | `text` (`text`, `json`) |
| `log.output` | `TIX_LOG_OUTPUT` | `stderr` (`stderr`, `stdout`, or a file path) |
| `log.file.max_size_mb` | `TIX_LOG_FILE_MAX_SIZE_MB` | `100` |
| `log.file.max_age` | `TIX_LOG_FILE_MAX_AGE` | `168h` |
| `log.file.max_backups` | `TIX_LOG_FILE_MAX_BACKUPS` | `7` |
| `log.file.compress` | `TIX_LOG_FILE_COMPRESS` | `false` |
| `output.format` | `TIX_OUTPUT_FORMAT` | `table` (`table`, `json`, `yaml`, `ndjson`) |
| `output.color` | `TIX_OUTPUT_COLOR` | `auto` (`auto`, `always`, `never`) |
| `output.time_format` | `TIX_OUTPUT_TIME_FORMAT` | `iso` (`iso`, `rfc3339`, `short`, `us`, `relative`) |
| `output.timezone` | `TIX_OUTPUT_TIMEZONE` | `local` (`local`, `utc`, or an IANA name) |
| `ssh.listen` | `TIX_SSH_LISTEN` | `127.0.0.1:2222` |
| `ssh.host_key` | `TIX_SSH_HOST_KEY` | (unset: beside the database) |
| `ssh.allow_public` | `TIX_SSH_ALLOW_PUBLIC` | `false` |
| `ssh.demo` | `TIX_SSH_DEMO` | `false` |
| `ssh.tenant_ttl` | `TIX_SSH_TENANT_TTL` | `6h` |
| `ssh.reap_interval` | `TIX_SSH_REAP_INTERVAL` | `10m` |
| `ssh.max_tenants` | `TIX_SSH_MAX_TENANTS` | `200` |
| `ssh.max_tasks` | `TIX_SSH_MAX_TASKS` | `200` |
| `ssh.lease_ttl` | `TIX_SSH_LEASE_TTL` | `2m` |
| `ssh.rate_per_hour` | `TIX_SSH_RATE_PER_HOUR` | `60` |
| `ssh.rate_burst` | `TIX_SSH_RATE_BURST` | `5` |
| `ssh.idle_timeout` | `TIX_SSH_IDLE_TIMEOUT` | `30m` |
| `ssh.keepalive_interval` | `TIX_SSH_KEEPALIVE_INTERVAL` | `30s` |
| `ssh.keepalive_max_missed` | `TIX_SSH_KEEPALIVE_MAX_MISSED` | `3` |
| `ssh.max_sessions_per_key` | `TIX_SSH_MAX_SESSIONS_PER_KEY` | `3` |
| `ssh.max_sessions` | `TIX_SSH_MAX_SESSIONS` | `100` |
| `tui.keymap` | `TIX_TUI_KEYMAP` | `default` |

List values are comma-separated. Durations use Go syntax (`15m`, `24h`, `720h`) plus a day unit
(`30d`, `1d 12h`), so anything the product prints can be typed back.

`tui.keymap` picks the terminal interface's keybinding scheme: `default`, `vim`, `emacs`, `nano` or
`helix`. `tix tui --keys` takes the same names for a single run, and the TUI's settings view lists them
with what each one rebinds. A scheme only moves the actions it names; everything else keeps the shipped
keys, so no scheme can leave an action unreachable. There is no `mac` scheme: a terminal never receives
the command key, and the ctrl chords macOS applies to every text field are the emacs ones, so `emacs`
is the mac scheme.

`database.connect_timeout` bounds the reachability check a PostgreSQL target makes before the process will
serve anything. The default, `15s`, is what the engine waited before the wait was configurable, so an
installation that sets nothing behaves exactly as it did. Raise it when the database starts beside this
process and is still coming up, which is the ordinary shape of a compose file or a pod: the alternative is a
process that refuses to start against a database that is merely busy. Lower it when something above restarts
this process and a fast failure is worth more than a slow success. It must be positive; zero would mean "give
up immediately", not "wait forever". SQLite ignores it, having no connection to wait for.

### Logging

`log.level` and `log.format` decide how much is emitted and in what shape. `text` is what a person reads at a
terminal; `json` is one object per record, for anything that parses the stream.

`log.output` decides where records land, and it is the key the rest hang off. The default, `stderr`, is what a
supervisor already collects, and the `log.file.*` keys then govern nothing. Naming a path instead makes tix the
owner of that file, and the rotation settings are what keeps it bounded:

```yaml
log:
  output: /var/log/tix/tix.log
  format: json
  file:
    max_size_mb: 100
    max_age: 168h
    max_backups: 7
    compress: false
```

A file is rotated once the next record would carry it past `max_size_mb`, to a sibling named after the rotation
time, as in `tix-20260923T041233.417.log`. Archives are then removed once there are more than `max_backups` of
them or once they are older than `max_age`, whichever comes first. Setting either to `0` drops that bound alone,
which is what a deployment shipping its logs elsewhere wants; setting both to `0` keeps every archive forever and
is a deliberate choice rather than something you can arrive at by accident. `compress` gzips an archive once it is
closed, off by default because it spends CPU on the machine that is already busy writing the log.

The shipped values bound the worst case at eight files of 100 MiB, so 800 MiB. That number is the point of the
defaults: large enough that no incident is lost to rotation, small enough that no reasonable disk is filled by a
server nobody is watching.

Rotation is not limited to `tix serve`. The destination is a property of the process, so a command that logs
honours it wherever it runs. The file is opened only by a command that actually builds a logger, which today is
`tix serve` and `tix ssh`, so an ordinary `tix task add` never creates one.

The directory is created if it is missing, and the file is written `0600`: a record carries tenant keys and
request paths. A path tix cannot open is refused at startup rather than at the first record, because a server
that started and then logged nothing is the failure that is hardest to notice.

### Retention

`retention.audit`, `retention.events` and `retention.webhook_deliveries` are the windows history is kept for, and
each is governed independently, so shortening one never disturbs another. They are the defaults a tenant that has
set no policy of its own is pruned by; `tix retention show` reports the effective windows, and `tix retention set`
overrides them for one tenant. Pruning runs from `tix prune`, or from the background pruner inside `tix serve`.

The audit window defaults to a year and the other two to thirty days, because an audit entry is the compliance
record while an event is a transport buffer and a delivery is a receipt. A negative window is refused. A window of
`0` does not mean "keep forever": it means "unset", and the shipped default for that class applies. Say `87600h`
rather than `0` if you mean ten years.

`server.trusted_proxies` lists the reverse proxies, as IPs or CIDR blocks, whose `X-Forwarded-Proto` and
`X-Forwarded-For` are believed. Any client can send those headers, so an empty list, the default, believes
neither from anybody, and a request is attributed to the address that opened the connection. When the immediate
peer is on the list, the effective scheme comes from `X-Forwarded-Proto` and the client address is the rightmost
`X-Forwarded-For` entry that is not itself a listed proxy. An address that does not parse is refused at startup.

`server.cookie_security` decides the `Secure` flag on the session and CSRF cookies. `auto` follows the effective
scheme of each request, so a deployment behind a TLS-terminating proxy gets `Secure` once that proxy is listed in
`server.trusted_proxies`. `always` sets it unconditionally; `never` never sets it, which is only correct on a
network that is deliberately plaintext. Serving TLS from tix itself sets it regardless of this key.

`auth.mode` and `hooks.mode` each accept one value in this build. `none` and `oidc` for `auth.mode`, and `warn`
and `enforce` for `hooks.mode`, are refused with "is not implemented by this build" rather than accepted and
ignored, because a setting that reads as a security control must never be silent.

`hooks.mode` and `webhooks.drain_mode` are unrelated, despite the similar names. `hooks.mode` is the git hook
setting. `webhooks.drain_mode` decides which process delivers queued webhook events: `inline` drains them in the
command that produced them, `server` leaves them for `tix serve`, and `off` queues them and delivers nothing.

### The SSH listener

The `ssh.*` keys configure `tix ssh`, which serves the terminal interface over SSH. Every one of them is also a
flag on the command, and the flag wins wherever one was given, so `TIX_SSH_MAX_TENANTS=50 tix ssh --max-tenants 10`
admits ten. A flag nobody typed does not count as a layer: it leaves the configured value alone even though the
flag has a default of its own. The deployment most likely to run this listener is a container, where a command
line is the hardest layer to reach and an environment variable the easiest, which is why none of this is
flag-only. See [deployment.md](deployment.md) for what each setting protects.

`ssh.demo` picks the mode. It is `false`, so the listener serves only the keys enrolled with `tix user key add`
and refuses everything else. Setting it to `true` is `--demo`: any key is accepted and handed a seeded
ephemeral tenant, and the listener then refuses the zero-configuration store and needs a database of its own.
This default flipped: the sandbox used to be unconditional. `ssh.tenant_ttl`, `ssh.reap_interval`,
`ssh.max_tenants`, `ssh.max_tasks` and `ssh.lease_ttl` shape that sandbox and do nothing while `ssh.demo` is
false.

None of these keys reach `tix serve`. Its four SSH flags (`--ssh-listen`, `--ssh-host-key`,
`--ssh-allow-public`, `--ssh-idle-timeout`) are flags only, so `TIX_SSH_LISTEN` in a `tix serve` environment
binds nothing, and `tix serve` has no demo mode at all.

`ssh.idle_timeout` closes a session nobody is typing at, measured from the last key the interface saw.
`ssh.keepalive_interval` and `ssh.keepalive_max_missed` are a different question: whether the client is still
there at all. They are deliberately separate, and the idle clock is fed by keystrokes rather than by traffic, so
a keepalive cannot hold an abandoned session open. The defaults notice a vanished client in about two minutes
rather than in the thirty the idle timeout would take, because a session whose client has gone still holds its
slot and any lease it was carrying.

`ssh.max_sessions_per_key` and `ssh.max_sessions` cap sessions that are live at once, which `ssh.rate_per_hour`
does not: the rate limit counts connections from one source address over an hour, and says nothing about how
many of them are still open. A refusal names the limit and is worded the same for every key, so it cannot tell a
stranger whether the listener had seen theirs before. A per-key cap above `ssh.max_sessions` is refused at
startup as unreachable.

A negative window or a negative cap is refused at startup rather than accepted and quietly replaced by a default
much later.

### The default project

`project` names the project used when a command does not say. It is the default for `tix task add`, `tix task ls`
and `tix tui`, and `-p`/`--project` on any of them overrides it for that run. Unset, `tix task add` uses the only
project when there is one and is exit 2 when there are several.

### How a timestamp is shown

`output.time_format` and `output.timezone` govern how an instant is shown to a person: the CLI table, a live
`tix watch` line, and the web interface all render every timestamp the same way, through one shared setting.

An unknown format or an unknown timezone name is refused at load rather than silently falling back, because a
timestamp quietly in the wrong zone is worse than an error at startup.

`output.time_format` picks the layout, shown here for `2026-09-21T14:05:09Z`:

| Name | Renders as |
| --- | --- |
| `iso` (default) | `2026-09-21 14:05` |
| `rfc3339` | `2026-09-21T14:05:09+03:00` |
| `short` | `21 Sep 14:05` |
| `us` | `09/21/2026 2:05 PM` |
| `relative` | `18 minutes ago` |

A blank field in a table, or an empty value in a `Due` or `Expires` column on the web, means the timestamp was
never set: the zero time renders as nothing rather than as `1970-01-01`, which every reader misreads as a real
date. A relative timestamp on the web still carries the absolute instant in its `title` attribute (hover it), in
the fixed `iso` layout regardless of the configured format, so a reader can always get the exact time.

`output.timezone` picks the zone a timestamp is converted into before it is laid out:

- `local` (the default) uses the machine's own zone: the CLI's host, or the `tix serve` process for the web.
- `utc` fixes it to UTC regardless of the machine.
- Any IANA zone name (`Asia/Tokyo`, `America/New_York`, `Europe/Sofia`, ...) fixes it to that zone.

The binary embeds the zone database, so a name resolves the same on a distroless or scratch image as on a
machine carrying `/usr/share/zoneinfo`. A deployment does not have to install `tzdata` to honour a zone a
reader or an operator named.

**Machine-readable output is not affected.** `-o json`, `-o yaml` and `-o ndjson` always carry timestamps as RFC
3339 in UTC, whatever `output.time_format` and `output.timezone` are set to, because a script or another program
parsing a timestamp must never have to guess which zone or format a deployment happened to configure.

```console
$ TIX_OUTPUT_TIME_FORMAT=short TIX_OUTPUT_TIMEZONE=Asia/Tokyo tix task ls
┌───────┬─────────────────┬────────┬──────────┬──────────┬──────┬──────────────┬─────────┐
│ REF   │ TITLE           │ STATUS │ PRIORITY │ ASSIGNEE │ TAGS │ UPDATED      │ BLOCKED │
├───────┼─────────────────┼────────┼──────────┼──────────┼──────┼──────────────┼─────────┤
│ ENG-7 │ Wire the output │ doing  │ high     │ act_1    │ cli  │ 21 Sep 23:05 │ no      │
└───────┴─────────────────┴────────┴──────────┴──────────┴──────┴──────────────┴─────────┘
$ TIX_OUTPUT_TIME_FORMAT=short TIX_OUTPUT_TIMEZONE=Asia/Tokyo tix task ls -o json | grep updated_at
  "updated_at": "2026-09-21T14:05:09Z",
```

The web reads the same two keys from the server's configuration: it is one setting for every browser hitting that
`tix serve` process, not a per-user preference. That is deliberate for now, since the configuration keys describe
a server setting, and a tenant's members are usually close enough in time zone for one setting to be fine. A
future per-user version would need somewhere to keep each signed-in actor's preferred format and zone (most
naturally alongside the theme and keyboard-scheme choices the web already remembers per browser, or on the actor
record if it should follow someone to a new device) and would resolve the rendering per request from that instead
of once when the server starts.

### Variables outside the generated scheme

| Variable | Effect |
| --- | --- |
| `TIX_TOKEN` | the personal access token a command authenticates with, equivalent to `--token` |
| `NO_COLOR` | set and non-empty, suppresses colour, per [no-color.org](https://no-color.org) |
| `TIX_NO_COLOR` | the tix-scoped spelling of the same thing |

The two colour variables only apply while the colour mode is `auto`. An explicit `TIX_OUTPUT_COLOR` (or
`output.color`, or `--color`/`--no-color`) beats them, so `NO_COLOR=1 TIX_OUTPUT_COLOR=always tix task ls` is
coloured.

## Global flags

These apply to every command:

| Flag | Effect |
| --- | --- |
| `--config` | configuration file to use |
| `--ctx` | named context to use |
| `--db` | database DSN, instead of the configured target |
| `--server` | server URL, instead of the configured target |
| `--token` | API token to authenticate with |
| `--tenant` | tenant key to work in, instead of the configured one |
| `-o, --output` | `table`, `json`, `yaml` or `ndjson` |
| `--color` | force coloured output, even when not writing to a terminal |
| `--no-color` | disable coloured output |
| `-q, --quiet` | suppress diagnostics |
| `-v, --verbose` | report how the target was resolved |
| `--no-discovery` | ignore per-directory context files |
| `--allow-network-fs` | allow opening a database on a network filesystem, which risks corruption |
| `--log-level` | `debug`, `info`, `warn` or `error` |
| `--log-format` | `text` or `json` |
| `--log-output` | `stderr`, `stdout` or a file path to rotate |
| `--log-max-size-mb` | size one log file may reach before it is rotated |
| `--log-max-age` | how long a rotated log file is kept |
| `--log-max-backups` | how many rotated log files are kept |
| `--log-compress` | gzip a rotated log file |

`--db` and `--server` are mutually exclusive in effect: one names a local database, the other a remote server.
`--color` and `--no-color` together are exit 2.

The seven `--log-*` flags are the flag layer of the `log.*` keys, so they apply to every command for the same
reason the keys do, and only a command that builds a logger acts on them. See [Logging](#logging).

`tix project create` and `tix project edit` take their own `--color`, which names a palette colour for the
project and shadows the global flag on those two commands.

## The configuration file

`$XDG_CONFIG_HOME/tix/config.yaml`, falling back to `~/.config/tix/config.yaml`. `TIX_CONFIG` or `--config`
override the location, and `TIX_CONFIG` naming a file that does not exist is an error rather than a silent
fallback.

```yaml
tenant: acme
project: infra
current_context: work

database:
  dsn: sqlite://~/.local/share/tix/tix.db

output:
  format: table
  color: auto
  time_format: iso
  timezone: local

webhooks:
  drain_mode: inline

retention:
  audit: 8760h
  events: 720h
  webhook_deliveries: 720h

contexts:
  work:
    server: https://tix.internal.example.com
    token: tix_pat_...
    tenant: acme
    project: infra
  local:
    database: sqlite://~/work/project.db
    tenant: default
```

`~` is expanded in paths, including inside a DSN.

## Contexts

A context is a named bundle of target and identity: which database or server, which tenant, which project, which
token.

```sh
tix ctx add work --server https://tix.internal.example.com --token "$TOKEN" --tenant acme --use
tix ctx add local --db sqlite://~/work/project.db --project infra
tix ctx list
tix ctx show work
tix ctx use local
tix ctx rm work
```

| Flag on `ctx add` | Effect |
| --- | --- |
| `--db` | database DSN the context points at |
| `--server` | server URL the context points at |
| `--token` | API token the context authenticates with |
| `--tenant` | tenant key the context selects |
| `--project` | default project key |
| `--use` | also make the context current |

A context may name a database or a server, not both; giving both is exit 2.

Select one for a single command with `--ctx work`, or make it the default with `tix ctx use work`, which sets
`current_context`.

## Per-directory discovery

When `discovery.enabled` is true, tix walks up from the working directory looking for the first of
`discovery.filenames` (`.tix.yaml`, then `.tix/config.yaml`), stopping at the directory holding `.git`. That file
sits in the `file` layer alongside the global configuration, so a repository can pin the project or the database
its tasks live in:

```yaml
# .tix.yaml at the repository root
project: infra
database:
  dsn: sqlite://./.tix/tasks.db
```

Everyone working in that tree gets the right project without arranging anything. Discovery stops at the
repository root, so a stray `.tix.yaml` in a parent directory outside the repository cannot reach in.

`--no-discovery`, or `TIX_DISCOVERY_ENABLED=false`, turns it off. Discovery is also skipped when a higher layer
has already disabled it, so an environment variable in CI is enough to make a build reproducible regardless of
what is checked in.

## .env

The nearest `.env` at or above the working directory, found the same way, supplies `TIX_*` variables. It sits
below the real environment and above the configuration file, which is the ordering that lets a developer keep
credentials out of a shell profile while still allowing an explicit `TIX_TOKEN=... tix ...` to win.

## Checking the result

```sh
tix doctor
```

```text
┌───────────────┬────────┬───────────────────────────────────────────────────────────┐
│ NAME          │ STATUS │ DETAIL                                                    │
├───────────────┼────────┼───────────────────────────────────────────────────────────┤
│ version       │ ok     │ dev                                                       │
│ configuration │ ok     │ no configuration file; using defaults                     │
│ target        │ ok     │ local /home/you/.local/share/tix/tix.db (from config)     │
│ schema        │ ok     │ version 3                                                 │
│ identity      │ ok     │ local                                                     │
└───────────────┴────────┴───────────────────────────────────────────────────────────┘
```

`-o json` gives the same checks as records, which is the form a health script should read.

Exit 1 when a check fails.
