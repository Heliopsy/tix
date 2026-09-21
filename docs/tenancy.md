# Tenancy

A tenant is the isolation boundary. Tasks, projects, workflows, fields, tags, users, tokens, webhooks, events and
audit entries all belong to exactly one, and nothing crosses.

A single-user install never has to think about this. One tenant called `default` is created on first use and
everything lands in it.

## Tenants

```sh
tix tenant create acme "Acme Corp"
tix tenant ls
tix tenant show acme
tix tenant edit acme --name "Acme Limited"
tix tenant rm acme
```

Keys must be valid and unique; a duplicate is exit 4.

## Members and roles

Membership is per tenant. The same person can hold different roles in different tenants.

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

User accounts are the one exception to "belongs to exactly one tenant": a user row is global so that the same
address can hold a membership in several tenants, and it is the actor row sharing its identifier that binds it to
one. Listing users therefore filters by actor rather than by a `tenant_id` column. The listing pages the global
ordering until this tenant's page is full, so the cursor it hands back always names a user the caller can see; a
cursor is base64 of a sort key and an identifier, not a secret, and one taken from the unfiltered page would have
named another tenant's user and their email address.

Tokens carry explicit scopes instead of a role, and may additionally be pinned to one project with
`tix token create ... --project infra`. See [agents.md](agents.md) for the scope vocabulary.

Pinning is confinement, not a hint. The policy in `internal/authz` holds an allow list of the operations whose
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

This layer is narrower than the one above it, and it is worth being exact about where it stops. The dialect
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

Name it once with a context instead of repeating the flag:

```sh
tix ctx add acme --db sqlite://~/.local/share/tix/tix.db --tenant acme --use
```

Against a remote server the tenant comes from the credentials and the hostname, not from the client: a token
authenticates within the tenant it was minted in, and the `Host` header selects the tenant before any handler
runs. A client cannot ask a server for a tenant it does not hold a token for.

## Related

- [configuration.md](configuration.md) for contexts and the precedence rules
- [deployment.md](deployment.md) for running the server behind a proxy that preserves `Host`
- [scaling.md](scaling.md) for the PostgreSQL engine
