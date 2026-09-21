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

Two settings that shape how the server is reached have no flag and are set from configuration only:
`server.trusted_proxies` / `TIX_SERVER_TRUSTED_PROXIES` and `server.cookie_security` /
`TIX_SERVER_COOKIE_SECURITY`. See [configuration.md](configuration.md).

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

**List the proxy in `server.trusted_proxies`.** Without it tix reads neither `X-Forwarded-Proto` nor
`X-Forwarded-For`, because any client can send them: a forged `X-Forwarded-Proto: https` would otherwise be
enough to change how cookies are issued, and a forged `X-Forwarded-For` would be enough to wear somebody else's
address. With the proxy listed, the client's scheme and address are taken from those headers when, and only
when, the connection came from that proxy.

```sh
TIX_SERVER_TRUSTED_PROXIES=127.0.0.1,::1 tix serve --listen 127.0.0.1:8080
```

```yaml
server:
  trusted_proxies: [127.0.0.1, "::1"]
```

This is what puts `Secure` on the session cookie in this deployment. tix terminates no TLS here, so it holds no
certificate, and the flag follows the scheme the client used rather than the scheme of the hop to the proxy. Set
`server.cookie_security` to `always` or `never` to override the derivation; see
[configuration.md](configuration.md).

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

## Webhook delivery targets

A webhook endpoint is registered by a tenant administrator, who is a lower-privilege party than whoever runs the
server. The url they supply is a request the server makes from inside your network, so it is treated as untrusted
input rather than as configuration.

Registration and delivery both refuse a target that resolves to:

- loopback, including `localhost`, `127.0.0.0/8` and `::1`
- link-local, which is where the cloud metadata endpoint lives at `169.254.169.254` and `fe80::/10`
- RFC1918 (`10/8`, `172.16/12`, `192.168/16`) and IPv6 unique-local `fc00::/7`
- the unspecified address, multicast, the IPv4 broadcast address, carrier-grade NAT `100.64/10`, IETF protocol
  assignments `192.0.0.0/24`, the benchmarking range `198.18/15` and the NAT64 well-known prefixes

Three details are worth knowing, because a guard that only inspects the string is not a guard:

- **The address decides, not the name.** The check runs again in the dialer, immediately before connect, on the
  address the socket is about to use. A hostname that answers publicly while the endpoint is being registered and
  answers `127.0.0.1` when the delivery is attempted is refused at that second point, so DNS rebinding does not
  get past it. The lookup done at registration time is a courtesy that gives an immediate error for the obvious
  case; it is not what the guard rests on.
- **Redirects are not followed.** A delivery that is answered with a 3xx stops there and is recorded as a failed
  attempt carrying that status. Following it would take the signature headers to a host that passed no check,
  which is the simplest way around any address policy. A receiver that needs to move should be re-registered at
  its new url.
- **No proxy is used for delivery.** `HTTP_PROXY` and friends are ignored on this path, because a proxied request
  dials the proxy and the guard would then be inspecting the proxy's address rather than the endpoint's.

A url that embeds a username or password (`https://user:pass@host/hook`) is refused outright. Send a credential in
a header your receiver checks, or in a path segment you treat as a bearer.

A delivery failure is recorded in categories only — `endpoint address is not permitted`, `endpoint host could not
be resolved`, `endpoint did not answer in time`, `endpoint could not be reached` — because the delivery log is
readable by the tenant who registered the endpoint, and a precise transport error turns it into a port scan of
your network.

### Allowing an internal target

Some deployments genuinely deliver to something inside the network: a queue bridge on the same host, a service on
the cluster network. That is allowed only when the operator turns it on, never by a tenant, and it is off by
default.

Today the allowance is the same switch as the plaintext opt-out: the `service.WithInsecureWebhooks(true)`
construction option. It is not yet reachable from a flag, an environment variable or the config file, so a stock
`tix serve` refuses every internal target. If your deployment needs one, that wiring has to be added first — see
the note in `internal/service/local.go`.

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

