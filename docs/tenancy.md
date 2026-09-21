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

Tokens carry explicit scopes instead of a role, and may additionally be pinned to one project with
`tix token create ... --project infra`. See [agents.md](agents.md) for the scope vocabulary.

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

A host that maps to no tenant falls back to the server's default tenant, which `tix serve` sets to the tenant of
the database it opened. That is what makes a single-tenant deployment behave sensibly on `127.0.0.1` with no
domain configured at all. A server assembled with no default answers an unmapped host as not found.

## How isolation is enforced

Isolation is structural rather than a `WHERE` clause somebody has to remember. Four layers, each of which would
have to fail independently:

**The scoped query builder.** Every statement is built by `internal/store/sql`, which requires a `TenantScope` and
has no API for producing an unscoped statement against a tenant-owned table. A query that forgot its tenant does
not compile into existence.

**A lint rule.** `forbidigo` in `.golangci.yml` rejects `(*database/sql.DB).Query`, `QueryRow` and `Exec`
anywhere outside `internal/store/sql`. A developer cannot reach around the builder without the build failing.

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

There is an isolation test suite: `just leak`.

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
