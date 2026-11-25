# Design: a hard second factor policy

## Behaviour before this change

With `require_totp_<surface>` on and a person holding no confirmed factor:

| Surface | What happened |
| --- | --- |
| Browser sign-in | Enrolled inside the sign-in (ticket, QR, code), session minted after |
| A live session | Kept working: the flag is read only when a session is minted |
| `tix login` | Enrolled on a terminal; without one refused as "no terminal" (exit 6) |
| `tix login --ssh-key` | Same as `tix login` (it reads the CLI flag) |
| SSH board | Enrolled in the terminal before the board; a command without a pty refused |
| Person-owned API token | Worked: tokens are exempt from every requirement |
| Agent token | Worked |
| MCP | Worked: it runs on a token |
| Tenant switch | Refused, `second_factor_required` |

Gaps: sessions minted before the flag, sessions minted on another surface whose flag is off (a session is
not bound to the surface it was minted on), every person-owned token including the one `tix login --tenant`
mints, MCP, and the live sessions of somebody whose factor an administrator cleared.

## Decisions

### A policy beside the flags, not a fourth flag

`mfa_policy: off | required` is a different kind of thing from the flags: it is evaluated on every request,
not at sign-in, and it reaches tokens. Folding it into the flags would change what an existing tenant's
flags do the day it upgrades, including refusing its people's tokens. So the flags keep their meaning and are
not migrated. `required` means every surface; per-surface control of the hard gate would leave exactly the
gaps the owner asked to close.

### One gate, in the service

The gate runs inside `authorize` and `authorizeGlobal`, the single enforcement point every listing and
mutating method already starts from, plus the three methods that resolve the caller without it
(`GetActor`, `ListActors`, `RotateToken`). Every surface reaches the service, so the browser, the API, MCP,
the terminal interface over SSH and the CLI are gated by the same code. What stays open is exactly the set of
methods that never call it: `WhoAmI`, the caller's own `TOTP*` operations, `Logout`, `GetAccountLog`,
`ListSessions`/`RevokeSession`, `GetTimeSettings`/`SetTimePrefs` and `SwitchTenant`.

The gate reads the tenant first and stops there when the policy is off, so a tenant without it pays one
primary-key read per authorization. It is not cached: a cache would have to be invalidated across processes
when a factor is enrolled or cleared.

### Who is exempt

An actor with no person account behind it: agents, service actors, the machine-local actor and the system.
There is nobody to enrol. A token belongs to whichever actor it was minted for, so a token minted for a person
is gated with that person, and one minted for an agent is not.

### The grace

`mfa_required_at` is stamped when the policy turns on and cleared when it turns off. The gate applies from
`mfa_required_at + mfa_grace`. Changing the grace moves the deadline without restarting it. The deadline is
the tenant's, not each person's: somebody joining after it meets the gate at once, which on the web is the
enrolment screen and nothing worse.

### The browser signs in to enrolment

Under the policy the browser no longer enrols inside the sign-in. It mints the session and lets the gate send
the reader to `/settings/totp`. One path then serves a new sign-in, a session that was live when the policy
went on, and a session whose factor an administrator cleared. The session is not re-minted after enrolment:
the gate reads the enrolment, not the session, so the same session works the moment the code is confirmed.
It still records that its sign-in passed no factor, so a later switch to a tenant requiring one asks for a
code.

### The CLI keeps enrolling on a terminal

The command line already has an enrolment flow inside `tix login`, so a terminal keeps it. Without a terminal
there is nobody to scan a code, and the refusal is the gate's own: forbidden, exit 5, reason
`second_factor_enrollment_required`, naming the web enrolment screen. `tix user totp enrol` works for anyone
holding a credential, because the gate leaves it open.

### Lockout guard

Turning the policy on is refused when the caller is a person with no confirmed factor. The alternative, walking
them through enrolment first, already exists: every surface's enrolment is open to them. Refusing keeps the
rule in one place. A caller with no person account (the machine-local actor) is not refused, since the gate
does not apply to it either.

### Audit

The policy change is the tenant update entry, which already carries the whole tenant before and after; its
metadata names the new `mfa_policy` when it changed. Gate refusals are recorded as `user.totp.gate` at most
once per person, tenant and hour in each process, written outside the refused request's work. Enrolment is the
existing `user.totp.confirm`. Each carries the access channel it came through.
