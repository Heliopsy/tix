# Host the SSH interface for real users

## Why

`tix ssh` today serves one identity model: any key is accepted, and its fingerprint is hashed into an
ephemeral sandbox tenant that is seeded with demo content and reaped after a time to live. That is the right
shape for a public demo and the wrong shape for everything else. An operator who wants their team to reach
their real board over SSH has no way to say "this key is Alice", because no table records a public key at all.

The seam for this already exists and says so. `auth.PublicKeyLookup` resolves a fingerprint to an actor, and
the sandbox provisioner is one implementation of it. What is missing is the second implementation and the
table behind it.

Separately, a host that wants both the web interface and SSH runs two processes against one database today.
That works, because the outbox makes a mutation on one visible to the other, but it costs two service units,
two configuration surfaces, two shutdown paths, and on SQLite two writer pools competing for the same file.

## What Changes

- **Enrolled keys.** An `ssh_keys` table records a public key against an actor in a tenant. A second
  `PublicKeyLookup` resolves a presented fingerprint to that actor and refuses a key nobody enrolled.
- **Two listener modes, and the safe one is the default.** `--demo` opts in to the sandbox behaviour that is
  currently unconditional. Without it the listener serves enrolled keys only. A listener that hands a tenant
  to any stranger should be a deliberate choice.
- **The SSH username selects the tenant.** A fingerprint enrolled in exactly one tenant needs no username. A
  fingerprint enrolled in several is ambiguous, and `ssh acme@host` resolves it, because the username is the
  only field an SSH client offers before authentication.
- **`tix user key add | ls | rm`**, with the matching API routes and web screen the capability registry
  requires of every operation.
- **Revocation.** A revoked key stops authenticating. Live sessions it already holds are bounded by the idle
  timeout rather than cut, which is stated as a limit rather than left to be discovered.
- **`tix serve --ssh-listen`** runs both listeners in one process, under one lifecycle, over one connection
  pool, alongside the sweeper, dispatcher and pruner that already run there.
- **A renderer per session**, so colour depth follows each client's terminal instead of whichever client
  connected first.

## Impact

- The default behaviour of `tix ssh` changes. A deployment relying on sandbox provisioning must add `--demo`.
  This is a breaking change to a command released before 1.0, and it is called out in the release notes.
- `tix ssh` refuses the zero-configuration store today because it faces strangers. In enrolled mode serving
  the real store is the point, so that refusal narrows to demo mode.
