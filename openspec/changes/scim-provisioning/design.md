# Design

## Where the endpoints live

`/scim/v2` is mounted on the API router, behind the same middleware as `/api/v1`: request id,
forwarded headers, body limit, timeout, tenant resolution and authentication. The tenant is the one
the request resolves to by host, exactly as the API does. On a host that serves several tenants the
bearer token already names its tenant (the credential lookup the API uses), so a provider configured
with the public address and a tenant's token lands in that tenant. A `/t/{tenant}/scim/v2` prefix
was considered and dropped: nothing else in tix carries the tenant in the path, and it would give a
second way to name a tenant that has to agree with the host.

The protocol lives in `internal/scim`: the resource shapes, the filter parser, PATCH application and
the error schema. It turns a request into a call on `core.Provisioner` and nothing more. The rules
(who may provision, matching an existing account, roles, deactivation) live in `internal/service`.

`core.Provisioner` is deliberately not part of `core.Service`. Its operations are the SCIM protocol
and have no CLI, web or terminal binding to give them; a SCIM client is the one caller. They are not
in `api/openapi.yaml` either, so `tix mcp` offers no tool for them: SCIM is described by its own
`ServiceProviderConfig`, `ResourceTypes` and `Schemas`. The administration around it (enable,
disable, the token, group mappings, status) is ordinary `Service` surface with full parity.

## Credential

`scim:provision` is a new scope with three properties:

- **Named only.** Like `global:admin` and `account:read`, neither `*` nor the admin role implies it,
  so no existing token or session gains it.
- **Exclusive.** A token carrying it carries no other scope and no project. Minting one needs
  `tenant:admin`.
- **Confined.** The token verifier refuses such a token unless the request is under `/scim/v2`, so it
  cannot read `whoami`, the actor directory or anything else that needs only a credential. The check
  sits in the verifier, so it holds for the HTTP API, the browser and a CLI pointed at a database.

`tix tenant scim enable` creates the tenant's provisioning agent (an actor with handle `scim`) the
first time, and mints a token named `scim` for it when it has no live one, shown once.
`tix tenant scim token` revokes that agent's live tokens and mints a new one. The changes a provider
makes are therefore attributed to the `scim` agent, through the token name, and keep working when
the admin who set it up leaves. `tix token create NAME --scope scim:provision` also works and acts as
the caller, which means it stops when that person is disabled; the docs recommend the helper.

## On and off

`scim_config.enabled` is the switch, off until enabled. While it is off every SCIM endpoint answers
404 with a SCIM error, after authentication: an unauthenticated caller gets 401 and a credential
without the scope 403, whatever the switch. 404 was chosen over 403 because the endpoints, not the
caller, are what is missing, and a provider reports 404 as "not configured". Turning SCIM off keeps
the token, so turning it back on resumes the same provider configuration; rotating the token is how
an admin cuts a provider off.

## Users

A SCIM User is a person with a membership in the tenant. `id` is the person's tix user id in the
tenant (their actor). Provisioning metadata lives in `scim_users`, keyed by tenant and actor:

| SCIM attribute | tix |
| --- | --- |
| `id` | the user id in this tenant |
| `userName` | `scim_users.user_name`, unique per tenant ignoring case; the account email for a person never provisioned |
| `externalId` | `scim_users.external_id` |
| `emails` | the primary, else `work`, else first value is the account address at creation; kept in `scim_users.email` |
| `name.givenName`, `name.familyName` | `scim_users` |
| `displayName` | the person's display name in this tenant |
| `active` | not disabled in this tenant |
| `groups` | read only, from `scim_group_members` |

Creating a user needs an email: the primary email, or the `userName` when it is an address. If an
account with that address exists it is linked: its actor here is reused or created, a membership is
granted if it has none, and the `scim_users` row is written. A person already provisioned is a 409
`uniqueness`. Otherwise an account is created with no password. A provisioned person signs in with
SSO, or the admin issues an invite or sets a password. An account's address is not changed by a
later update; it is global across tenants and a provider of one tenant does not own it.

Listing walks the tenant's memberships ordered by actor id in the service and slices
`startIndex`/`count` there, so no query uses `OFFSET`. Filters on `userName`, `externalId`,
`emails` and `id` are point lookups.

ETags and `meta.version` are not implemented; `ServiceProviderConfig` says so. Bulk, sort and
change-password are unsupported. Unknown attributes are accepted and ignored, as providers send
extension attributes tix has no place for.

## Deactivation and deletion

`active=false` sets the person's tenant-level disabled mark (the same one an admin sets for one
tenant), and deletes their sessions in this tenant. Sign-in already refuses a disabled member on
every method. Their API tokens and SSH keys are not revoked: every credential check already asks
whether the actor is disabled, so they stop working while the person is inactive and work again on
reactivation. That is the recommended behaviour: a provider flaps `active` on licence and policy
changes, and revoking would force every reactivated person to re-issue keys. Tasks, comments and
history are untouched.

DELETE removes the membership, ends sessions and revokes tokens and SSH keys in the tenant (the
person is no longer part of it), removes them from SCIM groups and drops the `scim_users` row. The
account and the actor stay, so history still names them. The account is never deleted, even when this
was its only tenant, unlike `tix user rm`: deleting from a provider is often a reversible
unassignment.

## Groups

tix had no groups or teams. A SCIM Group becomes a small tenant entity, `scim_groups` with
`scim_group_members`, which exists to carry a provider's group memberships and nothing else in tix
reads it. The alternative, mapping a group straight onto a role without storing it, fails the
protocol: a provider reads a group back and patches its members, so it has to exist.

`scim_group_roles` maps a group display name, ignoring case, to a tenant role. A name is mapped
rather than a group id so an admin can map before the provider pushes the group. The rule for a
person provisioned over SCIM:

- in at least one mapped group: their role is the highest role mapped to those groups;
- in none: their role is left as it is, except that leaving their last mapped group (by a member
  change, a group deletion or the mapping being removed) drops them to `viewer`.

A person created over SCIM in no mapped group starts as `viewer`, the default for an account somebody
else created. People never provisioned over SCIM are never touched by a mapping.

## Audit

Every SCIM write goes through the service's mutation, so its audit entries and events are written in
the same transaction, with `via.kind` `scim` and the token's name and id. The HTTP layer sets that
kind for any request under `/scim/v2`. Actions: `scim.user.create`, `scim.user.link`,
`scim.user.update`, `scim.user.delete`, `scim.group.create`, `scim.group.update`, `scim.group.delete`
and `scim.role`; the administration actions are `scim.enable`, `scim.disable`, `scim.token`,
`scim.map` and `scim.unmap`. Membership and role changes also land in the person's account log.
`scim_config.last_activity_at` is touched by every provisioning write, and `tix tenant scim show`
reports it with the counts; `tix audit ls --via scim` lists the changes.

## Not done

- ETags, bulk, sort, `attributes` projection beyond `excludedAttributes=members`, the enterprise user
  extension, password changes over SCIM.
- SAML, a later change.
