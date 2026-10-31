# The second factor

`tix` can require a time-based one-time code (TOTP, RFC 6238) in addition to a password or an SSH key.
It is off everywhere until somebody turns it on, and it is configured per tenant and per surface.

## The three surfaces

A second factor applies to the three ways a human proves who they are interactively:

| Surface | What the first factor is | Flag |
| --- | --- | --- |
| Browser | password | `--require-totp-web` |
| `tix login` | password | `--require-totp-cli` |
| SSH | an enrolled public key | `--require-totp-ssh` |

They are independent. A tenant can require a code in the browser and leave the SSH board on keys alone, or
the reverse:

```sh
tix tenant edit acme --require-totp-web
tix tenant edit acme --require-totp-ssh=false
```

SSH is worth calling out. A key alone is one factor: whoever holds the private key is in. With
`--require-totp-ssh` the listener asks for a code after the key is accepted and before the terminal
interface draws anything belonging to the tenant.

**API tokens are exempt and always will be.** A token is not an interactive sign-in, there is nobody present
to read a code off a phone, and a surface that accepted a code from a flag or an environment variable would
put a live second factor in shell history and in the process table. Unattended work uses
`tix token create`, which is scoped and revocable. That is the answer to "how do I automate this", not a
way around the factor.

## Enrolling

Enrolment is opt-in and belongs to the user, not the administrator:

```sh
tix user totp enrol      # prints the otpauth:// uri and a scannable code
tix user totp status
tix user totp codes      # replaces the recovery codes
tix user totp rm
```

The browser has the same thing under Settings, and the terminal interface over SSH offers it too.

Enrolment is per person, not per tenant. One credential set, one enrolment, one entry in your authenticator
app, however many tenants you belong to.

Confirming an enrolment issues **ten recovery codes**, shown once and stored only as hashes. Nothing can
produce them again. They are single use, and spending one is recorded in the audit log by which code it was.

Turning a requirement on does not lock anybody out. A user subject to it who holds no enrolment is taken
through enrolment at their next sign-in rather than refused, the administrator who turned it on first among
them.

## Over HTTP

The same operations are routes under `/api/v1`:

| Command | Route |
| --- | --- |
| `tix user totp status` | `GET /auth/totp` |
| `tix user totp enrol` | `POST /auth/totp`, then `POST /auth/totp/confirm` with the code |
| `tix user totp codes` | `POST /auth/totp/recovery-codes` |
| `tix user totp rm` | `DELETE /auth/totp` |
| `tix user totp clear <user-id>` | `DELETE /users/{id}/totp` |

A sign-in that needs a code takes two calls to `POST /auth/login`. The first carries the email, the password
and the `surface` (`web`, `cli` or `ssh`, with `cli` assumed when it is left out) and answers with a
`challenge` holding a `ticket`. The second carries that ticket and the code, and answers with the session.
`tix login` does exactly this, prompting for the code between the two. `POST /auth/totp/verify` proves a
code on its own and creates nothing; it exists for a surface whose first factor is a key rather than a
password. [api.md](api.md#second-factor) has the request and response bodies.

## The key

Secrets are sealed with AES-256-GCM under a key you configure:

```sh
TIX_AUTH_TOTP_KEY=$(head -c 32 /dev/urandom | base64)
```

or `auth.totp_key` in the configuration file. It is 32 bytes, base64, and it is validated at startup, so a
malformed key fails when the process starts rather than when somebody first tries to enrol.

**The key is as load-bearing as the database.** A secret has to be fed into HMAC on every verification, so it
cannot be hashed the way a password is; it is stored sealed, and the key that opens it lives in
configuration and never in the database. Back it up with the same seriousness, and keep it somewhere other
than beside the database, or the pair travels together and the sealing has bought nothing.

Losing it means **every enrolment has to be cleared and made again**. There is no fallback to a plaintext
column, deliberately: a fallback is the kind of thing chosen once under time pressure and then run for two
years, and its failure is silent, because the feature goes on looking like it works.

Without a key configured:

- enrolment is refused outright, naming `auth.totp_key`
- turning on a requirement is refused too, because the enrolment that would satisfy it cannot happen, so the
  requirement would turn away everybody not already enrolled
- `tix doctor` says so, and reports it as a failure rather than a warning if enrolments already exist, since
  those are unreadable until each one is cleared

## Rate limiting and replay

Five wrong codes lock verification for fifteen minutes. A locked account is told to wait, not told the code
was wrong, so the two cases are not confused with each other.

A code is single use. The acceptance window spans one step either side of now, which is what lets a drifting
clock still verify, and without a replay guard that same window would leave an observed code usable for
ninety seconds. The highest accepted step is recorded, and a code at or below it is refused.

## When somebody loses their phone

Recovery codes first. If those are gone too, an administrator holding the `totp:admin` scope clears the
enrolment:

```sh
tix user totp clear <user-id>
```

The browser offers the same from the user list, which also says who currently holds a factor. An
administrator can clear an enrolment and can see that one exists; no secret and no recovery code is ever
theirs to see.

If the tenant requires a factor, the next sign-in walks that person through enrolling again.

## Checking the configuration

```sh
tix doctor
```

reports whether a key is configured, how many accounts depend on it, and whether this tenant is in the
configuration that ends with nobody able to sign in: a factor required on all three surfaces with fewer
than two administrators holding one. A single enrolled administrator who loses both the device and the
codes leaves a tenant with no way back in.

`tix status` carries the same facts under `second_factor`, in every output format.
