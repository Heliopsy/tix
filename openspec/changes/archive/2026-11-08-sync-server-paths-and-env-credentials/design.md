# Design

## Who the service is serving

`service.Local` cannot tell from a request whether the process answers the operator or a tenant: the
same instance backs the CLI, HTTP, the web and SSH inside `tix serve`. So the decision is made once, at
wiring. `connect.Overrides.Serving` is set by `tix serve` alone, and `connect` passes
`service.WithSyncHost(SyncHost{Trusted: !Serving, ...})`. A `Local` built without the option is shared,
so a new server path that forgets to wire it fails closed.

The `Via` on a request was rejected as the signal because it describes a channel, not a trust level, and
a client hint feeds part of it.

## Files

Stored paths on a shared host are honoured only under `<sync.file_root>/<tenant key>`. Relative paths are
joined to that directory; absolute ones must already be inside it. The save checks the path lexically
only, so it reveals nothing. Each read resolves symbolic links on both the directory and the file, checks
the result with `filepath.Rel`, and opens the resolved name through `os.Root` with `O_NOFOLLOW`, so a link
swapped in after the check still cannot escape. Every failure returns one message.

The service now reads the generic adapter's file itself and hands the bytes to the adapter, so the
adapter never opens a path a tenant chose.

A global admin gets no exception. The setting is stored on the tenant's row, a tenant admin can edit the
source afterwards with `--keep-settings` semantics, and tracking who set each field needs a column. The
file root covers the operator's need without it.

A data upload for generic sources is left for a follow-up: with a file root the operator can place data
for a tenant, and one-off files already go through project import.

## Environment

`sync.env_sources` lists `TENANT/SOURCE` or `TENANT/*`. An unbound source gets an empty environment, so
the resolution shows nothing from it and its missing hints do not name variables. Credentials from the
environment additionally require that the source stores no url, or the same url as
`TIX_SYNC_<SOURCE>_URL`; the check runs in the resolution and again when the adapter config is built.
This url rule holds on a direct command as well, because a database can be shared between a server and
the operator's shell.

## Compatibility

Single-tenant servers that relied on the environment or server paths must set `sync.env_sources:
[default/*]` and move files under `sync.file_root`. Sources already stored with a path outside the
directory are kept but reported missing and refused at run time, with a message saying what to do.
