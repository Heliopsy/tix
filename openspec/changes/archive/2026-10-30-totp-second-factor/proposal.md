# A second factor, per surface

## Why

`users` carries `password_hash` and nothing else. Every way into a tix account
is one factor:

- **Web and `tix login`** take a password. One factor, something you know.
- **`tix ssh`** takes an enrolled public key. One factor, something you have.
  The key is not a *second* factor of anything; there is no first one. A
  stolen laptop with an unencrypted key file is a tenant.

So a deployment serving several tenants over the network today has no way to
require more than a single credential from a human, and no way to say that the
browser needs more than the terminal does.

`sso_provider` and `sso_subject` exist on `users`, NULL and never read, as the
v2 SSO seam. That seam answers a different question: it delegates identity to
somebody else's IdP. It does not help a deployment that wants to keep local
accounts and add a factor to them, which is the common case for a small team
running one binary.

## What Changes

**TOTP (RFC 6238) becomes an optional second factor on the three interactive
sign-in paths, enrolled per user, required per tenant per surface.**

- **Enrolment is opt-in and belongs to the user.** Any user may enrol from the
  browser or from `tix user totp enrol`. Nobody is enrolled by default and no
  existing deployment changes behaviour until somebody asks for it.

- **Requirement is per tenant and per surface, not one switch.** A tenant admin
  can require TOTP in the browser and leave the SSH board on keys alone, or the
  reverse. Three independent flags: `web`, `cli`, `ssh`. All default off.

- **A user who is not enrolled on a surface that requires it is sent through
  enrolment, not refused.** Turning a flag on must not lock out the tenant that
  turned it on.

- **API tokens are exempt, permanently and explicitly.** A scoped PAT is what
  an agent authenticates with and it cannot be prompted. The spec says so
  rather than leaving it to be discovered, and `tix doctor` says so too: a
  tenant that requires TOTP everywhere and mints a long-lived token has not
  achieved what it thinks it has.

- **Recovery is two independent paths.** Ten single-use codes, shown once at
  enrolment, and a tenant admin who can clear another user's enrolment. Either
  alone has a failure mode that ends with a permanently locked tenant.

- **The secret is encrypted at rest under a server-held key.** A TOTP secret,
  unlike a password, has to be recoverable to be verified, so it cannot be
  hashed. A plaintext column would mean that reading the database file yields
  working second factors for every enrolled user. The key lives in
  configuration, never in the database, and enrolment is refused outright when
  no key is set rather than falling back to plaintext.

## What this does not change

- No-auth mode. A local file with no accounts stays a local file with no
  accounts.
- Direct `--db` access. TOTP guards *sign-in*, so it is worth stating plainly
  that it does not guard the database file: anyone who can open the file is
  already past every authentication mode tix has. This was true before and is
  unchanged, but a second factor is exactly the feature that invites the wrong
  assumption.
- The SSO seam. `sso_provider`/`sso_subject` stay NULL and unread.

## Open question, deliberately left to the design

Whether `actors` or `users` owns the enrolment. The design answers `users`, and
the reasoning matters beyond this change: it is the same question behind the
defect where one email cannot hold an identity in two tenants.
