# Design

## Identity: fingerprint selects the actor, username selects the tenant

SSH offers the server two things before authentication: a username, and proof of a private key. Everything
else arrives after a session opens, which is too late to decide who is connecting.

A fingerprint alone is not sufficient. `actors` are tenant-scoped, and one human with memberships in two
tenants presents the same key to both, so fingerprint to actor is not a function. The username is the only
remaining field, and it carries no meaning to tix otherwise, so it becomes the tenant key.

Resolution order, applied at public-key callback time:

1. The username names a tenant, and the fingerprint is enrolled in it: that actor.
2. The username is the neutral default (`tix`), and the fingerprint is enrolled in exactly one tenant: that
   actor.
3. The username is the neutral default and the fingerprint is enrolled in several: refuse, naming the tenant
   keys to use as the username. Guessing here would drop somebody into the wrong tenant, which is the failure
   this whole design exists to prevent.
4. Otherwise: refuse.

The refusal in case 3 names tenants the holder of that key is already enrolled in, so it discloses nothing to
anyone who did not already have access. Cases 1 and 4 are deliberately indistinguishable to a stranger: an
unenrolled key learns nothing about which tenants exist.

### Why not a key that is global rather than tenant-scoped

A global `ssh_keys` table keyed on fingerprint alone would make resolution a single lookup and avoid the
username entirely. It also puts a credential outside the tenant boundary, where a tenant administrator can
enrol a key that authenticates somewhere they have no rights. Every other credential in tix is tenant-scoped,
and the isolation story is only as strong as its weakest table.

## Schema

```sql
CREATE TABLE ssh_keys (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id),
    actor_id     TEXT NOT NULL REFERENCES actors(id),
    fingerprint  TEXT NOT NULL,
    public_key   TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    last_used_at TEXT,
    revoked_at   TEXT,
    UNIQUE (tenant_id, fingerprint)
);
CREATE INDEX ssh_keys_lookup ON ssh_keys (fingerprint) WHERE revoked_at IS NULL;
```

`UNIQUE (tenant_id, fingerprint)` and not a global unique, per the reasoning above. The partial index serves
the one hot query, which is resolution by fingerprint across tenants in case 2.

The Postgres path additionally needs an RLS policy. `adjustments()` in `internal/store/postgres/schema.go`
only runs at migration 1, so a tenant-scoped table added later receives no policy: that function must be
extended, not just the migration written. This is the known trap recorded against the postgres store, and it
is the reason this table's RLS coverage gets its own leak test rather than being assumed.

`last_used_at` is written on a successful authentication. It is the only write on the authentication path, it
is not in a transaction with anything, and a failure to write it does not fail the connection: an audit
convenience must not become an availability dependency.

## What gets stored is the key, not what was submitted

`ssh.ParseAuthorizedKey` accepts a full authorized_keys line, which may carry options ahead of the key type.
Verified against the library rather than assumed:

```text
input:  command="/bin/sh",no-pty ssh-ed25519 AAAAC3Nza... user@host
result: err=nil, opts=[command="/bin/sh" no-pty], comment="user@host"
```

The options parse successfully and are then discarded by anything that only takes the key. So a service that
stores the submitted text stores attacker-controlled `command=` and `no-pty` directives, which are then shown
in the web interface and carried in exports, and which would be honoured by anything that ever wrote these
rows to a real authorized_keys file.

Enrolment therefore parses the submission and stores `ssh.MarshalAuthorizedKey` of the parsed key: the key
alone, no comment. The fingerprint is taken from the parsed key for the same reason. This also makes the
uniqueness constraint mean what it should, since two submissions differing only in comment or whitespace
collapse to one fingerprint and one canonical form rather than becoming two rows for one identity.

A line carrying options is **refused**, not accepted with the options dropped. Stripping them silently would
leave a submitter who pasted a restricting line believing a restriction had been recorded when none was, and
a false belief about a security control is worse than a rejection. The refusal names the offending field.

Submissions carrying more than one key are refused rather than silently taking the first.

## Revocation

Setting `revoked_at` stops the key authenticating, immediately, because resolution consults it on every
connection. It does not terminate sessions the key already holds.

Cutting live sessions would mean the listener watching for revocations of keys it is currently serving, which
is a second subscription and a second failure mode on the connection path. The exposure is bounded by
`--idle-timeout` (30m by default) and by the keepalive, and the bound is documented next to the command. An
operator who needs a hard cut restarts the listener, which the command's help says.

This is a real limitation and it is stated rather than hidden.

## Running web and SSH in one process

`tix serve --ssh-listen <addr>` constructs the same `sshd.Server` the standalone command does and runs it as
one more member of the server's existing worker set, next to the lease sweeper, webhook dispatcher and
retention pruner. `sshd.Server` already exposes `Listen`, `Serve(ctx)` and `Close`, so no new lifecycle
concept is introduced.

Two properties follow from being one process rather than two:

- **One connection pool.** On SQLite this removes two writer pools contending for one file, which is the
  configuration most likely to be deployed and the one where contention is worst.
- **One shutdown.** SSH sessions are long-lived, so they are drained on the same `--shutdown-timeout` as
  in-flight HTTP requests, and a session still open when it expires is closed with a message rather than cut.

Binding order matters: the SSH listener binds before the HTTP server starts accepting, so a port conflict
fails the process at startup instead of after it has begun answering requests.

`--ssh-listen` is empty by default. Adding a listener that accepts connections is never implied.

The standalone `tix ssh` remains, because a demo box that serves only the sandbox should not have to run a web
server it does not want.

## Why the default flips to enrolled

The current behaviour provisions a tenant for any key that connects. As a demo that is the feature. As the
default for a command an operator reaches for when they want their team on their board, it silently does the
opposite of what they asked, and it does so without erroring, which is the worst shape a security default can
take.

Flipping it breaks existing invocations. The command is pre-1.0, the break is loud (an unenrolled key is
refused with a message naming `--demo`), and the alternative is a default that fails open.

## Per-session renderer

lipgloss resolves colour depth once, globally, from the first output it is given. With many concurrent SSH
sessions that means every client renders at the depth of whichever connected first.

Measured rather than assumed: because the interface's palette is entirely ANSI-16, and termenv passes an
ANSI colour through TrueColor, ANSI256 and ANSI unchanged, a truecolour client and a 256-colour client
produce identical bytes today. The difference that is visible now is colour against none, and it was real:
a client on `TERM=vt100` was being sent 165 colour escapes it cannot display, because the global profile had
been fixed by an earlier session. The per-session renderer is what stops that, and it is also what keeps the
guarantee true if a colour outside ANSI-16 is ever used.

`tui.Config` gains a `*lipgloss.Renderer`. Sessions construct one against their own pty, and the local
terminal path keeps today's behaviour by passing the default renderer. This is carried from `ssh-terminal-access`
task 7.4, which deferred it.

## Rejected alternatives

- **`authorized_keys` file.** No tenant, no actor, no revocation trail, no API, and it would have to be parsed
  and watched. The database already holds every other credential.
- **Enrolment by first connection ("trust on first use").** Indistinguishable from the demo mode it would sit
  next to, and it makes the first connection the security decision, which is exactly backwards.
- **Password authentication as a fallback.** A second credential path to the same actor, and the one an
  operator is most likely to leave weak.