Back the `/var/lib/tix` volume with local storage. A named volume on the container host is fine; an NFS- or
SMB-backed volume, or a network block device mounted from another host, is not: `tix` refuses to open a SQLite
database it detects on one, for the reasons in [scaling.md](scaling.md#sqlite). Run PostgreSQL instead if the
deployment needs its database on shared storage.

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

## The terminal interface over SSH

`tix ssh` serves the terminal interface over SSH. tix is the SSH server: there is no sshd, no system user
and no shell. A client connects, proves a public key, and lands on a board.

```sh
tix ssh --db /var/lib/tix/demo.db
ssh -p 2222 visitor@localhost
```

### The fingerprint is the identity

Any public key is accepted. SSH requires a client to prove a key, but nothing requires the server to have seen
it before, and that proof is the whole identity here: no signup, no password, no enrolment.

Each fingerprint gets an **ephemeral tenant of its own**, seeded on first connection with a demo board. The same
key connecting again gets the same tenant back, with whatever the visitor changed still in it. Two visitors are
two tenants, so the scoped query builder, the tenant predicate and, on PostgreSQL, row-level security are all
doing their real job.

A visitor holds an explicit set of scopes rather than a role: enough to work the board, edit projects and edit
workflows, and not enough to administer the tenant, mint tokens, create users, register webhooks or bulk import.

### Its own database

This listener faces strangers, so it takes its own target. `tix ssh` refuses the zero-configuration store:

```console
$ tix ssh
error: invalid: ssh refuses the zero-configuration store, which is somebody's real work: name a database of its
own with --db
```

Give it a database nothing else uses. Sandboxes are created and deleted in it continuously, and no real tenant
should share that process boundary.

### Flags

| Flag | Default | Effect |
| --- | --- | --- |
| `--listen` | `127.0.0.1:2222` | address to bind |
| `--host-key` | beside the database | persisted host key, generated on first run at mode 0600 |
| `--allow-public` | off | allow a non-loopback bind |
| `--tenant-ttl` | `6h` | how long a sandbox survives without a visit |
| `--reap-interval` | `10m` | how often expired sandboxes are deleted |
| `--max-tenants` | `200` | live sandboxes before a new key is refused |
| `--max-tasks` | `200` | tasks one sandbox may hold |
| `--lease-ttl` | `2m` | lease length in a seeded sandbox |
| `--rate-per-hour` | `60` | connections per hour from one source address |
| `--rate-burst` | `5` | connections one source may make back to back |
| `--idle-timeout` | `30m` | how long a session may sit idle |

### The non-loopback bind guard

Like `tix serve`, binding anything other than loopback needs an explicit choice:

```console
$ tix ssh --db /var/lib/tix/demo.db --listen 0.0.0.0:2222
error: invalid: refusing to bind non-loopback address "0.0.0.0:2222" without tls: configure a certificate and
key, or pass the explicit insecure opt-out
```

Pass `--allow-public` when that is what you want. SSH encrypts its own transport, so unlike `tix serve` there is
no certificate to configure; the guard exists so a demo is never exposed by accident.

### Colour

Whether a session is drawn in colour is decided from what the client said, not from the server's own terminal:
`NO_COLOR` or `TIX_NO_COLOR` set to a non-empty value, a `TERM` of `dumb`, or a session that names no terminal
type all get a monochrome board. Everything else gets colour, at ANSI-16, which is the whole palette the
interface uses.

### Expiry, and why it slides

The time to live counts from the **last connection**, not from creation, so someone who keeps coming back keeps
their board. A sandbox nobody has visited for the time to live is deleted with everything under it: the delete
is the store's hard delete, and every tenant-owned table cascades from `tenants(id)`.

When `--max-tenants` is reached a **new** fingerprint is refused with a message saying so. An existing sandbox is
never evicted to make room; silently deleting somebody's work to admit a stranger would be the worst behaviour
available. A fingerprint whose sandbox has already been reaped simply gets a fresh one.

The interface says, on the board and again when the session ends, that this is a sandbox and roughly how long it
survives unvisited.

### Port 22

Binding port 22 for a bare `ssh tix.example.com` is a deployment problem, not a code one. tix binds 2222 and
does not ask for the privilege to bind a low port. Put one of these in front:

```sh
# iptables
iptables -t nat -A PREROUTING -p tcp --dport 22 -j REDIRECT --to-port 2222

# systemd socket activation, or simply
# AmbientCapabilities=CAP_NET_BIND_SERVICE in the unit, with --listen 0.0.0.0:22 --allow-public
```

If the host already runs a real sshd on 22, give tix its own address or leave it on 2222.

## Related

- [api.md](api.md) for routes, errors and the event protocol
- [scaling.md](scaling.md) for when one process stops being enough
- [tenancy.md](tenancy.md) for hostname to tenant mapping
- [agents.md](agents.md) for the lease behaviour the demo board shows off
