# Serving the terminal interface over SSH

Read this only if you are standing up a tix SSH listener or connecting to one.
Nothing here is needed to work a queue.

`tix ssh` serves the terminal interface (the same thing as `tix tui`) over
SSH. tix is the SSH server itself: no sshd, no system account, no password. A
public key is the only credential, and declaring a public-key handler and no
other is what refuses a password, a keyboard-interactive exchange and an
authentication offering nothing.

**By default only enrolled keys are accepted.** A key nobody enrolled is
refused. A key is enrolled against one actor in one tenant, and the session
holds exactly the authority that actor's membership grants; the key itself
grants nothing of its own, so a demotion lands on the next connection.

```sh
tix user key add --file ~/.ssh/id_ed25519.pub --actor ada --label laptop
ssh-add -L | head -1 | tix user key add --file - --actor ada
tix user key ls --actor ada
tix user key rm 01JB2K3M4N5P6Q7R8S9T
```

`--actor` takes a handle or an id and defaults to *you*, so enrolling somebody
else's key means naming them. `--file -` reads stdin, `--dry-run` reports what
would be enrolled without writing. The CLI is not the only route: the same
three operations are on the API at `/api/v1/ssh-keys` (`GET`, `POST`, `DELETE
/api/v1/ssh-keys/{id}`), in the browser at `/admin/ssh-keys` and on the
terminal interface's account screen (`@`, your own keys only). `user key rm`
stops a key authenticating at once but does not cut the sessions it already
holds; those end on `--idle-timeout`.

Authority comes from the tenant membership, not from the key, so enrolling
against the implicit `local` actor a zero-configuration CLI runs as
authenticates and then holds **no scopes at all**: it has no membership row.
Pass `--actor` naming a real user.

**An enrolled key also signs a person in on the command line.** `tix login
--ssh-key` tries the keys in `ssh-agent`, then `~/.ssh/id_ed25519`,
`id_ecdsa`, `id_rsa`; `tix login --ssh-key=PATH` uses that key only and asks
for its passphrase without echo. It prints the same session `tix login
--email` does. The signature is an `ssh-keygen -Y sign` signature under the
namespace `tix-login@HOST`, over a nonce valid two minutes and usable once
per key, so it is worthless for git or against another server. A TOTP
enrolment still asks for the code after the key. Over HTTP it is
`POST /api/v1/auth/ssh/challenge` then `POST /api/v1/auth/ssh/verify`. Exit
`5` names the check that failed (not enrolled, revoked, wrong namespace,
expired or used challenge), `2` a configured tenant the server does not
serve. This is for people; an agent still uses a token.

**The username is the tenant selector, not a login name.** A fingerprint
identifies the actor, and SSH offers the server only one other field, so the
username carries the tenant. A key enrolled in exactly one tenant connects as
the neutral user `tix`; a key enrolled in several names the tenant key:

```sh
ssh -p 2222 tix@localhost                 # the one tenant this key is enrolled in
ssh -p 2222 acme@localhost                # this key's enrolment in tenant acme
```

**The command picks the interface.** No command lands on a start menu (Boards,
Watch; a `--demo` sandbox opens its board instead). `board` is the project
list, `board KEY` or a bare `KEY` opens that board (pass `-t`), and `watch`
streams the lines `tix watch` prints, as the key's actor:

```sh
ssh -p 2222 tix@localhost watch infra --type 'task.*'      # plain, pipeable
ssh -p 2222 tix@localhost watch --since 42 -o ndjson | jq .
ssh -t -p 2222 tix@localhost watch                         # coloured; ctrl-c or q ends it
```

`watch` takes `--project` (key or id, resolved under the read check), `--type`,
`--actor`, `--since`, `--limit` and `-o table|ndjson`, plus project keys as
words. `watch` and `board` are reserved first words in any case: a project keyed
`watch` opens with `board watch`. A refused command lists every interface. The
stream ends on client disconnect, the keepalive limit, `tix connection kill`
and shutdown; `--idle-timeout` does not apply to it. Inside the interface `~`
opens the start menu from anywhere.

Every failure leaves by the same door, so a stranger cannot use the listener to
learn which tenants exist. An unenrolled key, a username naming a tenant that
does not exist and a username naming one that holds no enrolment of this key
are one sentence:

```console
$ ssh -p 2222 tix@localhost
tix: this key is not enrolled here: ask an operator to enrol it with `tix user key add`
```

An ambiguous fingerprint is refused rather than guessed, and naming the
candidates discloses nothing because each one already admits this key:

```console
$ ssh -p 2222 tix@localhost
tix: this key is enrolled in more than one tenant: connect as one of acme, default, for example `ssh acme@host`
```

**`--demo` is the sandbox, and it is off by default.** With it, any key is
accepted and handed a seeded ephemeral tenant of its own. The fingerprint is
the identity, so there is no signup: a new fingerprint gets tenant
`sandbox-<32 hex>` named `Demo sandbox SHA256:…`, seeded with one `demo` board
and an actor called `visitor`; the same key coming back gets that tenant
again; and a sandbox nobody visits for `--tenant-ttl` (default 6h) is deleted
with everything in it. The username is ignored in this mode.

```sh
tix ssh --demo --db /var/lib/tix/demo.db
```

A visitor holds an explicit scope set rather than a role: task read, write,
transition, claim and delete, project and workflow read and write,
`comment:write`, `artifact:write`, `event:subscribe`, `audit:read` and
`export`. Held back is what would reach outside the sandbox or break it:
administering the tenant, minting credentials, creating users, registering
webhooks (which would make the listener issue outbound requests) and bulk
import. An enrolled session is not scoped this way at all.

Only `--demo` refuses the zero-configuration store, because a listener that
provisions a tenant for any stranger must not share a database with real work:

```console
$ tix ssh --demo
error: invalid: ssh --demo refuses the database tix keeps your own work in: give the demo a database of its own with --db
```

The enrolled listener has no such refusal; serving the configured target is
the point of it, so `tix ssh --db /var/lib/tix/tix.db` is the intended case.
That means `tix ssh` with no `--db` serves the zero-configuration store, which
is somebody's real work. Name a database unless that is what you meant.

Exit `2` for invalid configuration, `1` for a fatal error. It binds
`127.0.0.1:2222` by default and refuses a non-loopback `--listen` unless you
also pass `--allow-public`. A host key is generated beside the database on
first run unless `--host-key` says otherwise, and a non-SQLite target has no
such place, so `--host-key` is required there. The rest is capacity and
hygiene: `--max-sessions` (100), `--max-sessions-per-key` (3),
`--idle-timeout` (30m), `--keepalive-interval` (30s),
`--keepalive-max-missed` (3), `--rate-per-hour` (60) and `--rate-burst` (5)
per source address. These shape a sandbox and do nothing without `--demo`:
`--max-tenants` (200), `--max-tasks` (200), `--lease-ttl` (2m),
`--reap-interval` (10m). Every one is also a `TIX_SSH_*` environment variable.

The rate limit is refused in the handshake, before any lookup, so exhausting
it looks like `Permission denied (publickey)` from the client and not like a
tix message. Verified: reconnecting past `--rate-burst` from one address.

`tix serve --ssh-listen <addr>` runs the same enrolled listener beside the
HTTP server, over one database and under one shutdown. Sandbox mode is not
available there, because that process serves real work. Its flags are spelled
`--ssh-host-key`, `--ssh-allow-public` and `--ssh-idle-timeout`, and unlike
`tix ssh` they read no configuration key: `TIX_SSH_LISTEN` in the environment
of a `tix serve` binds nothing.
