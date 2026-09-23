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
| `--tls-cert, --tls-key` | (none) | serve HTTPS |
| `--insecure-no-tls` | off | allow a non-loopback bind without TLS |
| `--max-body-bytes` | 1 MiB | request body cap |
| `--request-timeout` | 30s | per-request timeout |
| `--shutdown-timeout` | | how long shutdown waits for in-flight requests |
| `--sweep-interval` | `1m` | how often expired leases are swept |
| `--no-lease-sweeper` | off | disable the sweeper |
| `--prune-interval` | `1h` | how often retention pruning runs |
| `--no-retention-pruner` | off | disable the pruner |
| `--no-webhook-dispatcher` | off | disable the dispatcher, leaving queued deliveries for another drainer |

`--listen` also reads from `server.listen` / `TIX_SERVER_LISTEN`.

The four `--ssh-*` flags belong to the same command and are listed with the listener they turn on, under
[Both listeners in one process](#both-listeners-in-one-process). Together the two tables are every flag
`tix serve` registers, and a test asserts it.

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

Against PostgreSQL in the same compose file or pod, raise `TIX_DATABASE_CONNECT_TIMEOUT` above its `15s`
default. The database is often still starting when this process first reaches it, and the startup check
refuses to serve rather than waiting past its timeout, so a database that is merely slow to accept the first
connection reads as an unreachable one. See [configuration.md](configuration.md).

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

One behaviour change needs an action rather than a backup: `tix ssh` now serves enrolled keys by default and
the sandbox is behind `--demo`. See
[Upgrading a listener that relied on the sandbox](#upgrading-a-listener-that-relied-on-the-sandbox).

## The terminal interface over SSH

`tix ssh` serves the terminal interface over SSH. tix is the SSH server: there is no sshd, no system user
and no shell. A client connects, proves a public key, and lands on a board.

There are two modes, and the safe one is the default:

- **Enrolled**, the default. Only a key somebody recorded against an actor is accepted, and the session holds
  exactly that actor's authority. This is the mode for putting your team on your real board.
- **Demo**, behind `--demo`. Any key is accepted and handed a seeded ephemeral tenant of its own. This is the
  mode for a public sandbox, and it takes a database of its own.

```sh
# enrolled, beside the web interface, one process
tix serve --db /var/lib/tix/tix.db --ssh-listen 127.0.0.1:2222

# enrolled, on its own
tix ssh --db /var/lib/tix/tix.db

# the sandbox
tix ssh --demo --db /var/lib/tix/demo.db
```

The default used to be the sandbox. If you are upgrading, read
[Upgrading a listener that relied on the sandbox](#upgrading-a-listener-that-relied-on-the-sandbox).

### Enrolling a key

A public key is the only credential. No password, no keyboard-interactive, no fallback. Enrol the public
half, never the private one:

```sh
tix user key add --file ~/.ssh/id_ed25519.pub --actor ada --label laptop
tix user key ls --actor ada
tix user key rm 01JB2K3M4N5P6Q7R8S9T
```

`--actor` takes a handle or an identifier and defaults to you, so enrolling somebody else's key means naming
them, and doing that needs authority over that actor in the tenant. `--file -` reads standard input:

```sh
ssh-add -L | head -1 | tix user key add --file - --actor ada
```

`--dry-run` reports what would be enrolled without writing. `--label` is a note for telling one key from
another.

What is stored is the parsed key alone. The comment `ssh-keygen` writes on the end is dropped, and a line
carrying authorized_keys options such as `command=` or `no-pty` is refused rather than accepted with the
options quietly discarded. Two spellings of one key are one identity, so re-enrolling the same key in a
tenant is a conflict, not a second row.

Exit codes: `add` gives 2 for an invalid key, 3 for an unknown actor, 4 for a key already enrolled and 5 for
permission denied. `rm` gives 3 for an unknown key and 5 for permission denied.

`tix user key ls` defaults to your own actor, so an administrator looking for somebody else's key has to pass
`--actor`. Revoked keys are listed too, carrying a revocation time rather than being omitted: a key that
stopped working is usually the one being looked for.

The same three operations are on the API at `/api/v1/ssh-keys` (`GET`, `POST`, and `DELETE
/api/v1/ssh-keys/{id}`) and in the web interface at `/admin/ssh-keys`.

### The username selects the tenant

A fingerprint identifies the actor. A key enrolled in two tenants speaks for a different actor in each, and
SSH offers the server only one other field before authentication, so the username carries the tenant.

A key enrolled in exactly one tenant needs no username. Use the neutral default:

```sh
ssh -p 2222 tix@tix.example.com
```

A key enrolled in several is resolved by naming one, using the tenant key:

```sh
ssh -p 2222 acme@tix.example.com
```

Connecting without naming one is refused, and the refusal names the tenants to choose between:

```console
$ ssh -p 2222 tix@tix.example.com
tix: this key is enrolled in more than one tenant: connect as one of acme, default, for example `ssh acme@host`
```

No tenant is picked on your behalf. Guessing would drop somebody into the wrong board.

A key nobody enrolled gets a refusal that says nothing about what exists:

```console
$ ssh -p 2222 tix@tix.example.com
tix: this key is not enrolled here: ask an operator to enrol it with `tix user key add`
```

Naming a tenant that exists and naming one that does not are answered the same way, and the ambiguity message
names only tenants that key is already enrolled in.

### The zero-configuration `local` actor cannot use the hosted listener

This one looks exactly like a bug the first time you hit it, so here it is plainly.

A CLI with no configuration runs as an implicit actor called `local`, which holds `ScopeAll` in-process. It
has no `tenant_members` row. The enrolled lookup reads authority from membership, so a key enrolled against
`local` authenticates, opens a session, and then holds **no scopes at all**:

```console
$ tix user key add --file ~/.ssh/id_ed25519.pub --db /var/lib/tix/tix.db
enrolled SHA256:w2wQhqhgmnIU01ZAXN279/kVpeLT94MoitXJddsSuVE

$ ssh -p 2222 tix@localhost
tix │ as local │ ◌ disconnected, retrying
0 projects │ ✗ forbidden: missing required scope
0 projects │ ✗ event stream: forbidden: missing required scope
```

Nothing is broken. The key is enrolled against an actor that is not a member of anything.

Create a real actor and enrol against that:

```sh
tix user create ada@example.com --handle ada --role admin --db /var/lib/tix/tix.db
tix user key add --file ~/.ssh/id_ed25519.pub --actor ada --db /var/lib/tix/tix.db
```

```console
$ ssh -p 2222 tix@localhost
tix │ as ada │ ● live
4 projects │ ✓ connected
```

`tix user key add` without `--actor` enrols against whoever you are, which on a zero-configuration CLI is
`local`. Pass `--actor` when enrolling for a hosted listener.

### Revocation does not cut live sessions

`tix user key rm ID` stops the key authenticating immediately. It does not touch sessions the key is already
holding, and the command says so:

```console
$ tix user key rm 01JB2K3M4N5P6Q7R8S9T
sessions this key already holds end on the listener's idle timeout
```

A held session is bounded by `--idle-timeout`, 30 minutes by default, and by the keepalive. Two ways to make
that bound shorter or unnecessary:

- Lower `--idle-timeout` (`--ssh-idle-timeout` on `tix serve`) for the deployment.
- Cut the session now with `tix connection kill`. See [Live connections](#live-connections).

Restarting the listener also works, and ends everybody else's session too.

### Flags

These apply in both modes:

| Flag | Default | Effect |
| --- | --- | --- |
| `--demo` | off | accept any key and give it a seeded ephemeral tenant, instead of serving enrolled keys |
| `--listen` | `127.0.0.1:2222` | address to bind |
| `--host-key` | beside the database | persisted host key, generated on first run at mode 0600 |
| `--allow-public` | off | allow a non-loopback bind |
| `--rate-per-hour` | `60` | connections per hour from one source address |
| `--rate-burst` | `5` | connections one source may make back to back |
| `--idle-timeout` | `30m` | how long a session may sit idle with nobody typing |
| `--keepalive-interval` | `30s` | how often a client is asked whether it is still there |
| `--keepalive-max-missed` | `3` | unanswered keepalives before the connection is dropped |
| `--max-sessions-per-key` | `3` | sessions one key may hold at once |
| `--max-sessions` | `100` | sessions the listener may hold at once |

These shape the sandbox and do nothing without `--demo`:

| Flag | Default | Effect |
| --- | --- | --- |
| `--tenant-ttl` | `6h` | how long a sandbox survives without a visit |
| `--reap-interval` | `10m` | how often expired sandboxes are deleted |
| `--max-tenants` | `200` | live sandboxes before a new key is refused |
| `--max-tasks` | `200` | tasks one sandbox may hold |
| `--lease-ttl` | `2m` | lease length in a seeded sandbox |

Every one of these is also a configuration key under `ssh.` with a generated `TIX_SSH_*` variable, so a
container deployment does not have to reach the command line for any of it. The flag wins where one was
given; see [configuration.md](configuration.md) for the key list and the layer order.

```sh
# the same listener, configured rather than flagged
TIX_SSH_LISTEN=0.0.0.0:2222 TIX_SSH_ALLOW_PUBLIC=true TIX_SSH_DEMO=true TIX_SSH_MAX_TENANTS=500 \
  tix ssh --db /var/lib/tix/demo.db
```

### Both listeners in one process

`tix serve --ssh-listen <addr>` runs the SSH listener beside the HTTP server, over one database, under one
shutdown, next to the lease sweeper, webhook dispatcher and retention pruner.

```sh
tix serve --db /var/lib/tix/tix.db --listen 127.0.0.1:8080 --ssh-listen 127.0.0.1:2222
```

```console
tix listening on 127.0.0.1:8080
tix ssh listening on 127.0.0.1:2222
```

Four flags, and no demo mode: this process serves real work.

| Flag | Default | Effect |
| --- | --- | --- |
| `--ssh-listen` | (none) | address to serve the terminal interface over SSH on |
| `--ssh-host-key` | beside the database | persisted SSH host key, generated on first run |
| `--ssh-allow-public` | off | allow a non-loopback SSH bind |
| `--ssh-idle-timeout` | `30m` | how long an SSH session may sit idle, which bounds a revoked key |

Three things to know:

- **Only the flag turns it on.** `--ssh-listen` is empty by default, and unlike `tix ssh` these four read no
  configuration key: `TIX_SSH_LISTEN` in the environment of a `tix serve` binds nothing.
- **Set `--ssh-host-key` on anything but SQLite.** The default is `ssh_host_ed25519_key` beside the database
  file, generated on first run at mode 0600, so a PostgreSQL target has no default and the process refuses to
  start:

  ```console
  $ tix serve --db postgres://tix@db/tix --ssh-listen 127.0.0.1:2222
  error: invalid: a host key path is required for this target; pass --host-key
  ```

  That is exit 2. On `tix serve` the flag to pass is `--ssh-host-key`, whatever the message says. Give it a
  stable path on persistent storage either way: a host key regenerated on every restart hands every returning
  user a changed-host-key warning, which is the warning you want people to take seriously.
- **The SSH listener binds first.** A port conflict fails the process at startup, before the HTTP server
  accepts anything, rather than after it has begun answering requests.

SSH sessions drain on `--shutdown-timeout` alongside in-flight HTTP requests. A session still open when it
expires is told the server is shutting down and then closed, rather than cut silently.

### Demo mode

`--demo` is the public sandbox. Any public key is accepted: SSH requires a client to prove a key, but nothing
requires the server to have seen it before, and that proof is the whole identity here. No signup, no
password, no enrolment.

Each fingerprint gets an **ephemeral tenant of its own**, seeded on first connection with a demo board. The
same key connecting again gets the same tenant back, with whatever the visitor changed still in it. Two
visitors are two tenants, so the scoped query builder, the tenant predicate and, on PostgreSQL, row-level
security are all doing their real job.

A visitor holds an explicit set of scopes rather than a role: enough to work the board, edit projects and
edit workflows, and not enough to administer the tenant, mint tokens, create users, register webhooks or bulk
import.

#### Its own database

This listener faces strangers, so it takes its own target. `tix ssh --demo` refuses the zero-configuration
store:

```console
$ tix ssh --demo
error: invalid: ssh --demo refuses the database tix keeps your own work in: give the demo a database of its
own with --db
```

That is exit 2. Give it a database nothing else uses: sandboxes are created and deleted in it continuously,
and no real tenant should share that process boundary. Enrolled mode has no such refusal, because serving the
configured target is the point.

#### Expiry, and why it slides

The time to live counts from the **last connection**, not from creation, so someone who keeps coming back
keeps their board. A sandbox nobody has visited for the time to live is deleted with everything under it: the
delete is the store's hard delete, and every tenant-owned table cascades from `tenants(id)`.

When `--max-tenants` is reached a **new** fingerprint is refused with a message saying so. An existing
sandbox is never evicted to make room; silently deleting somebody's work to admit a stranger would be the
worst behaviour available. A fingerprint whose sandbox has already been reaped simply gets a fresh one.

The interface says, on the board and again when the session ends, that this is a sandbox and roughly how long
it survives unvisited.

### Upgrading a listener that relied on the sandbox

**Breaking change.** `tix ssh` provisioned a sandbox for any key that connected. It now serves enrolled keys
only, and refuses a key nobody enrolled.

If you were running a demo, add `--demo`:

```diff
-tix ssh --db /var/lib/tix/demo.db --listen 0.0.0.0:2222 --allow-public
+tix ssh --demo --db /var/lib/tix/demo.db --listen 0.0.0.0:2222 --allow-public
```

Or set `ssh.demo: true`, or `TIX_SSH_DEMO=true`, which is usually easier in a container.

If you were not running a demo, you were running the wrong thing and did not know: enrol the keys that should
reach the board with `tix user key add --actor <handle>` and drop the sandbox flags, which do nothing without
`--demo`.

The break is loud. An unenrolled key is refused on connection with a message naming enrolment, and nothing
fails open.

### A client that goes away without saying so

`--idle-timeout` closes a session nobody is typing at. It does not notice a session whose **client** has gone: a
closed laptop, an expired NAT entry, a dropped network. Nothing arrives and nothing closes, so that session holds
its slot against `--max-sessions` and keeps any lease it was carrying until the idle timeout finally expires. On
a demo whose whole argument is that a lease returns work when a worker dies, a zombie session sitting on a claim
is precisely the wrong demonstration.

So the listener asks. Every `--keepalive-interval` it sends the request every SSH client answers, and after
`--keepalive-max-missed` unanswered ones it closes the connection rather than the session, because a write to a
vanished client blocks until TCP gives up. The defaults notice in about two minutes, which is the length of a
seeded lease rather than the length of the idle timeout.

The two are independent by construction. A keepalive is traffic, and so is its reply, so an idle clock fed by
traffic would be reset by the very mechanism meant to detect an absent client. The idle clock is fed by the key
and mouse messages the interface receives instead, which no protocol traffic produces: keepalives refresh the
transport's own deadlines and move nothing that decides idleness.

### Caps on live sessions

`--rate-per-hour` counts connections from one source address over an hour. It says nothing about how many are
still open, so one key can hold fifty sessions at once, each a program with its own event subscription, without
ever exceeding its allowance. `--max-sessions-per-key` and `--max-sessions` cap that.

A session over either cap is refused on the session's standard error, before any board is drawn, with a message
naming the limit:

```console
$ ssh -p 2222 visitor@localhost
tix: this key is at its limit of 3 concurrent sessions; close one and reconnect
```

The cap is taken before the key is looked up at all, and the wording does not depend on the key, so a refusal
cannot tell a stranger whether this listener had seen theirs before.

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

Each session resolves this from its own client. A client on a terminal that cannot show colour gets none,
whatever any other session connected before it is using.

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

## Live connections

A running server holds two kinds of live connection: browsers and API clients on the event stream, and
terminal sessions over SSH. `tix connection` shows them and ends them.

```sh
tix connection ls
tix connection kill 01JB2K3M4N5P6Q7R8S9T
```

Both need `tenant.admin`. The same two operations are on the API at `GET /api/v1/connections` and `DELETE
/api/v1/connections/{id}`, and in the web interface at `/admin/connections`.

### The view is one server's, not the database's

The registry is in memory, per process. A connection lives in exactly one server, and you query the server
holding it.

That is worth stating because the obvious thing to do is the wrong one. `tix connection ls --db <file>` is a
separate process with its own empty registry, so it reports zero while sessions are live:

```console
$ tix connection ls --db /var/lib/tix/tix.db
answered by AMGUL-555b237f; this view covers this server only
Server AMGUL-555b237f, holding 0 connections in total.
No results.
```

Ask the server instead:

```console
$ tix connection ls --server https://tix.example.com --token "$TIX_TOKEN"
answered by AMGUL-e67c6b0a; this view covers this server only
Server AMGUL-e67c6b0a, holding 2 connections in total.
┌────────────────────────────┬─────────┬───────┬───────────┬──────────────────┬──────────────────────┐
│ ID                         │ SURFACE │ ACTOR │ FROM      │ SINCE            │ KEY                  │
├────────────────────────────┼─────────┼───────┼───────────┼──────────────────┼──────────────────────┤
│ 01M34EAF2S6B0AGD9JRW3C2A4N │ ssh     │ ada   │ 127.0.0.1 │ 2026-09-22 14:34 │ SHA256:PvO4MBAgUf... │
└────────────────────────────┴─────────┴───────┴───────────┴──────────────────┴──────────────────────┘
```

Every answer names the server that gave it and says the view covers that server only. Run several servers
against one database and you get a partial view from each, which is why the line is there rather than left to
be inferred. Nothing is stored: a restart empties the registry, because the connections are gone too.

`SURFACE` is `events` for an event-stream connection and `ssh` for a terminal session. `KEY` is the
fingerprint that opened an SSH session, and is empty for anything else.

### Counts

The per-surface counts are your tenant's. The process total covers every connection the server holds and
carries no breakdown by tenant, actor or address, because "this server is holding 240 connections" is
capacity information an operator needs and says nothing about who anybody else is.

```console
$ tix connection ls --server https://tix.example.com --token "$TIX_TOKEN" -o json
{
  "server_id": "AMGUL-e67c6b0a",
  "connections": [ ... ],
  "counts": {"events": 0, "ssh": 1, "tenant": 1, "process": 2}
}
```

A connection belonging to another tenant is absent from the list and from the tenant counts, and its
identifier is reported as not found rather than as forbidden.

### Ending is not revocation

These are two different controls and neither substitutes for the other:

- **Revoking a key stops the NEXT connection.** `tix user key rm ID` stops the key authenticating at once.
  The session it is already holding stays up until the idle timeout.
- **Ending a connection stops THIS one.** `tix connection kill ID` closes the socket now. It takes nothing
  away: the holder may reconnect immediately with credentials that are still valid.

The command says so:

```console
$ tix connection kill 01JB2K3M4N5P6Q7R8S9T
ending is not revocation; the holder may reconnect
┌────────────────────────────┬────────┬──────┬───────┐
│ REF                        │ STATUS │ CODE │ ERROR │
├────────────────────────────┼────────┼──────┼───────┤
│ 01M34EAF2S6B0AGD9JRW3C2A4N │ ok     │      │       │
└────────────────────────────┴────────┴──────┴───────┘
```

To get somebody off and keep them off, do both: revoke the key or the token, then end the connections it is
holding. An identifier this server does not hold is exit 3:

```console
$ tix connection kill 01JB2K3M4N5P6Q7R8S9T
error: not_found: no live connection "01JB2K3M4N5P6Q7R8S9T" on this server
```

Ending is audited. The audit entry names the actor ended, the surface and the caller, and it commits before
the connection closes: a record of a cut that did not happen is a smaller problem than a cut with no record.

## Related

- [api.md](api.md) for routes, errors and the event protocol
- [scaling.md](scaling.md) for when one process stops being enough
- [tenancy.md](tenancy.md) for hostname to tenant mapping
- [agents.md](agents.md) for the lease behaviour the demo board shows off
- [configuration.md](configuration.md) for the `ssh.*` keys and the layer order
