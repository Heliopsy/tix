# Sync server paths and environment credentials

## Why

On a server that hosts several tenants, two parts of external sync trusted input from a tenant admin
more than they should.

- A source's `file` setting and its `mapping_path` were read from the server's filesystem as given. Only
  `..` was refused, so an absolute path reached any file the server process could read, and a run
  imported its content as tasks. Refusals also said whether a path existed and could quote a parse
  error from its first bytes.
- `TIX_SYNC_<SOURCE>_TOKEN`, `_USER` and `_PASSWORD` were keyed by source name alone. Any tenant that
  named a source the same as the operator's got the operator's credentials, sent to whatever url the
  tenant stored. The resolution output also listed which of those variables were set.

## What Changes

- **BREAKING** `tix serve` reads no file a tenant names unless the operator sets `sync.file_root`
  (`TIX_SYNC_FILE_ROOT`). A stored `file` or `mapping_path` must then sit in `<file_root>/<tenant key>/`,
  checked lexically when saved and again on every read with symbolic links resolved and an `O_NOFOLLOW`
  open through `os.Root`. Every refused read answers the same way.
- **BREAKING** On `tix serve` the `TIX_SYNC_<SOURCE>_*` environment applies only to the sources the
  operator binds in `sync.env_sources` (`TIX_SYNC_ENV_SOURCES`), as `TENANT/SOURCE` or `TENANT/*`. An
  unbound source neither uses those variables nor learns whether any is set.
- **BREAKING** Credentials from the environment are sent only to the url the environment names. A
  source that stores a different url is told to store its own credentials.
- A command run against the database directly is the operator's own and keeps reading their files and
  environment as before, apart from the url rule.
- The web dialog, CLI help, docs and the skill say where files may live and what the environment covers.

## Capabilities

### Modified Capabilities

- `external-sync`: server paths are confined per tenant on a shared server, the environment is bound per
  tenant and source, and environment credentials go only to the environment's url.

## Impact

- `internal/service` (sync resolution, config, scope), `internal/sync` (confined reads, bindings, generic
  data), `internal/config` (`sync.file_root`, `sync.env_sources`), `internal/connect`, `cmd/serve.go`.
- Operators who ran sync through `tix serve` with server files or `TIX_SYNC_*` variables need to set the
  new keys; see the migration note in `docs/import-export.md`.
