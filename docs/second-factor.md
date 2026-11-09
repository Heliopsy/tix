# The second factor

`tix` can require a time-based one-time code (TOTP, RFC 6238) in addition to a password or an SSH key.
It is off everywhere until somebody turns it on. A tenant has two controls: the per-surface flags below,
which ask for a code at sign-in, and [the tenant policy](#the-tenant-policy), which refuses a person with no
factor everything but enrolment, on every surface and with every credential of theirs.

## The three surfaces

A second factor applies to the three ways a human proves who they are interactively:

| Surface | What the first factor is | Flag |
| --- | --- | --- |
| Browser | password | `--require-totp-web` |
| `tix login` | password, or an enrolled key with `--ssh-key` | `--require-totp-cli` |
| SSH | an enrolled public key | `--require-totp-ssh` |

They are independent. A tenant can require a code in the browser and leave the SSH board on keys alone, or
the reverse:

```sh
tix tenant edit acme --require-totp-web
tix tenant edit acme --require-totp-ssh=false
```

Over SSH, a key alone is one factor: whoever holds the private key is in. With
`--require-totp-ssh` the listener asks for a code after the key is accepted and before the terminal
interface draws anything belonging to the tenant.

**API tokens are never asked for a code, and always will be exempt from that.** A token is not an
interactive sign-in, there is nobody present to read a code off a phone, and a surface that accepted a code
from a flag or an environment variable would put a live second factor in shell history and in the process
table. Unattended work uses `tix token create`, which is scoped and revocable. Under the per-surface flags a
token works whatever its owner holds; under [the tenant policy](#the-tenant-policy) a token minted for a
person stops working until that person enrols, and an agent's token goes on working.

The flags are read when a session is minted and not again. A session that was live when a flag went on keeps
working, and a session minted on a surface whose flag is off is not bound to that surface. Closing those gaps
is what the policy is for.

## The tenant policy

```sh
tix tenant edit acme --mfa-policy required --mfa-grace 7d
tix tenant edit acme --mfa-policy off
```

`mfa_policy` is `off` (the default) or `required`, and `required` covers every surface. `mfa_grace` is how
long after the policy was turned on people without a factor may keep working; `0`, the default, gates at
once. The grace runs from the moment the policy turned `required` (`mfa_required_at` on the tenant), so
changing the grace later moves the deadline without restarting it. The deadline belongs to the tenant: a
person who joins after it meets the gate on their first request.

The same two settings are `PATCH /api/v1/tenants/{ref}` (and so the MCP tool), the Second factor section of
the Tenant screen in the browser, and the tenant editor (`e` on `T`) in the terminal interface. A tenant
administrator or a global admin may set them.

Once the grace is over, a person holding no confirmed factor is refused every operation with forbidden,
reason `second_factor_enrollment_required` (HTTP 403, exit code 5), with a session or with a token minted
for them, through the browser, the API, MCP, the CLI or the terminal over SSH. What stays open is exactly:

- enrolling (`tix user totp status|enrol`, `/settings/totp`, `GET|POST /auth/totp`, `POST /auth/totp/confirm`)
- who am I, their own account history, their own sessions and their own date settings
- signing out, and switching to another tenant they belong to

How each surface meets it:

| Surface | A person with no factor, after the grace |
| --- | --- |
| Browser sign-in | Signed in to a session that lands on `/settings/totp`, the enrolment screen |
| A live browser session | The next page goes to `/settings/totp`; an htmx or form request gets 403 with `HX-Redirect` |
| `tix login` | Enrols on the terminal, as below; without a terminal, refused with exit 5 naming `/settings/totp` |
| SSH | Enrolment in the terminal before anything is drawn; a command without a pty is refused naming `ssh -t` |
| Token minted for the person | 403 `second_factor_enrollment_required`, until they enrol; then it works again |
| Agent or service token | Not affected |
| Tenant switch into the tenant | Refused with `second_factor_required` |

**Agent tokens are exempt** because an agent has no person behind it and nobody could enrol for it. The test
is the account behind the token's actor, not the token: a token belongs to the actor it was minted for, and
`tix token create` without `--actor` mints it for you. A token minted for a person, including the one
`tix login --tenant` stores and any CI token a person minted for themselves, is that person's and is refused
with them. A token of an actor with no account behind it is not: the machine-local actor that
`tix token create` speaks as on the server's own database, or any actor `tix actor ls` lists as an agent.
Before turning the policy on, check which actor each automation's token belongs to.

During the grace nothing is refused. The browser shows a banner naming the deadline on every page,
`tix login` prints the deadline after signing in, and `whoami` and `tix user totp status` carry it
(`second_factor_due`, `enrollment_due`).

Turning the policy on is refused while you hold no factor yourself, so nobody can lock themselves out with
it: enrol first. It is refused too without [the key](#the-key). Under the policy nobody can remove their own
factor; an administrator clearing somebody's factor (`tix user totp clear`) puts that person straight back
behind the gate, on their next request. Global admins follow the policy of the tenant they are in, with no
exception.

`tix user ls --without-2fa` (`GET /users?without_totp=true`, the "Without a second factor" filter on the
Users screen, `/` on the people screen of the terminal) lists who still has to enrol; `tix user ls` shows
everyone's factor in its 2FA column.

The audit log records a change to the policy as `tenant.update`, whose metadata names `mfa_policy` and
`mfa_grace` when they changed; a refusal as `user.totp.gate`, at most once an hour for one person in one
process, so a retrying script does not fill the log; and an enrolment as `user.totp.confirm`. Each entry
carries the channel it came through.

## Enrolling

Enrolment is opt-in and belongs to the user, not the administrator:

```sh
tix user totp enrol      # prints the otpauth:// uri and a scannable code
tix user totp status
tix user totp codes      # replaces the recovery codes
tix user totp rm
```

The browser has the same thing under Settings, and the terminal interface has it on the account screen
(`$`), which draws the enrolment as a QR code when the terminal has colour and room for it and always as
the key and the URI. See [tui.md](tui.md#the-account-screen).

Enrolment is per person, not per tenant. One credential set, one enrolment, one entry in your authenticator
app, however many tenants you belong to.

Confirming an enrolment issues **ten recovery codes**, shown once and stored only as hashes. Nothing can
produce them again. They are single use, and spending one is recorded in the audit log by which code it was.

Turning a flag on does not lock anybody out. A user subject to it who holds no enrolment is taken
through enrolment at their next sign-in rather than refused, the administrator who turned it on first among
them. Under the tenant policy the browser signs them in to the enrolment screen instead, and the command line
and SSH enrol them in the terminal.

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

If the tenant requires a factor, the next sign-in walks that person through enrolling again. Under the tenant
policy their live sessions and tokens are confined to enrolment at once.

## Checking the configuration

```sh
tix doctor
```

reports whether a key is configured, how many accounts depend on it, and whether this tenant is in the
configuration that ends with nobody able to sign in: a factor required on all three surfaces with fewer
than two administrators holding one. A single enrolled administrator who loses both the device and the
codes leaves a tenant with no way back in.

`tix status` carries the same facts under `second_factor`, in every output format.
