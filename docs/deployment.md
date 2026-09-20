# Deployment

`tix serve` runs the HTTP API, the WebSocket event stream and the web UI from one process, over the database it
was pointed at.

```sh
tix serve --listen 127.0.0.1:8080
```

A CLI on the same machine can keep talking to the database directly at the same time. Both paths run the same
service code, so a direct write is visible to a subscriber on the server immediately.

## Flags

| Flag | Default | Effect |
| --- | --- | --- |
| `--listen` | `127.0.0.1:8080` | address to bind |
| `--tls-cert`, `--tls-key` | (none) | serve HTTPS |
| `--insecure-no-tls` | off | allow a non-loopback bind without TLS |
| `--max-body-bytes` | 1 MiB | request body cap |
| `--request-timeout` | 30s | per-request timeout |
| `--shutdown-timeout` | | how long shutdown waits for in-flight requests |
| `--sweep-interval` | `1m` | how often expired leases are swept |
| `--no-lease-sweeper` | off | disable the sweeper |
| `--prune-interval` | `1h` | how often retention pruning runs |
| `--no-retention-pruner` | off | disable the pruner |

`--listen` also reads from `server.listen` / `TIX_SERVER_LISTEN`.

## The non-loopback bind guard

Binding anything other than loopback without TLS is refused:

```console
$ tix serve --listen 0.0.0.0:8080
error: invalid: refusing to bind non-loopback address "0.0.0.0:8080" without tls: configure a certificate and key,
or pass the explicit insecure opt-out
```

That is exit 2, before the socket is opened. The point is that bearer tokens and session cookies do not survive a
plaintext hop, and the failure mode of getting it wrong is silent.

Three ways forward, in order of preference:

**Terminate TLS in tix.**

```sh
tix serve --listen 0.0.0.0:8443 --tls-cert /etc/tix/tls.crt --tls-key /etc/tix/tls.key
```

**Keep tix on loopback and put a proxy in front.** The guard does not fire, because the bind is still loopback:

```sh
tix serve --listen 127.0.0.1:8080
```

**Opt out explicitly.** `--insecure-no-tls` allows the bind and logs a warning on every start. Use it only where
the network itself is the boundary, such as a pod whose port is reachable only from a sidecar.

## Behind a reverse proxy

Preserve the `Host` header. The tenant is resolved from it, so a proxy that rewrites `Host` sends every request
to the default tenant regardless of which hostname the client asked for. See [tenancy.md](tenancy.md).

nginx:

```nginx
server {
  listen 443 ssl;
  server_name tix.example.com;

  ssl_certificate     /etc/letsencrypt/live/tix.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/tix.example.com/privkey.pem;

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host              $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;

    # the event stream is a long-lived WebSocket
    proxy_http_version 1.1;
    proxy_set_header Upgrade    $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
  }
}
```

Caddy:

```caddyfile
tix.example.com {
  reverse_proxy 127.0.0.1:8080
}
```

Two things break the event stream if you get them wrong: a proxy that does not forward `Upgrade` and
`Connection`, and a read timeout shorter than the quiet period between events. The server pings every 30 seconds,
so a read timeout above a minute is enough.

## Health checks

| Route | Use |
| --- | --- |
| `GET /healthz` | liveness; the process is up |
| `GET /readyz` | readiness; the database answers and migrations are applied |

Neither needs a credential.

## Background workers

The server runs two tickers:

- the **lease sweeper**, which expires leases past their deadline and reverts their tasks where the workflow says
  to. Without it, a task held by a dead worker stays held. Disable it only if something else runs
  `tix claim sweep`.
- the **retention pruner**, which removes events and audit entries past `retention.events` and `retention.audit`.
  Disable it only if something else runs `tix prune`.

Run exactly one process with these enabled against a given database.

## systemd

```ini
[Unit]
Description=tix
After=network-online.target

[Service]
Type=simple
User=tix
Environment=TIX_DATABASE_DSN=sqlite:///var/lib/tix/tix.db
ExecStart=/usr/local/bin/tix serve --listen 127.0.0.1:8080
Restart=on-failure

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/tix

[Install]
WantedBy=multi-user.target
```

`--shutdown-timeout` bounds how long a stop waits for in-flight requests, so keep systemd's `TimeoutStopSec`
above it.

## Containers

`Containerfile` at the repository root builds a runtime image, and `just image` produces it as
`localhost/tix:latest`. Builds are `CGO_ENABLED=0` with a pure-Go SQLite driver, so the image needs no libc.

```sh
just image

podman run --rm -p 127.0.0.1:8080:8080 \
  -v tix-data:/var/lib/tix \
  -e TIX_DATABASE_DSN=sqlite:///var/lib/tix/tix.db \
  localhost/tix:latest serve --listen 0.0.0.0:8080 --insecure-no-tls
```

Binding `0.0.0.0` inside the container is what makes the published port reachable, which is why the opt-out is
there. Publish it on `127.0.0.1` and terminate TLS outside, or mount a certificate and drop the opt-out.

## Backup

For SQLite, stop writers or use `sqlite3 tix.db ".backup out.db"` rather than copying a live file. A logical
alternative that works while the server runs, and is portable across engines:

```sh
tix export --comments --artifacts > snapshot-$(date +%F).ndjson
```

Restore into an empty database with `tix import --mode replace`. Try the restore before you need it: `--dry-run`
reports what would change without writing.

## Upgrades

Migrations run automatically when the database is opened, and `tix doctor` reports the applied schema version.
Take a backup before upgrading a shared deployment, and roll one process at a time only after confirming the new
version's migrations have been applied.

## Related

- [api.md](api.md) for routes, errors and the event protocol
- [scaling.md](scaling.md) for when one process stops being enough
- [tenancy.md](tenancy.md) for hostname to tenant mapping
