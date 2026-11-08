# Tenancy

A tenant is the isolation boundary. Tasks, projects, workflows, fields, tags, users, tokens, webhooks, events and
audit entries all belong to exactly one, and nothing crosses.

A single-user install never has to think about this. One tenant called `default` is created on first use and
everything lands in it.

## Tenants

```sh
tix tenant create acme "Acme Corp"
tix tenant create acme --name "Acme Corp" --admin ada@example.com --admin bob@example.com
tix tenant ls
tix tenant show acme
tix tenant edit acme --name "Acme Limited"
tix tenant rm acme
```

Keys must be valid and unique; a duplicate is exit 4. Creating a tenant needs a [global admin](#global-admins).

### First admins

A tenant created with no members is one nobody can sign in to, since every other call acts on the caller's own
tenant. So creation takes its first admins: each `--admin EMAIL` (or each entry of `admins` on
`POST /api/v1/tenants`) becomes a user holding the admin role in the new tenant and in no other. A password left
out is generated, 24 characters from `crypto/rand` over an alphabet without the look-alikes 0, O, o, 1, l and I,
and returned once:

```console
$ tix tenant create test --name Test --admin villy@example.com
...
these passwords are shown once and cannot be retrieved again
EMAIL              HANDLE  PASSWORD
villy@example.com  villy   xxxxxx-xxxxxx-xxxxxx-xxxxxx
```

`-o json` carries the same in `initial_credentials`, as the API does. Nothing stores the password in plain text,
and the audit entry and event for each new user record the account and `credential: generated`, never the value.
A lost one is replaced by setting a new password (`tix tenant user edit KEY ID --password ...`).

The handle comes from the part of the address before `@` unless one is given over the API. An address named twice,
two admins that would share a handle, a supplied password under 12 characters, or more than 10 admins are refused
before anything is written. An address that already has an account becomes an admin of the new tenant with the
password and second factor it has: nothing is generated for it, and supplying a password for it is refused.
See [One person in several tenants](#one-person-in-several-tenants).

The tenant and the record of its creation are written in the caller's tenant; the builtin workflow and the admins
in one transaction in the new tenant. If that second transaction fails, the tenant is removed again, with a
`tenant.delete` audit entry carrying `reason: setup_failed` beside the creation, so a retry with the same key
works.

## Global admins

A global admin administers the installation rather than one tenant: they create tenants, list, rename and delete
any tenant, and manage the users of any tenant. Every other tenant admin stays confined to their own tenant.

```sh
tix --db /var/lib/tix/tix.db global-admin grant ada@example.com   # the first one, from the server's shell
tix global-admin ls
tix global-admin grant bob@example.com
tix global-admin revoke bob@example.com
tix tenant user ls acme
tix tenant user create acme carol@example.com --role admin --password '...'
tix tenant user edit acme 01J0000000000000000000 --role member
tix tenant user rm acme 01J0000000000000000000
```

It is a flag on the user, not a role: roles are per tenant. Who counts as one:

- **A shell holding the database.** A command with `--db` (or a local store) and no `--token` stands as a global
  admin, because whoever can open the file already holds every row. This is how the first one is granted.
- **A signed-in global admin**, in the browser or with `tix login`. The flag is read on every call, so a revoke
  takes effect on live sessions at once.
- **A token naming `global:admin`** whose actor is a global admin at the time of the call. `*` does not carry
  `global:admin`, so no token minted before global admins existed gained any authority. Only a global admin may
  mint a token with it, and only for an actor who is one: `tix token create ops --scope '*' --scope global:admin`.

The last global admin cannot be revoked (exit 6, `precondition_failed` with `reason: last_global_admin`). Grants
and revokes are recorded as `global_admin.grant` and `global_admin.revoke` in the granter's tenant.

**Upgrading.** Before this, any tenant admin could create a tenant. Now only a global admin can, so grant the
first one from the server's shell before relying on tenant creation over the API or the browser.

In the browser, a global admin gets a Global admin entry under Admin: every tenant with create, rename and delete,
the passwords a creation generated shown once with a copy button each, the global admins, and each tenant's users.
Over MCP the global admin tools are offered only to a token naming `global:admin` whose actor is one; see
[mcp.md](mcp.md).

### The trash

A tenant keeps deleted tasks in a trash by default, where `tix task restore` and the browser's Trash bring
them back. A tenant admin can turn it off:

```sh
tix tenant edit acme --trash off       # deleting a task is permanent from now on
tix task trash                          # enabled, and how many tasks it holds
tix tenant edit acme --trash on        # back to restorable deletes
```

The same setting is the Trash choice on the browser's Tenant screen and `trash_off` on
`PATCH /api/v1/tenants/{ref}`. With it off, every delete is permanent on every surface, as if `--hard` were
given: `tix task rm` prints a note on stderr saying so, the browser asks before each delete instead of
offering Undo, and the Trash entry leaves the sidebar. `tix task restore`, `tix task ls --trashed` and
`is:trashed` are refused with exit 6 (`422 precondition_failed` with `"reason": "trash_off"` over the API),
and the bookmarked trash address shows that refusal in its filter bar.

Turning the trash off does not empty it. Tasks deleted while it was on stay where they are, out of reach,
and `tix task trash` and the Tenant screen count them; turning it back on makes them listable and
restorable again. Emptying the trash on the same click would make the safe-looking choice the one that
cannot be taken back. Emptying the trash after a number of days is a [housekeeping](configuration.md#housekeeping)
rule, off unless a tenant admin sets it, and every housekeeping rule pauses while the trash is off, so
what it holds then is kept as promised. Changing the setting writes one `tenant.update` audit entry and one
`tenant.updated` event in the same transaction.

## Members and roles

Membership is per tenant: a role says what an account may do in the tenant it belongs to.

```sh
tix user create alice@example.com --handle alice --display-name "Alice" --role member
tix member add 01J0000000000000000000 --role admin
tix member ls
tix member rm 01J0000000000000000000
```

| Role | Carries |
| --- | --- |
| `viewer` | `task:read`, `project:read`, `workflow:read`, `event:subscribe`, `audit:read` |
| `member` | viewer, plus task write, transition, claim, `comment:write`, `artifact:write`, `export` |
| `admin` | `*` |

User accounts are the one exception to "belongs to exactly one tenant": an account is global, with one email,
one password and one second factor, and a person has an actor in each tenant they belong to, linked to the
account. Listing users therefore filters by actor rather than by a `tenant_id` column. The listing pages the
global ordering until this tenant's page is full, so the cursor it hands back always names a user the caller
can see; a cursor is base64 of a sort key and an identifier, not a secret, and
one taken from the unfiltered page would have named another tenant's user and their email address.

### Invites and first passwords

An administrator has two ways to bring somebody in, on the Users screen or from the command line.

**Set a password and require a change.** `tix user create EMAIL --password - --require-password-change`
(the browser's create dialog ticks the box by default). The first password sign-in with it is answered with
a `password_change` challenge rather than a session, on every surface: the browser shows a Choose a new
password page, `tix login` asks for the new password twice on a terminal, and `POST /api/v1/auth/login`
expects the same email and password again with `new_password`. The new one must be at least 12 characters
and differ from the old. Only that change clears the flag, recorded as `user.password_change`; an
administrator can also set or lift it later with `tix user edit ID --require-password-change[=false]` or the
Manage dialog. API tokens and SSH key sign-in are not passwords and are not affected.

**Send an invite link.**

```sh
tix user invite ada@example.com --role member --expires 3d
tix user invite ls
tix user invite reissue 01J0000000000000000000
tix user invite revoke 01J0000000000000000000
```

The account, its actor and its membership are created at once, with no password, so the person can be
assigned work before they accept. The answer carries a single-use link, `https://HOST/invite/tix_inv_...`,
shown once. It expires after 7 days unless `--expires` says otherwise, between one hour and 30 days.
Opening it shows a page to choose a password and a display name, and accepting signs the person in,
second factor rules included. An address of this tenant's user who holds no membership is granted one and
keeps their password, which they confirm on the page instead. An address that belongs to a member who can
already sign in is refused. An address of a person in another tenant gets an invite too; see below.

tix sends no email: copy the link and send it yourself. It is built from the scheme and host the request
reached tix at, as calendar feed addresses are, so create it through the address people use. Run against a
database directly, the command prints only the path.

Only the SHA-256 hash of the secret is stored, it never appears in the audit log, events or the request log,
and an unknown, expired, used or revoked link all answer the same "this invite link is not valid" message.
Re-issuing stops the old link at once; revoking stops it and leaves the account without a password until
somebody re-issues it or deletes the account.

### One person in several tenants

One person, with one email, one password and one second factor, can belong to several tenants, with a role in
each. They sign in to each tenant at its address with the same credentials, and act there as their actor in
that tenant, which has its own handle.

A global admin adds an existing account to a tenant directly, without asking the person:

```sh
tix tenant user add test villy@example.com --role admin
```

The browser offers the same as Add an existing person on a tenant's users screen in the Global admin area,
and `POST /api/v1/tenants/{ref}/members` on the API.

A tenant admin cannot add somebody else's account to their tenant. `tix user create` with such an address is
refused as a conflict. `tix user invite` sends an invite instead, and the person holds no membership here
until they accept it with their own password, and with a code when their account has a second factor. An
account of another tenant that has no password yet cannot be invited, because accepting would set its
password and whoever holds the link would then hold the account everywhere; a global admin adds it instead.

An administrator of one tenant must not be able to take over a person who also works in another, so for such
a person a tenant admin who is not a global admin cannot set the password, require a password change or clear
the second factor; the answer is forbidden with reason `account_in_other_tenants`, naming no other tenant.
Only the person or a global admin can. A tenant admin keeps those powers over people who are members of
their tenant only. Disabling and removing such a person work in this tenant alone: their sessions, tokens and
SSH keys here stop at once, and their other tenants are untouched. Removing a person from their last tenant
deletes the account, as before. A disabled account stops working in every tenant at once, because each
request checks it.

### Account history

Every change to an account is recorded in its own history, in the same transaction as the change: password
set, password change required or lifted, second factor enrolled or cleared, recovery codes issued or used,
account disabled or enabled (for the account, or in one tenant), global admin granted or revoked, and
memberships added, changed or removed. Each entry names the tenant it was made from, who made it, whether as
a global admin, and the access channel. It never holds a password, hash, code or token.

The person reads it on the Your account's history screen linked from Settings (`/account`), with
`tix account log`, or `GET /api/v1/account/log`; a token needs the `account:read` scope, which `*` does not
carry and which a person can only mint for themselves. A global admin reads anybody's from the Global admin
area, with `tix global-admin account-log EMAIL`, or `GET /api/v1/accounts/{email}/log`. A tenant admin never
reads it, because it names the person's other tenants. A deleted account's history is kept.

Tokens carry explicit scopes instead of a role, and may additionally be pinned to one project with
`tix token create ... --project infra`. See [agents.md](agents.md) for the scope vocabulary.

A pinned token is confined to its project. The policy in `internal/authz` holds an allow list of the operations whose
subject belongs to a single project: reading, creating, updating, transitioning, claiming and deleting tasks,
reading and writing projects, reading workflows, and writing fields, comments and artifacts. A pinned token may
perform those, and only inside its own project. Every other operation is refused outright, because its subject
belongs to the tenant rather than to a project and there is nothing to confine it to: exporting and importing,
reading the audit log, subscribing to the tenant event stream, administering tenants, users, tokens, webhooks,
sync sources and retention, and rewriting workflow definitions. That includes minting tokens, so a pinned token
cannot mint itself an unpinned one.

Reading the audit log is the clearest case. An audit entry records the tenant, the actor and the subject, but
not a project, so there is no column to filter on; a pinned token is refused the whole listing rather than
handed all of it.

### Switching tenant

A session belongs to one tenant. On a host that serves several tenants (one no domain claims, or one whose
domain a global admin marked shared, see [Domains](#domains)), the tenant is taken once per request, before
any credential is examined:

1. The `X-Tix-Tenant` header, when sent, names it. A tenant that has an address of its own, a domain not
   marked shared, is refused here: its sessions live at that address.
2. Otherwise a session cookie or API token picks the tenant it was minted in, and is then checked inside that
   tenant as anywhere else. A credential of a tenant with an address of its own is ignored here.
3. With neither, the host serves its own tenant: the domain's, or the server's default.

A host bound to one tenant keeps the strict rule: `X-Tix-Tenant` must name that tenant or be absent, and a
credential of another tenant is refused.

Signing in on a shared host goes by the account. When the address is not a member of the host's own tenant,
the sign-in goes to the first tenant, by key, it belongs to and that this host serves. After the password and
any second factor, a person with more than one tenant here lands on Choose a tenant (`/switch`).

In the browser, a person in more than one tenant has the current tenant's name as the first entry of the
sidebar. It opens a dialog of their tenants with the role in each: the current one marked, a Switch button for
each tenant this host serves, and a link to the sign-in page of a tenant served at its own address. Without
scripting the entry is a link to `?dialog=tenants`, which draws the same list in place. The dialog lists only
the person's own memberships; a global admin reaches other tenants from the Global admin area, which the
dialog links to.

Switch posts to `/switch` (`POST /api/v1/auth/switch` with `{"tenant": "KEY"}` on the API). It mints a session
for the person's actor in the target and ends the one presented. When the target requires a second factor on
the web and the current session was signed in without one, a code is asked for first; an account with no
second factor set up is refused with reason `second_factor_required` and sets one up under Settings. The
switch is recorded in the target as `user.login` with method `switch`, and nothing is recorded in the tenant
left. Only a browser session switches: a token stays in the tenant it was minted for.

`GET /api/v1/whoami` reports `account_id`, `email`, `tenant_key` and `tenant_name`, and, for a session or a
token carrying `account:read`, `memberships`: each tenant's `tenant_key`, `tenant_name`, `role`, `actor_id`,
`handle` and `host`, the tenant's own address or empty when it is served on a shared host alone.

## Domains

A tenant can own hostnames. When a request arrives at `tix serve`, the `Host` header is resolved to a tenant, and
the request is scoped to it before any handler runs.

```sh
tix domain add tix.acme.example.com
tix domain add tix.acme.example.com --cert-mode file --cert /etc/tix/acme.crt --key /etc/tix/acme.key
tix domain ls
tix domain rm tix.acme.example.com
```

`--cert-mode` is `none` (the default) or `file`. Mapping a hostname that another tenant already claims is exit 4.

Resolving a hostname to a tenant reads across tenants, because it runs before any credential exists. That is an
in-process call from the server's own middleware. The same operation is also reachable as
`GET /api/v1/domains/{hostname}`, where a credential does exist, and there it answers only for the tenant the
request already resolved to; a hostname belonging to another tenant is reported as not found, exactly as an
unmapped one is.

A host that maps to no tenant falls back to the server's default tenant, which `tix serve` sets to the tenant of
the database it opened. That is what makes a single-tenant deployment behave sensibly on `127.0.0.1` with no
domain configured at all. A server assembled with no default answers an unmapped host as not found.

## How isolation is enforced

Isolation is structural rather than a `WHERE` clause somebody has to remember. Four layers, each of which would
have to fail independently:

**The scoped query builder.** Every statement is built by `internal/store/sql`, which requires a `TenantScope` and
has no API for producing an unscoped statement against a tenant-owned table. A query that forgot its tenant does
not compile into existence.

**A lint rule.** `forbidigo` in `.golangci.yml` rejects `Query`, `QueryRow` and `Exec`, with or without the
`Context` suffix, on a raw `*sql.DB`, `*sql.Tx` or `*sql.Conn`. Taking a handle out of the store and querying it
directly fails the build.

This layer is narrower than the one above it. The dialect
packages run the builder's output through an executor interface, and every correct call site in
`internal/store/sqlite` and `internal/store/postgres` goes through that interface; forbidding it would flag some
sixty sanctioned call sites and prove nothing, so the rule does not try. What the rule cannot see is a query
string that never came from the builder. The builder is still the first layer; the lint is a backstop against
reaching past it for a connection, not a proof that every statement is scoped.

The exclusions are narrow and each carries its reason next to it in `.golangci.yml`:

| Path | Why |
| --- | --- |
| `internal/store/sql/` | The builder itself, the one place raw calls belong |
| `internal/store/migrations/` | Migrations create the tenant-owned tables; there is no scope over a table that does not exist yet |
| `internal/store/postgres/schema.go` | DDL, the RLS policies themselves, and monthly partition maintenance act on the schema, not on rows |
| `internal/store/{postgres,sqlite}/tx.go` | Transaction lifecycle, plus `set_config` and `SET LOCAL ROLE`; these install the scope every later statement runs under |
| `internal/store/postgres/listen.go` | `LISTEN` on a dedicated connection carries a channel name, no rows |
| `internal/store/sqlite/sqlite.go` | `PRAGMA journal_mode` at open, before any tenant is known |
| `*_test.go` | Tests build ad-hoc unscoped queries on purpose, to prove the other layers hold |

**Request scoping.** In the HTTP layer, the tenant is resolved from the `Host` header before authentication
proceeds and is carried in the request context. Handlers receive a scope, not a tenant identifier they could
mistype. A token authenticates within one tenant; a token minted in one tenant cannot address another.

**Row-level security, on PostgreSQL.** Every tenant-owned table has RLS enabled and forced, with a policy
comparing `tenant_id` to `current_setting('tix.tenant_id')`. Each transaction sets that value with
`set_config(..., true)`, so it is transaction-local and cannot leak onto the next transaction that reuses the
connection. Migration creates an unprivileged `tix_app` role and every transaction enters it with
`SET LOCAL ROLE`, because forcing RLS covers the table owner but not a superuser or a role with `BYPASSRLS`.
Give the application a plain login role:

```sql
CREATE ROLE tix LOGIN PASSWORD '...';
CREATE DATABASE tix OWNER tix;
```

The first three layers apply to both engines. The fourth means that on PostgreSQL, a bug in the application layer
still does not produce a cross-tenant read, because the database itself refuses.

There is an isolation test suite: `just leak`. It starts a PostgreSQL container, because layer four only exists
on PostgreSQL and a run without one would skip it in silence, and it runs verbosely so the suite shows what it
proved rather than only that it exited zero.

## Choosing a tenant

Against a local database, `--tenant`, `TIX_TENANT`, the `tenant` configuration key and a context's `--tenant` all
select which tenant a command works in. The flag wins, then the environment, then the file.

```console
$ tix tenant create acme "Acme Corp"
$ tix --tenant acme project create web Web
$ tix project ls -o json | grep -o '"key":"[^"]*"'
"key":"default"
$ tix --tenant acme project ls -o json | grep -o '"key":"[^"]*"'
"key":"web"
$ TIX_TENANT=acme tix project ls -o json | grep -o '"key":"[^"]*"'
"key":"web"
```

`tix tenant use` makes the choice stick, so the flag is for exceptions rather than for every command:

```console
$ tix tenant use acme
reached tenant acme
$ tix project ls -o json | grep -o '"key":"[^"]*"'
"key":"web"
$ tix --tenant default project ls -o json | grep -o '"key":"[^"]*"'
"key":"default"
```

The key is checked before it is written, by opening the target tenant and asking it who you are. This matters
because an actor is bound to exactly one tenant, so from inside `default` a tenant named `acme` and a
tenant that was never created are indistinguishable: `tix tenant ls` returns your own tenant and nothing
else, by design. Writing an unreachable key would leave every later command failing with an error that names
the wrong problem. Pass `--force` for the case where the tenant genuinely does not exist yet.

With a context current, the key is stored on that context rather than at the top level, because a context is
what pins the database or server the tenant lives in:

```sh
tix ctx add acme --db sqlite://~/.local/share/tix/tix.db --tenant acme --use
tix ctx use acme          # switches database and tenant together
tix tenant use other      # changes only the tenant, on the current context
```

Nothing else in the configuration file is touched: selecting a tenant writes the one key, so a database path
you were resolving from `XDG_DATA_HOME` or `TIX_DATABASE_DSN` keeps resolving from there afterwards.

Inside the terminal interface, `tix tui --tenant acme` starts in that tenant, and `tix tenant use` before
launching it has the same effect. `T` opens the tenant view mid-session, which shows the tenant in force and
takes a key to switch to. It asks for a key rather than offering a list, for the same reason `tix tenant use`
checks one by opening it: the actor cannot see any tenant but its own, so there is no list to draw. The key is
checked by opening a connection pinned to it and asking who you are there; a key that cannot be reached is
refused and the session stays where it was.

A switch inside the interface lasts for the session. Everything loaded from the previous tenant is dropped
(the project list, the open board, the event tail), because all of it belonged to that tenant, and the session
lands back on the project list of the new one. A lease this session was holding goes with it: a lease is held
in the tenant it was taken in, so it is not released by walking away from it and will expire on its own. The
status bar says so when it happens. Use `tix tenant use` to write the choice down for later runs.

Against a remote server the tenant comes from the credentials and the hostname, not from the client: a token
authenticates within the tenant it was minted in, and the `Host` header selects the tenant before any handler
runs. A client cannot ask a server for a tenant it does not hold a token for.

## Related

- [configuration.md](configuration.md) for contexts and the precedence rules
- [deployment.md](deployment.md) for running the server behind a proxy that preserves `Host`
- [scaling.md](scaling.md) for the PostgreSQL engine
