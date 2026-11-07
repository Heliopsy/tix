# Design

## The channel is set by the transport, never by the request

`core.WithVia` is called only where a request enters: the HTTP middleware (`withForwarded` sets
the path-based base, `authenticate` overrides it from the credential), `connect.Conn.Context` for
the CLI, `sshd.sessionCtx` for terminal sessions, `server.systemContext` for background jobs and
the sandbox seed. `service.RunSync`, `ImportFrom` and `ImportBundle` derive `sync` or `import`
from whatever channel called them, keeping it as `origin`. The mutation reads the channel once,
next to `Source`, and stamps it on every audit entry and event it queues, so a call site cannot
forget it.

The credential decides: a bearer token is `api`, a session cookie is `web`, neither falls back to
the path (`/api/` is `api`, anything else `web`). The `X-Tix-Client` header refines only `api`,
and only to `cli-remote` or `mcp`. `Via.Refine` is the one function that reads it, and it returns
any other kind unchanged.

## Nothing secret is stored

The token is recorded by ID and name, the values `auth.TokenVerifier` already resolved. The
session is `auth.SessionRef`: `s-` and 16 hex characters of a SHA-256 over the stored hash, so it
tells sessions apart without being the cookie or the hash a lookup accepts. The user agent is cut
to 160 bytes.

## Columns and JSON

`via_kind` and `via_token` are columns because the feed and `tix audit ls` filter on them, and an
index `(tenant_id, via_kind, seq)` serves the filter. The rest is JSON text in `via_detail`, kept
TEXT on PostgreSQL too since nothing queries inside it. `via_kind` defaults to `unknown`, which is
what every row written before 0033 reads as.

## Events carry less

Events reach webhooks and every subscriber with `events:subscribe`. They carry `Via.Public()`:
kind, origin, token ID and name, job. The network details need `audit:read`.

## Host details are opt-in

A local CLI could record the host name and OS user cheaply, but every member with `audit:read`
would then see which machine somebody worked from. `cli.record_host` turns it on; it is off by
default.

## Surface vs channel

`Source` stays as it was; existing filters and links keep working. The web feed's surface dropdown
is replaced by the channel dropdown, which says the same thing more precisely; a `source=` link
still filters and still shows its chip.
