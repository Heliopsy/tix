# Design

## The credential is global, so it hangs off `users`

`users` has no tenant column and `users.email` is globally unique. `actors` is
tenant-scoped and carries a nullable `user_id` back to the global row. The
documented tenancy model is one credential set per human, with an actor and a
role per tenant they belong to.

A second factor is part of a credential set, so `user_totp` references
`users(id)`. One phone, one enrolment, every tenant.

The alternative, enrolment per actor, would mean a person in three tenants
scanning three QR codes and holding three entries in their authenticator app
for one identity, and would make "require TOTP" in tenant A silently satisfiable
by not enrolling there. It is the wrong shape.

**This collides with a defect that already exists.** `internal/service/user.go`
mints the actor with the user's own ID:

```go
id := l.ids.New()
u := &core.User{ID: id, Email: email, ...}
a := &core.Actor{ID: id, Kind: core.ActorUser, ...}
```

So one user can only ever have one actor, and no operation attaches an existing
user to a second tenant. The schema allows what the service forbids. This change
does **not** fix that: it is a separate defect with its own blast radius, and
bundling a credential feature with an identity-model fix would make both harder
to review. But the design is stated here because `user_totp` keying on `users`
is only correct under the model the schema already describes, and anyone
reading this later should know the two are connected.

## Algorithm: the boring choice, on purpose

RFC 6238, HMAC-SHA1, 6 digits, 30-second period.

SHA-256 and 8 digits are both in the RFC and both better. They are not an
option, because the `algorithm` and `digits` parameters of the `otpauth://`
URI are ignored by most authenticator apps, which assume SHA1/6/30 whatever the
URI says. Choosing the stronger parameters produces an enrolment that silently
generates codes the server rejects, and the user blames the clock.

The weakness is not meaningful here. SHA1's collision problems do not apply to
HMAC-SHA1, and a 6-digit code's 10⁻⁶ guess probability is handled by rate
limiting, not by digit count.

## Skew and replay

Accept the step before, the current step and the step after: a 90-second
window, which is what tolerating clock drift in both directions costs.

Then store the last accepted step on the enrolment and **reject any code whose
step is less than or equal to it**. Without that, a code shouted across a room,
read off a shoulder or captured from a terminal's scrollback is reusable for up
to 90 seconds. The window makes the feature work; the high-water mark is what
makes a code single-use. Both are required, and the second is the one that gets
left out.

## Secret storage: encrypted, with no plaintext fallback

A password is verified by hashing the input and comparing, so the stored form
can be one-way. A TOTP secret has to be fed into HMAC on every verification, so
it has to be recoverable. Hashing is not available.

The secret is sealed with AES-256-GCM under a key from configuration
(`auth.totp_key` / `TIX_AUTH_TOTP_KEY`), 32 bytes, base64. The nonce is random
per secret and stored beside the ciphertext. The user's ID is passed as
additional authenticated data, so a ciphertext moved from one row to another
fails to open rather than enrolling the attacker's phone against another
account.

**When no key is configured, enrolment fails.** It does not fall back to storing
the secret in the clear. A fallback is the kind of thing that is chosen once
under time pressure and then runs in production for two years, and the failure
is silent: the feature appears to work and the property it exists for is gone.

The cost is honest and goes in the deployment docs: the key is as load-bearing
as the database. Lose it and every enrolled secret is undecryptable. That is
survivable, because the escape is the same one a lost phone uses, admin reset
plus re-enrolment, but it has to be known in advance rather than discovered.
`tix doctor` reports whether a key is set, whether any enrolments exist, and
flags the combination of enrolments present with no key as the state it is.

## Recovery codes are hashed with SHA-256, not argon2id

Ten codes, 128 bits of entropy each from `crypto/rand`, rendered base32,
displayed exactly once.

argon2id exists to make guessing a *low-entropy* human-chosen secret expensive.
A 128-bit random code is not guessable at any work factor, so the stretching
buys nothing. It costs something real: verification has to try the submitted
code against every unused hash, and ten argon2id comparisons at the parameters
tix uses for passwords is most of a second of CPU on a path an attacker can
call repeatedly.

So: SHA-256, compared in constant time. The reasoning is written down here
because "we used a fast hash for a credential" reads like a bug to a reviewer
who does not know the codes are machine-generated, and the next person to touch
this should find the argument rather than re-derive it or "fix" it.

A used code is marked used, not deleted, so an audit entry can say which one.

## One verifier, three surfaces

`internal/auth` gains the verifier. It is not duplicated per transport, and it
is not reachable from `internal/client`: the browser, `tix login` and `tix ssh`
all mint a session through `service`, and the check lives on that path.

What differs per surface is only *where the prompt is drawn* and *which flag
decides whether it is drawn at all*:

| Surface | Flag | Prompt |
| --- | --- | --- |
| Browser | `require_totp_web` | A second page after the password form |
| `tix login` | `require_totp_cli` | A terminal prompt, never an argument |
| `tix ssh` | `require_totp_ssh` | After the key is accepted, before the board |
| API token | none, exempt | none, it cannot be prompted |

The code is never a flag. `--code 123456` would put a live second factor in
shell history and in the process table, and the only reason to want it is
scripting, which is what API tokens are for.

For SSH the ordering matters: the key is verified first and TOTP second, so an
unenrolled key is refused without ever revealing that an account exists or
drawing a prompt.

## Requirement is a tenant policy, applied at session mint

Three booleans on `tenants`, defaulting false. The check runs when a session is
minted, against the tenant the session is for, and not on every subsequent
request: a minted session has already satisfied the policy, and re-checking per
request would mean a flag flipped mid-session logs everyone out, which is a
different feature.

**A user who is not enrolled, on a surface that requires it, enters enrolment
rather than being refused.** This is the difference between a flag an admin can
turn on and a flag that bricks the tenant. The admin who enables it is usually
not enrolled either.

The one case with no escape is a tenant requiring TOTP on every surface where
every admin has lost both device and codes. `doctor` warns when a tenant
requires TOTP on all three surfaces and has fewer than two enrolled admins,
which is the configuration that gets somebody locked out.

## Rate limiting

Five consecutive failures on one enrolment locks verification for fifteen
minutes. Counted on the enrolment, not the session or the address, because an
attacker controls both of those and controls neither the account nor the clock.
A success clears the counter. The lock is reported as a distinct error so the
user is told to wait rather than told their code is wrong.

## What is deliberately not in this change

- **WebAuthn / passkeys.** The right long-term answer and a much larger one:
  attestation, per-origin credentials, a browser API with no terminal
  equivalent. TOTP works identically on all three surfaces, which is what this
  product needs first.
- **Per-user requirement.** Policy is per tenant. A single user who must have
  TOTP when nobody else does is a role question, not a factor question.
- **Trusted devices / "remember this browser".** It is a cookie that turns the
  second factor off, and it wants its own threat model.
- **Fixing the one-user-one-actor defect.** Named above, scoped separately.
