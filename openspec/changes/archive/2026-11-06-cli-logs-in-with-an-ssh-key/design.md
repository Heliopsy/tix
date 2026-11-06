# Design

## Protocol

1. The client asks `POST /api/v1/auth/ssh/challenge`. The server answers `nonce`, `namespace` and
   `expires_at`. Nothing is stored.
2. The client builds the namespace itself, `tix-login@` followed by the host of the server URL it was
   given, and refuses to sign if the server announced a different one.
3. The client signs the nonce bytes as an OpenSSH `sshsig` signature, SHA-512, under that namespace:
   byte for byte what `ssh-keygen -Y sign -n tix-login@HOST` produces for a file holding the nonce.
4. `POST /api/v1/auth/ssh/verify` carries `public_key` (authorized_keys form), `signature` (the armored
   block), `nonce`, and optionally `tenant`. The server checks, in order: the nonce, the signature, the
   enrolment, the account, the second factor, and only then claims the nonce and mints the session.

## The nonce is signed, not stored

The nonce is `base64(payload).base64(HMAC-SHA256)` over tenant, audience, issue time, expiry and 16
random bytes, the same shape as the TOTP sign-in ticket. It lives two minutes. A stored challenge would
be another credential table with its own sweep, and what it has to prove (this server issued this value
for this tenant and host a moment ago) a signature proves as well.

The MAC key is derived from `auth.totp_key` (HMAC under a fixed label, so it is never the sealing key
itself) when one is configured, and is otherwise 32 random bytes drawn when the process starts. Without a
configured key, a challenge from one replica does not verify on another, and every challenge dies with a
restart. That fails closed and costs a retry; a deployment running several replicas without sticky
routing sets `auth.totp_key`, which it already needs for second factors.

## Replay: a compare-and-swap on `last_used_at`

The brief offered remembering used nonces in memory, or a table. Memory is per process, so behind a load
balancer a captured `(nonce, signature)` would replay on another replica inside the window. A table
needs a migration and a sweep.

Instead, the claim is one conditional update of the enrolment the signature proved:

    UPDATE ssh_keys SET last_used_at = max(now, issued)
    WHERE tenant_id = ? AND id = ? AND (last_used_at IS NULL OR last_used_at < issued)

It succeeds only for a nonce issued after the key's last use, and the row it writes makes that nonce,
and every older one, unusable for this key. It runs in the transaction that mints the session, so a
refused claim leaves no session, and it is atomic across replicas because the database decides it. The
`max` is for clock skew between replicas: a nonce issued by a replica whose clock runs ahead still ends
at or below the stored instant.

Consequences, all failing closed:

- Single use is per key. One challenge may be tried with several keys until one is accepted, which is
  what lets the client walk the agent's keys without a round trip each. Using it with a second enrolled
  key needs that key's private half, which is a login, not a replay.
- Two logins with one key racing on overlapping challenges: the later-issued one wins, the other is told
  to start again.
- The SSH listener records `last_used_at` when the key opens a terminal session, which supersedes a
  challenge issued before it. The client asks for a new one.
- Timestamps are truncated to microseconds when minted, so PostgreSQL's precision cannot make an issued
  instant compare unequal to itself.

## Bound to the server, the tenant and the purpose

- The namespace `tix-login@HOST` is checked by the server against the Host of the request, the same
  value that resolved the tenant. A malicious server relaying a real server's challenge to a user cannot
  get it signed for the real host, because the client takes the host from its own configuration, not
  from the challenge.
- The nonce carries the audience and the tenant, so it is refused on another host or tenant.
- `sshsig` itself separates namespaces: a git commit signature (`git`) or a file signature (`file`)
  never verifies as a login, and a login signature never verifies as either.
- In local mode (`--db`) there is no host; the audience is `local` and the namespace `tix-login@local`.
- `ssh-rsa` (SHA-1) signatures are refused, as `sshsig` requires.

## Tenant

Over HTTP the tenant is already decided before credentials are looked at: the Host names it, as for the
password login, and a session minted for any other tenant would be refused by the next request anyway.
So the SSH username rule (a key enrolled in several tenants must name one) collapses here: the Host is
the name. A key enrolled in two tenants signs in to whichever one the server address resolves to, and
the enrolment in the other tenant is never read. The lookup is tenant-scoped (`GetSSHKeyByFingerprint`
on `store.Tx`), so the one cross-tenant door the SSH listener uses stays the only one.

`tenant` in the request is an assertion: when the caller names one (the CLI sends its configured
tenant) and the server resolved a different one, the login is refused naming both, rather than quietly
signing in somewhere the caller did not mean.

## Errors

Every refusal is `unauthenticated` (HTTP 401, exit 5) unless noted, and says which check failed: the
challenge expired, the challenge is not valid for this server, the signature does not verify, the
signature was made for another namespace, the key is not enrolled in this tenant, the key was revoked,
the challenge was already used, the key belongs to an agent rather than a person. A tenant mismatch is
`invalid` (exit 2). The two enrolment refusals carry `details.reason` (`ssh_key_not_enrolled`,
`ssh_key_revoked`) so the client knows to try its next key.

Saying "not enrolled" or "revoked" discloses nothing to a stranger: the signature is verified first, so
only the holder of the private key ever learns either.

The enrolment status of a public key is never answered without a signature. An endpoint that took a
list of public keys and said which were enrolled would let anyone test the keys GitHub publishes for
every user against a tenant.

## Second factor: still required

An SSH key replaces the password, not the code. A person with a confirmed TOTP enrolment, or in a tenant
that requires a factor on the command line, gets the same `code` (or `enrol`) challenge the password
step returns, with a ticket that remembers the method, and finishes with the existing `Login` call. The
key is something you have, but it is commonly a file on the same machine as the shell, or an agent
forwarded to hosts the person does not control; the factor the person or their tenant chose stays in
front of a session. This matches the SSH listener, which also asks for the code after the key.

## Rate limiting

Consistent with the password path. The password step has no request counter of its own (its brake is
the hash cost), and the second factor is limited per enrolment, five failures then fifteen minutes. An
SSH login reaches the same code step and the same lockout. The signature step has nothing to guess, and
verifying one costs microseconds, so a counter there would protect nothing and would let a stranger
lock a person out by address.

## Constant time

The nonce MAC is compared with `hmac.Equal`, and the public key in the signature against the presented
one with `subtle.ConstantTimeCompare`. Fingerprints are looked up, not compared.

## Only people

A key enrolled against an agent actor is refused here: an agent authenticates with an API token, and a
session belongs to a person the second-factor policy can apply to. A disabled user is refused like a
wrong password.

## Client

The key handling lives in `internal/sshlogin`, so `cmd/` stays wiring. Agent keys come first, then
`id_ed25519`, `id_ecdsa`, `id_rsa` under `~/.ssh`. An encrypted file key asks for its passphrase on
the terminal with echo off, and only when it is about to be used. One challenge is fetched and tried
with each key in turn; the next key is tried only on the two enrolment refusals.
