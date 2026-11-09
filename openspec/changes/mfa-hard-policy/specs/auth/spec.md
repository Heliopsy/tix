## MODIFIED Requirements

### Requirement: Per-tenant, per-surface requirement

A tenant SHALL be able to require a second factor independently for the browser,
for the command line, and for SSH. The three settings SHALL be independent, and
SHALL all default to not required.

Each per-surface setting SHALL be evaluated when a session is minted, against the tenant
the session is for, and SHALL NOT be re-evaluated on each subsequent request. The tenant
second factor policy is evaluated on every request instead.

#### Scenario: Required on one surface only

- **WHEN** a tenant requires a second factor for the browser and not for SSH
- **THEN** a browser sign-in asks for a code and an SSH session with an enrolled key does not

#### Scenario: A flag changed mid-session

- **WHEN** an administrator enables a per-surface requirement while a user holds a live session
- **THEN** that session continues, and the requirement applies the next time a session is minted

### Requirement: API tokens are exempt

An API token SHALL NOT be asked for a second factor code on any surface, because a token
carries no interactive channel on which a code could be requested. Under the tenant second
factor policy a token minted for a person who holds no factor SHALL be refused like that
person's session; a token of an actor with no person account SHALL NOT be.

Diagnostics SHALL state this.

#### Scenario: A token authenticates under a full requirement

- **WHEN** a tenant requires a second factor on every surface and an agent presents a valid API token
- **THEN** the request is authenticated and no code is demanded

#### Scenario: Diagnostics name the exemption

- **WHEN** `tix doctor` runs in a tenant requiring a second factor
- **THEN** it reports that API tokens bypass the requirement

## ADDED Requirements

### Requirement: A tenant second factor policy

A tenant SHALL carry `mfa_policy`, `off` by default or `required`, and `mfa_grace`, a duration
defaulting to zero. `required` SHALL apply to every surface. The time the policy last turned
`required` SHALL be stored, and cleared when it turns `off`. The policy SHALL be settable by a
tenant administrator or a global admin through the CLI, the HTTP API and so MCP, the web
interface and the terminal interface.

#### Scenario: Setting the policy

- **WHEN** an enrolled tenant administrator runs `tix tenant edit acme --mfa-policy required --mfa-grace 7d`
- **THEN** the tenant reads back `mfa_policy: required`, `mfa_grace: 7d` and the time it was turned on

### Requirement: An unenrolled person is gated under the policy

While the policy is `required` and its grace has ended, every service operation SHALL be refused
to a person holding no confirmed second factor, as forbidden with reason
`second_factor_enrollment_required`, on every surface and whether they present a session or a
token they own. Enrolling, who-am-I, their own account log, sessions and date settings, sign-out
and tenant switching SHALL stay open.

#### Scenario: A live session meets the gate

- **WHEN** the policy turns `required` with no grace while an unenrolled person holds a session
- **THEN** their next request to list tasks is refused with reason `second_factor_enrollment_required`

#### Scenario: Enrolment stays open

- **WHEN** a gated person begins and confirms an enrolment with the same session
- **THEN** both succeed, and the session lists tasks afterwards

#### Scenario: An administrator clears a factor

- **WHEN** an administrator clears the factor of an enrolled person under the policy
- **THEN** that person's next request outside enrolment is refused with the same reason

### Requirement: Agents are exempt from the policy gate

An actor with no person account behind it, an agent, a service actor, the machine-local actor
or the system, SHALL NOT be gated by the policy, because nobody could enrol for it. A token
minted before its person enrolled SHALL work again once they enrol.

#### Scenario: An agent token under the policy

- **WHEN** an agent's token lists tasks in a tenant whose policy is `required`
- **THEN** the request succeeds

#### Scenario: A person's token before and after enrolment

- **WHEN** an unenrolled person's token is refused under the policy and the person then enrols
- **THEN** the same token succeeds

### Requirement: The policy grace

Until `mfa_grace` has passed since the policy turned `required`, an unenrolled person SHALL NOT
be gated, and who-am-I and the second factor status SHALL report the deadline. Changing the grace
SHALL move the deadline without restarting it.

#### Scenario: Inside and after the grace

- **WHEN** the policy turns `required` with a grace of 7d and an unenrolled person lists tasks after 1d and again after 8d
- **THEN** the first succeeds with the deadline reported, and the second is refused

### Requirement: Turning the policy on cannot lock out the caller

Turning the policy `required` SHALL be refused, as a failed precondition naming how to enrol,
when the caller is a person holding no confirmed factor, and when no second factor key is
configured. A caller with no person account SHALL NOT be refused for the first reason.

#### Scenario: An unenrolled administrator

- **WHEN** a tenant administrator with no factor sets `mfa_policy` to `required`
- **THEN** the change is refused and the tenant keeps `off`

### Requirement: Command line sign-in under the policy

`tix login` SHALL take an unenrolled person through enrolment on a terminal once the grace has
ended. Without a terminal it SHALL be refused with reason `second_factor_enrollment_required`,
exit code 5, naming the web enrolment screen. Inside the grace it SHALL warn with the deadline
and sign in.

#### Scenario: No terminal

- **WHEN** an unenrolled person runs `tix login` with no terminal in a tenant whose policy is `required`
- **THEN** it exits 5 naming `/settings/totp`

### Requirement: Gate refusals and policy changes are audited

A change to the policy SHALL be recorded in the tenant update entry with `mfa_policy` in its
metadata. A gate refusal SHALL be recorded as `user.totp.gate` at most once per person, tenant
and hour within one process. Each SHALL carry its access channel.

#### Scenario: Repeated refusals

- **WHEN** a gated person makes three refused requests within a minute
- **THEN** one `user.totp.gate` entry is recorded

### Requirement: Administrators can list people without a second factor

The user listing SHALL report each person's factor and SHALL accept a filter for people without
one: `tix user ls --without-2fa`, `without_totp` on the HTTP listing, a filter on the web Users
screen and in the terminal people view.

#### Scenario: Filtering

- **WHEN** an administrator lists users with `--without-2fa` in a tenant where one of two people is enrolled
- **THEN** only the unenrolled person is listed
