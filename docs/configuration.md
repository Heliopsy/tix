# Configuration

tix runs with no configuration file. After `tix init`, `tix task add "buy milk"` works on a machine that has
never seen tix: it creates the database, the tenant and the default project on first use. Without `tix init`,
or a server or database named some other way, a first command asks on a terminal and refuses elsewhere rather
than create a local database nobody asked for; see [Where commands go](#where-commands-go).

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

Secrets are redacted in that output. `tix whoami` is the quicker answer when the question is only "which server
or database am I talking to": it names the server URL or the local database path, the context that chose it,
the tenant, the user, and whether a token is presented and where it came from. `-v` on any command reports how
the target was resolved.

## Keys

Every key has a generated `TIX_*` variable: uppercase the path, replace `.` and `-` with `_`, prefix `TIX_`.

| Key | Environment variable | Default |
| --- | --- | --- |
| `target` | `TIX_TARGET` | `auto` (`auto`, `local`) |
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
| `server.max_import_bytes` | `TIX_SERVER_MAX_IMPORT_BYTES` | `33554432` (32 MiB) |
| `auth.mode` | `TIX_AUTH_MODE` | `token` |
| `auth.totp_key` | `TIX_AUTH_TOTP_KEY` | (unset) |
| `hooks.mode` | `TIX_HOOKS_MODE` | `off` |
| `webhooks.drain_mode` | `TIX_WEBHOOKS_DRAIN_MODE` | `inline` (`inline`, `server`, `off`) |
| `webhooks.allow_private_targets` | `TIX_WEBHOOKS_ALLOW_PRIVATE_TARGETS` | `false` |
| `sync.file_root` | `TIX_SYNC_FILE_ROOT` | (unset: `tix serve` reads no file a tenant names) |
| `sync.env_sources` | `TIX_SYNC_ENV_SOURCES` | (unset: `tix serve` binds no source to `TIX_SYNC_<SOURCE>_*`) |
| `discovery.enabled` | `TIX_DISCOVERY_ENABLED` | `true` |
| `discovery.filenames` | `TIX_DISCOVERY_FILENAMES` | `.tix.yaml,.tix/config.yaml` |
| `lease.default` | `TIX_LEASE_DEFAULT` | (unset: `30m`) |
| `lease.max` | `TIX_LEASE_MAX` | (unset: `24h`) |
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
| `output.time_format` | `TIX_OUTPUT_TIME_FORMAT` | `iso` (`iso`, `rfc3339`, `short`, `long`, `dotted`, `dayfirst`, `us`, `relative`) |
| `output.timezone` | `TIX_OUTPUT_TIMEZONE` | `local` (`local`, `utc`, or an IANA name) |
| `cli.page_size` | `TIX_CLI_PAGE_SIZE` | `25` |
| `cli.target_notice` | `TIX_CLI_TARGET_NOTICE` | `true` |
| `cli.record_host` | `TIX_CLI_RECORD_HOST` | `false` |
| `web.page_size` | `TIX_WEB_PAGE_SIZE` | `25` |
| `ssh.listen` | `TIX_SSH_LISTEN` | `127.0.0.1:2222` |
| `ssh.host_key` | `TIX_SSH_HOST_KEY` | (unset: beside the database) |
| `ssh.allow_public` | `TIX_SSH_ALLOW_PUBLIC` | `false` |
| `ssh.public_address` | `TIX_SSH_PUBLIC_ADDRESS` | (unset: the browser's host and the `--ssh-listen` port) |
| `ssh.trusted_proxies` | `TIX_SSH_TRUSTED_PROXIES` | (unset) |
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
| `tui.motion` | `TIX_TUI_MOTION` | `on` (`on`, `off`) |

List values are comma-separated. Durations use Go syntax (`15m`, `24h`, `720h`) plus a day unit
(`30d`, `1d 12h`), so anything the product prints can be typed back.

`tui.keymap` picks the terminal interface's keybinding scheme: `default`, `vim`, `emacs`, `nano` or
`helix`. `tix tui --keys` takes the same names for a single run, and the TUI's settings view lists them
with what each one rebinds. A scheme only moves the actions it names; everything else keeps the shipped
keys, so no scheme can leave an action unreachable. There is no `mac` scheme: a terminal never receives
the command key, and the ctrl chords macOS applies to every text field are the emacs ones, so `emacs`
is the mac scheme.

`tui.motion` decides whether the terminal interface pulses the selected row. It is `on` or `off`
rather than a boolean so a value nobody set reads as unset rather than as off. The pulse changes
emphasis once every 800ms and stops after 45 seconds with no keystroke, resuming on the next one, so
an idle session sends nothing down an SSH connection. A session drawing without colour never pulses
at all: there is no second style it could alternate with.

`cli.page_size` and `web.page_size` are how many rows one page of a listing carries. There are two because
how many rows are worth showing is a property of the thing showing them: a browser window
and a terminal do not hold the same number of lines, and a terminal writing into a pipe is a third case.
Both default to 25.

`cli.page_size` is the default of `--limit` on every listing command (`tix task ls`, `project ls`, `actor ls`,
`user ls`, `tenant ls`, `audit ls`, `webhook deliveries`). Typing `--limit` wins, as the layer order says;
without it the configured value is used, so `TIX_CLI_PAGE_SIZE=100 tix task ls` prints a hundred rows.

`cli.record_host` adds this machine's host name and operating-system user to the access channel of every
change a command makes against a database directly (`--db`, the `cli-local` channel). It is off by default:
every member who can read the audit log would see which machine somebody worked from, which is more than
most teams want to share. Changes through a server never record it. See [audit-log.md](audit-log.md).

`web.page_size` is what a browser listing shows, and `tix serve` reads it. A reader can ask one page for a
different number with the `limit` query parameter, `/tasks?limit=100`, which the Previous and Next links
carry forward for the rest of that walk; a `limit` that is not a usable number is ignored rather than
failing the screen.

Both must be between 1 and 500, which is `core.MaxPageLimit`, the most any listing will return. An
out-of-range value is refused at startup with the key and the layer named, not clamped: a zero reads as
"show nothing", and silently turning it into a screenful would leave the operator believing the key does
something it does not. There is no `tui.page_size`: the terminal interface does not page, it reads the
listing and scrolls it.

`database.connect_timeout` bounds the reachability check a PostgreSQL target makes before the process will
serve anything. The default, `15s`, is what the engine waited before the wait was configurable, so an
installation that sets nothing behaves exactly as it did. Raise it when the database starts beside this
process and is still coming up, which is the ordinary shape of a compose file or a pod: the alternative is a
process that refuses to start against a database that is merely busy. Lower it when something above restarts
this process and a fast failure is worth more than a slow success. It must be positive; zero would mean "give
up immediately", not "wait forever". SQLite ignores it, having no connection to wait for.

### Where commands go

With nothing configured, a command has no target. tix used to create a database at
`~/.local/share/tix/tix.db` (or under `$XDG_DATA_HOME`) and carry on, which is how a person who meant to work
on a server spent a week filling a database on their own machine. It no longer does that by itself:

- On a terminal it asks whether to create the local database or connect to a server. Choosing the server
  writes nothing and prints the command to run.
- Anywhere else, a script or an agent, the command fails with exit `6` and names both ways forward:
  `tix ctx add NAME --server URL --token TOKEN --use` for a server, `tix init` for a local database.

Any one of these is the request for a local database, and with it every command behaves as before:

- a database already at the default path, so an existing install keeps working untouched;
- `target: local` in the configuration file, or `TIX_TARGET=local`, which `tix init` writes for you;
- `--db PATH` or `TIX_DATABASE_DSN`, which name the database outright;
- a context that names a database.

`target` changes nothing else. A server URL from any layer still wins over it.

While a command uses that default database, it prints one line on standard error naming it:

```text
using local database /home/you/.local/share/tix/tix.db (no server configured; run `tix ctx add` to use a server)
```

It is never on standard output, so `-o json` stays parseable, and `--quiet` does not hide it, because it is
the line that says where the work went. Set `cli.target_notice: false` (or `TIX_CLI_TARGET_NOTICE=false`) to
drop it. A target you named yourself, with `--db`, a context or `database.dsn`, prints nothing.

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

The shipped values bound the worst case at eight files of 100 MiB, so 800 MiB. The defaults are sized so that no
incident is lost to rotation and no reasonable disk is filled by a server nobody is watching.

Rotation is not limited to `tix serve`. The destination is a property of the process, so a command that logs
honours it wherever it runs. The file is opened only by a command that actually builds a logger, which today is
`tix serve` and `tix ssh`, so an ordinary `tix task add` never creates one.

The directory is created if it is missing, and the file is written `0600`: a record carries tenant keys and
request paths. A path tix cannot open is refused at startup rather than at the first record, because a server
that started and then logged nothing is the failure that is hardest to notice.

### Leases

`lease.default` and `lease.max` bound how long a claim on a task may hold before it expires and the task returns
to the queue. They are deployment-wide, and both are unset by default, which means the built-in `30m` and `24h`.

A lease lifetime is decided by the first of these that has an opinion:

1. the `ttl` on the claim itself (`tix claim next --ttl 45m`, or `"ttl"` in the API body)
2. the `default_lease` of the workflow governing the task's project, if somebody authored that workflow
3. the tenant's own `lease_default` (`tix tenant edit acme --lease-default 45m`)
4. `lease.default` here
5. the built-in `30m`

The builtin workflow is the exception in step 2, and every tenant starts with one. The shipped `default` workflow
carries a 30m `default_lease` that nobody chose, and counting it would mask the tenant setting on every project of
an ordinary installation: you would set `--lease-default` and watch it do nothing. So a workflow speaks for its
project only once a person has spoken through it. Editing the builtin with `tix workflow put` clears its builtin
flag, and from that point its lease is a choice and outranks the tenant's again. An unconfigured installation
still gets 30m, because that is what `lease.default` falls back to.

The maximum works the other way round. It is a clamp rather than an override, and the effective ceiling is the
lowest of the tenant's `lease_max`, `lease.max` here, and the built-in `24h`. A tenant can therefore tighten the
operator's ceiling and never raise it, which is what makes this setting worth having on a deployment where
tenants are administered by people other than whoever runs the machine.

The two bounds also fail differently when a lifetime exceeds the ceiling. A caller who named the figure is told:
`tix claim next --ttl 48h` is refused with "lease ttl 48h0m0s exceeds the maximum of 24h0m0s", because the error
is the useful answer to a request somebody typed. A *default* that exceeds the ceiling is clamped down to it
instead. Nobody typed that figure: it is what happens when an operator lowers `lease.max` below a default
some tenant configured months ago, and refusing every claim on that tenant until
somebody noticed would be a worse answer than shortening its leases.

Setting a tenant's default longer than its own maximum is refused at the point of setting, where both figures
are in front of you. `tix tenant edit acme --lease-default ""` clears the tenant's setting and returns that
bound to this file.

### Retention

`retention.audit`, `retention.events` and `retention.webhook_deliveries` are the windows history is kept for, and
each is governed independently, so shortening one never disturbs another. They are the defaults a tenant that has
set no policy of its own is pruned by; `tix retention show` reports the effective windows, and `tix retention set`
overrides them for one tenant. Pruning runs from `tix prune`, or from the background pruner inside `tix serve`.

The audit window defaults to a year and the other two to thirty days, because an audit entry is the compliance
record while an event is a transport buffer and a delivery is a receipt. A negative window is refused. A window of
`0` does not mean "keep forever": it means "unset", and the shipped default for that class applies. Say `87600h`
rather than `0` if you mean ten years.

### Housekeeping

Housekeeping tidies tasks on its own, and is per tenant rather than per deployment, so it has no configuration
key: a tenant admin sets it with `tix cleanup set`, the Housekeeping section of the Tenant screen, or
`PUT /api/v1/housekeeping`. It has three rules, all off until set:

| Rule | After N days |
| --- | --- |
| `done` | a task finished in a terminal done-category state goes to the trash |
| `cancelled` | a task finished in a terminal cancelled-category state goes to the trash |
| `trash` | a task in the trash since longer ago is deleted for good, its trashed subtasks first |

The tenant's rules are the default for every project. A project overrides any of them from the Housekeeping
section of its settings screen or `tix cleanup set --project KEY`, with `inherit`, `off` or a number of days.
On either screen the days field is drawn beside a rule only while it is set to After a number of days:

```sh
tix cleanup set --done 30 --cancelled 7 --trash 90   # the tenant default
tix cleanup set --project OPS --done off             # OPS keeps its done tasks
tix cleanup preview --project OPS                    # what would go now, changing nothing
tix cleanup show --project OPS                       # tenant, override and effective rule side by side
```

Tasks have no archived state, so a finished task leaves the board by going to the trash, where the Trash
listing and `tix task restore` bring it back. That is also why the trash setting matters here: with the
trash off a delete is permanent, so turning on `done` or `cancelled` is refused (exit 6, `trash_off`), and
if the trash is turned off afterwards every rule pauses, the trash rule included, until it is on again.
`show` and `preview` say so, and so do both screens. A done task with open subtasks stays until they are
finished and gone too.

`tix serve` applies the rules after every retention prune (`tix serve --prune-interval`, hourly by default) as
the system actor, at most 200 tasks per rule per tick; `tix cleanup run` does one pass now, and
`tix cleanup run --project KEY` or Clean up now in a project's Housekeeping settings one pass on that project.
Every task moved or purged writes the same `task.delete` audit entry and `task.deleted` event a delete by
hand does, with `cleanup` and `after_days` in the payload, so Activity shows it and nobody's inbox is
notified. Each statement re-checks the row it acts on, so two servers ticking at once never act on a task
twice.

`server.listen` is the address `tix serve` binds. `--listen` is its flag layer, and a flag nobody typed is not
a layer: it leaves the configured address alone even though it declares `127.0.0.1:8080` of its own. That is
what lets a container be pointed at an address with `TIX_SERVER_LISTEN` and no command line. It says nothing
about any other command; nothing else binds a port from it.

`server.token` is the credential a command presents. `--token` and `TIX_TOKEN` are its flag layer, so an
explicit one still wins, and below them come `TIX_SERVER_TOKEN`, a `.env` entry, the `token` of the current
context and a `server.token` in the configuration file, in that order. It is redacted wherever a value is
displayed.

`server.trusted_proxies` lists the reverse proxies, as IPs or CIDR blocks, whose `X-Forwarded-Proto` and
`X-Forwarded-For` are believed. Any client can send those headers, so an empty list, the default, believes
neither from anybody, and a request is attributed to the address that opened the connection. When the immediate
peer is on the list, the effective scheme comes from `X-Forwarded-Proto` and the client address is the rightmost
`X-Forwarded-For` entry that is not itself a listed proxy. An address that does not parse is refused at startup.

`server.cookie_security` decides the `Secure` flag on the session and CSRF cookies. `auto` follows the effective
scheme of each request, so a deployment behind a TLS-terminating proxy gets `Secure` once that proxy is listed in
`server.trusted_proxies`. `always` sets it unconditionally; `never` never sets it, which is only correct on a
network that is deliberately plaintext. Serving TLS from tix itself sets it regardless of this key.

`server.max_import_bytes` caps the body of a project task import, `POST /v1/projects/{ref}/tasks/import` and the
upload step of the browser's import, in bytes. Every other request answers to `tix serve --max-body-bytes`, 1 MiB
by default, which is sized for a form rather than a file. A larger import is refused with 413 and a message naming
both the limit and this key. The import also has limits of its own, 64 MiB among them, so raising this key past
that gains nothing. `tix serve` reads it and has no flag for it.

`auth.mode` and `hooks.mode` each accept one value in this build. `none` and `oidc` for `auth.mode`, and `warn`
and `enforce` for `hooks.mode`, are refused with "is not implemented by this build" rather than accepted and
ignored, because a setting that reads as a security control must never be silent.

`hooks.mode` and `webhooks.drain_mode` are unrelated, despite the similar names. `hooks.mode` is the git hook
setting. `webhooks.drain_mode` decides which process delivers queued webhook events: `inline` drains them in the
command that produced them, `server` leaves them for `tix serve`, and `off` queues them and delivers nothing.

### The SSH listener

The `ssh.*` keys configure `tix ssh`, which serves the terminal interface over SSH. Every one of them except
`ssh.trusted_proxies` is also a flag on the command, and the flag wins wherever one was given, so
`TIX_SSH_MAX_TENANTS=50 tix ssh --max-tenants 10` admits ten. A flag nobody typed does not count as a layer: it
leaves the configured value alone even though the flag has a default of its own. The deployment most likely to
run this listener is a container, where a command line is the hardest layer to reach and an environment variable
the easiest, which is why none of this is flag-only. See [deployment.md](deployment.md) for what each setting protects.

`ssh.demo` picks the mode. It is `false`, so the listener serves only the keys enrolled with `tix user key add`
and refuses everything else. Setting it to `true` is `--demo`: any key is accepted and handed a seeded
ephemeral tenant, and the listener then refuses the zero-configuration store and needs a database of its own.
This default flipped: the sandbox used to be unconditional. `ssh.tenant_ttl`, `ssh.reap_interval`,
`ssh.max_tenants`, `ssh.max_tasks` and `ssh.lease_ttl` shape that sandbox and do nothing while `ssh.demo` is
false.

`ssh.trusted_proxies` lists the L4 proxies, as IPs or CIDR blocks, whose PROXY protocol header the listener
believes. There are no headers in an SSH stream, so a proxy that terminates TCP in front of this listener can only
name the real client that way, and without it every connection appears to come from the proxy: one rate limit
bucket for the whole internet, one address in every session record and every log line. Any peer can prepend such a
header, so the list is default-deny. Empty, which is the default, means no header is read from anybody, nothing is
consumed from any stream, and the client address is the one the transport reports, as before this key existed.
When the peer that opened the connection is on the list, its header is read and the address it names becomes the
client address every reader sees, including the rate limiter. A listed peer that does not begin with a valid
header is closed rather than accepted as itself, and so is one that sends nothing before the listener's idle
timeout; a `PROXY UNKNOWN` line or a v2 `LOCAL` command keeps the transport address, because the proxy has said it
is speaking for itself. Both protocol versions are accepted. An entry that is neither an IP nor a CIDR block is
refused at startup.

This is a different key from `server.trusted_proxies` and neither implies the other. The HTTP reverse proxy and
the L4 SSH proxy need not be the same host, and a list written for one surface must not arm the other. See
[deployment.md](deployment.md) for a worked HAProxy example.

Of these keys only `ssh.trusted_proxies` and `ssh.public_address` reach `tix serve`, which has no flag for
either. Its four SSH
flags (`--ssh-listen`, `--ssh-host-key`, `--ssh-allow-public`, `--ssh-idle-timeout`) are flags only, so
`TIX_SSH_LISTEN` in a `tix serve` environment binds nothing, and `tix serve` has no demo mode at all.

`ssh.public_address` is where people reach the listener, as a host or `host:port`, when that is not where it
binds. A container that listens on `:2222` and is published as host port 2020 sets it to
`tix.example.com:2020`. Nothing binds it: the browser's [Connect an agent](web-ui.md#connect-an-agent) screen
writes it into the `ssh` commands it shows. Unset, the screen uses the host name the browser reached and the port
`--ssh-listen` binds. A scheme, a path, a user or a port outside 1 to 65535 is refused at startup.

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
`tix watch` line, the terminal interface and the web interface all render every timestamp the same way.

They are the bottom of three levels. A tenant administrator can set a tenant default (the Tenant screen,
`tix tenant edit --time-format dotted --timezone Europe/Sofia`, or `time_format` and `timezone` on
`PATCH /api/v1/tenants/REF`), and each person can set their own (Settings in the browser, `tix user time`,
or `PATCH /api/v1/me/time`). Each level left empty inherits the one below, and the format and the zone
resolve separately, so a person who picks only a zone keeps the tenant's format. A person's choice is stored
with their account, so it follows them to every browser, terminal and tenant. Agents have no preference of
their own and read the tenant's.

```console
$ tix user time
┌────────┬──────────────────────┬──────────────────────────┐
│ LEVEL  │ FORMAT               │ TIMEZONE                 │
├────────┼──────────────────────┼──────────────────────────┤
│ global │ iso                  │ local                    │
│ tenant │ dotted               │ (inherit)                │
│ user   │ (inherit)            │ Asia/Tokyo               │
│ in use │ dotted (from tenant) │ Asia/Tokyo (from user)   │
└────────┴──────────────────────┴──────────────────────────┘
```

On the command line and in the terminal interface, a format or zone set on the machine itself (a flag,
`TIX_OUTPUT_TIME_FORMAT`, `TIX_OUTPUT_TIMEZONE` or the configuration file) wins for that field on that
machine. A field left at its shipped default follows the account. For `tix serve`, these keys are the
deployment default every tenant and person inherits.

An unknown format or an unknown timezone name is refused at load rather than silently falling back, because a
timestamp quietly in the wrong zone is worse than an error at startup.

`output.time_format` picks the layout, shown here for `2026-09-21T14:05:09Z`:

| Name | Renders as |
| --- | --- |
| `iso` (default) | `2026-09-21 14:05` |
| `rfc3339` | `2026-09-21T14:05:09+03:00` |
| `short` | `21 Sep 14:05` |
| `long` | `21 Sep 2026 14:05` |
| `dotted` | `21.09.2026 14:05` |
| `dayfirst` | `21/09/2026 14:05` |
| `us` | `09/21/2026 2:05 PM` |
| `relative` | `18 minutes ago` |

The format also decides how a date typed by a person is read, in the web's due date fields, the quick-add
line on every surface (`due:`) and `tix task add --parse`. `YYYY-MM-DD` and a date with the month as a word
(`6 Nov 2026`) are read under every format. A date written with digits alone is read in the format's own
order: `06.11.2026` and `06/11/2026` are the 6th of November under `dotted` and `dayfirst`, `11/06/2026` is
the 6th of November under `us`, and under any other format such a date is refused rather than guessed.
`--due` on the command line, the HTTP API and the MCP tools take `YYYY-MM-DD` or RFC 3339 only, whatever the
format, because a script must not depend on a person's preference.

A blank field in a table, or an empty value in a `Due` or `Expires` column on the web, means the timestamp was
never set: the zero time renders as nothing rather than as `1970-01-01`, which every reader misreads as a real
date. Every timestamp on the web carries the full instant, to the second with its zone and offset and how long
ago it was (`06.11.2026 14:05:32 EET (UTC+02:00) · 2 hours ago`), on hover, to a screen reader, and on a tap
or keyboard focus, so a reader can always get the exact time.

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
| `TIX_TOKEN` | the personal access token a command authenticates with, equivalent to `--token`, and the flag layer of `server.token` |
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

`tix project create` and `tix project edit` take their own `--color`, which names a palette colour or a custom
`#rrggbb` for the project and shadows the global flag on those two commands.

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

The token a context carries is presented on every command run under it, with nothing exported. `tix ctx list`
shows whether each context has one (the `TOKEN` column, never the value), and `tix whoami` confirms which
server, tenant and user a command will reach:

```text
$ tix whoami
target  server https://tix.internal.example.com (context work)
tenant  acme
user    ada (Ada Lovelace), admin
token   yes, from context work
```

Up to 0.43.0 that held only for some commands. `task ls` and `task add` read the configuration before
connecting and presented the context's token; `task show`, `claim next`, `status` and others connected first
and presented nothing, so a server answered "no credential presented" and exporting `TIX_TOKEN` looked like
the only way in. Every command now resolves the token the same way.

`tix login` does not set up the command line: it prints a browser session, which the API does not accept as a
bearer token. For the command line, mint an API token (`tix token create`, or the tokens screen of the web
interface) and give it to `tix ctx add --token`.

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
below the real environment and above the configuration file, so a developer can keep credentials out of a shell
profile and an explicit `TIX_TOKEN=... tix ...` still wins.

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
