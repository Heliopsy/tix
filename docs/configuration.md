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
| `server.url` | `TIX_SERVER_URL` | (unset) |
| `server.listen` | `TIX_SERVER_LISTEN` | `127.0.0.1:8080` |
| `server.token` | `TIX_SERVER_TOKEN` | (unset) |
| `auth.mode` | `TIX_AUTH_MODE` | `token` (`none`, `token`, `oidc`) |
| `hooks.mode` | `TIX_HOOKS_MODE` | `off` (`off`, `warn`, `enforce`) |
| `discovery.enabled` | `TIX_DISCOVERY_ENABLED` | `true` |
| `discovery.filenames` | `TIX_DISCOVERY_FILENAMES` | `.tix.yaml,.tix/config.yaml` |
| `retention.audit` | `TIX_RETENTION_AUDIT` | `8760h` |
| `retention.events` | `TIX_RETENTION_EVENTS` | `720h` |
| `log.level` | `TIX_LOG_LEVEL` | `info` (`debug`, `info`, `warn`, `error`) |
| `output.format` | `TIX_OUTPUT_FORMAT` | `table` (`table`, `json`, `yaml`, `ndjson`) |

List values are comma-separated. Durations use Go syntax (`15m`, `24h`, `720h`).

`TIX_TOKEN` is separate from the generated names: it holds the personal access token a command authenticates
with, and is equivalent to `--token`.

## Global flags

These apply to every command:

| Flag | Effect |
| --- | --- |
| `--config` | configuration file to use |
| `--ctx` | named context to use |
| `--db` | database DSN, instead of the configured target |
| `--server` | server URL, instead of the configured target |
| `--token` | API token to authenticate with |
| `-o, --output` | `table`, `json`, `yaml` or `ndjson` |
| `-q, --quiet` | suppress diagnostics |
| `-v, --verbose` | report how the target was resolved |
| `--no-discovery` | ignore per-directory context files |

`--db` and `--server` are mutually exclusive in effect: one names a local database, the other a remote server.

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

retention:
  audit: 8760h
  events: 720h

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
version        ok   dev
configuration  ok   no configuration file; using defaults
target         ok   local /home/you/.local/share/tix/tix.db (from config)
schema         ok   version 2
identity       ok   local
```

Exit 1 when a check fails.
