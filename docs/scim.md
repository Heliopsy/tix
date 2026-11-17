# SCIM provisioning

SCIM 2.0 (RFC 7643 and RFC 7644) is how an identity provider such as Entra ID, Okta, Authentik or JumpCloud
creates, updates, deactivates and removes the people of an application. `tix serve` answers it, one tenant at a
time, so somebody who joins the company gets a tix account from the provider and somebody who leaves loses
access to the tenant without an admin remembering to do it.

SCIM is provisioning only. It does not sign anybody in. Sign-in with an identity provider over OIDC or
GitHub is a separate feature with its own settings ([sso.md](sso.md)), and SAML is planned for later. A person
the provider created has no password, so until single sign-on is set up they get in through an invite
(`tix user invite`) or a password an admin sets or resets.

## The endpoint

The endpoints are under `/scim/v2` on the address `tix serve` answers at, beside `/api/v1`:

| Route | |
| --- | --- |
| `GET /scim/v2/ServiceProviderConfig` | what tix supports |
| `GET /scim/v2/ResourceTypes`, `GET /scim/v2/ResourceTypes/ID` | `User` and `Group` |
| `GET /scim/v2/Schemas`, `GET /scim/v2/Schemas/ID` | the core User and Group schemas |
| `GET`, `POST /scim/v2/Users` | list and filter, create |
| `GET`, `PUT`, `PATCH`, `DELETE /scim/v2/Users/ID` | read, replace, patch, remove |
| `GET`, `POST /scim/v2/Groups` | list and filter, create |
| `GET`, `PUT`, `PATCH`, `DELETE /scim/v2/Groups/ID` | read, replace, patch, remove |

Requests and responses are `application/scim+json`; a body sent as `application/json` is accepted too.
Errors use the SCIM error schema (`urn:ietf:params:scim:api:messages:2.0:Error`) with `status`, `detail` and,
for a 400 or 409, `scimType`: `invalidFilter`, `invalidSyntax`, `invalidPath`, `invalidValue`, `mutability`,
`noTarget` or `uniqueness`. A method a resource does not take is 405.

The tenant is resolved the way the API resolves it ([tenancy.md](tenancy.md#choosing-a-tenant)). On a tenant's
own domain the host names the tenant. On a host that serves several tenants the bearer token does, because a
token belongs to the tenant it was minted in. Either way a provider is configured with one address and one
token, and there is no tenant in the path.

`tix tenant scim show` prints the endpoint to give the provider, built from `server.public_url` when it is set
and from the address of the request otherwise. The browser screen shows the same address with a Copy button.

## The token

The endpoints take one credential: an API token carrying the `scim:provision` scope. That scope is unlike the
others:

- **Named only.** Neither `*` nor the `admin` role carries it, so no existing token or session can provision.
- **Alone.** A token carrying it carries no other scope, no project and no claim requirement.
- **Minted by a tenant admin.** Anybody else asking for it is refused.
- **Confined.** The token is refused everywhere outside `/scim/v2`. It cannot read `whoami`, the actor
  directory or anything else, on the API, in the browser or from the CLI.

Without a credential a SCIM request is 401. A credential without the scope, an admin's session included, is 403.

`tix tenant scim enable` mints this token for you. `tix token create NAME --scope scim:provision` works too,
but that token acts as you, so it stops the day your account is disabled. The one `enable` mints belongs to the
tenant's `scim` agent and keeps working when the admin who set it up leaves.

## Turning it on

SCIM is off for every tenant until a tenant admin turns it on. There is nothing to set in the configuration
file.

```sh
tix tenant scim enable
```

The first time, this creates an agent with the handle `scim` and mints a token named `scim` for it, printed
once. Copy it into the provider straight away. Running `enable` again while that token is live mints nothing.
If a person already holds the handle `scim`, enabling is refused (exit 4) until they are renamed.

```sh
tix tenant scim show                 # on or off, endpoint, live tokens, counts, last change, mappings
tix tenant scim token                # revoke the scim agent's tokens and mint a new one, printed once
tix tenant scim disable              # every SCIM endpoint answers 404; the token is kept
tix tenant scim map-group Engineering --role member
tix tenant scim unmap-group Engineering
```

While SCIM is off every endpoint answers 404, which providers report as "not configured". Authentication still
comes first, so a missing token is 401 and a token without the scope is 403 whatever the switch says. Turning
it off keeps the token, so turning it back on resumes the same provider. To cut a provider off for good, run
`tix tenant scim token` and do not hand over the new one.

The same controls are on the SCIM provisioning screen in the browser, `/admin/scim` under Manage: the status
and endpoint, Turn SCIM on and off, Issue a new token, the group mappings, and the provider's recent changes.
Over HTTP they are `GET /api/v1/scim`, `POST /api/v1/scim/enable`, `POST /api/v1/scim/disable`,
`POST /api/v1/scim/token`, `PUT /api/v1/scim/groups` and `DELETE /api/v1/scim/groups/{group}`, each needing
`tenant:admin`. See [api.md](api.md#routes).

## Users

A SCIM User is a person with a membership in the tenant. Agents are not users here. Listing users lists every
person in the tenant, including people the provider never created; for those, `userName` is their account
email until the provider writes one.

| SCIM attribute | In tix |
| --- | --- |
| `id` | the person's user id in this tenant |
| `userName` | required, unique in the tenant ignoring case |
| `externalId` | kept as sent |
| `emails` | the primary address, else the `work` one, else the first; read back as one primary `work` email |
| `name.givenName`, `name.familyName` | kept as sent; `name.formatted` is built from them |
| `displayName` | the person's display name in this tenant, or the given and family name when none is sent |
| `active` | whether the person is enabled in this tenant; `true`, `false` and the strings `"True"`, `"False"` |
| `groups` | read only, the SCIM groups the person is in |
| `meta` | `created`, `lastModified`, `location` |

`userName` and `externalId` are at most 256 bytes. Attributes tix has no place for, the enterprise user
extension among them, are accepted and ignored.

### Creating a user

A new user needs an email: the one in `emails`, or the `userName` when it is an address.

- If an account with that address already has an actor in this tenant, it is linked: the person gets a
  `viewer` membership if they had none, and the provider's details are written. Linking a person the provider
  already created is a 409 `uniqueness`.
- If the address belongs to an account in other tenants only, the request is refused with 409. An admin of one
  tenant never adds somebody else's account to their tenant without that person accepting, so invite them
  with `tix user invite` first, then let the provider retry.
- Otherwise a new account is created with no password, a handle taken from the address, and the `viewer` role.

An account's email is global across tenants, so a later update from the provider never changes it. The address
the provider sends is kept with its SCIM record and read back in `emails`.

### Deactivating and removing

`active: false` disables the person in this tenant, the same mark an admin sets for one tenant, and ends
their sessions here. Their API tokens and SSH keys are kept but stop working while they are inactive, and work
again when the provider sets `active: true`. Their tasks, comments and history are untouched. Providers flip
`active` on licence and policy changes, and revoking keys on every flip would make each returning person issue
new ones.

`DELETE /scim/v2/Users/ID` removes the membership, ends the person's sessions, revokes their tokens and SSH keys
in this tenant and takes them out of every SCIM group. The account and the actor stay, so history still names
them, and the account is not deleted even when this was its only tenant: a provider deletes on an unassignment
that is often reversed. `tix user rm` is the way to delete an account.

## Groups and roles

A SCIM Group is a tenant group that exists to carry the provider's group memberships. tix has no other use for
it. Its `displayName` is required and unique in the tenant, ignoring case. Members are user ids of this tenant;
any other id is a 400. PATCH adds, removes and replaces members, including `remove` with a path such as
`members[value eq "ID"]`.

A group's display name can be mapped to a tenant role, `viewer`, `member` or `admin`, before or after the
provider pushes it:

```sh
tix tenant scim map-group "Engineering" --role member
tix tenant scim map-group tix-admins --role admin
```

For a person the provider created or updated:

- in at least one mapped group, their role is the highest role mapped to those groups;
- in none, their role is left as it is, except that leaving their last mapped group drops them to `viewer`.
  That happens when they are removed from it, when the group is deleted, or when its mapping is removed.

People the provider never touched are never changed by a mapping. Mapping a name again changes its role and
re-applies it.

## Filtering and paging

Filters take `eq` terms joined with `and`, the subset providers send to find a resource they are about to
create or update:

| Resource | Attributes |
| --- | --- |
| Users | `id`, `userName`, `externalId`, `emails`, `emails.value`, `emails[type eq "work"].value` |
| Groups | `id`, `displayName`, `externalId`, `members[value eq "ID"]` |

```text
GET /scim/v2/Users?filter=userName eq "ada@example.com"
GET /scim/v2/Groups?filter=displayName eq "Engineering"&excludedAttributes=members
```

Any other operator, `or`, `not` or attribute is a 400 `invalidFilter`. `startIndex` is 1-based and `count`
defaults to 100, at most 200. `excludedAttributes=members` leaves members out of groups, which providers ask for
when they only want to know a group exists.

## What is not supported

`ServiceProviderConfig` says so to the provider as well:

- ETags and `meta.version`.
- Bulk operations.
- Sorting.
- Changing a password over SCIM.
- The `attributes` parameter, and `excludedAttributes` other than `members` on groups.
- The enterprise user extension. Its attributes are accepted and ignored.

## Audit

Every change a provider makes is audited in the same transaction as the change, with the access channel `scim`
and the token's name and id ([audit-log.md](audit-log.md#the-access-channel)). Membership and role changes also
land in the person's account log.

| Action | When |
| --- | --- |
| `scim.user.create` | a new account was created |
| `scim.user.link` | an existing account was linked |
| `scim.user.update` | a user was replaced or patched |
| `scim.user.delete` | a user was removed from the tenant |
| `scim.group.create`, `scim.group.update`, `scim.group.delete` | a group changed |
| `scim.role` | a group mapping changed somebody's role |
| `scim.enable`, `scim.disable`, `scim.token` | an admin turned SCIM on or off, or issued a token |
| `scim.map`, `scim.unmap` | an admin mapped or unmapped a group |

```sh
tix audit ls --via scim
tix audit ls --via scim --action scim.user.delete
```

The SCIM screen lists the latest changes and links to the Activity feed narrowed to `scim`. `tix tenant scim
show` reports when the provider last changed anything.

## Setting up a provider

Each provider needs the endpoint and the token from `tix tenant scim enable`. The endpoint is the full address
including `/scim/v2`, for example `https://tix.example.com/scim/v2`; check it with `tix tenant scim show`. The
provider has to reach it over HTTPS from the internet, or from wherever the provider's agent runs.

Map your groups to roles before the first sync, so people land with the right role rather than as viewers.

### Microsoft Entra ID

1. In the Entra admin center, **Enterprise applications** > **New application** > **Create your own
   application**, name it and choose **Integrate any other application you don't find in the gallery
   (Non-gallery)**.
2. Open **Provisioning**, set **Provisioning Mode** to **Automatic**.
3. Under **Admin Credentials**, set **Tenant URL** to the endpoint and **Secret Token** to the token. **Test
   Connection** should succeed.
4. Under **Mappings**, keep **Provision Microsoft Entra ID Users** and, if you want groups, **Provision
   Microsoft Entra ID Groups**. The default user mappings (`userPrincipalName` to `userName`, `mail` to
   `emails[type eq "work"].value`, `objectId` to `externalId`, `IsSoftDeleted` to `active`) fit tix. Map
   `userName` to `mail` instead if your UPNs are not the addresses people use.
5. Assign users and groups under **Users and groups**, set **Scope** to **Sync only assigned users and groups**,
   and turn **Provisioning Status** on. **Provision on demand** pushes one user straight away for a first test.

Entra sets `active` to false when a user is unassigned or soft-deleted, and deletes the user later. Its sync
cycle runs about every 40 minutes.

### Okta

1. **Applications** > **Create App Integration**. Okta offers SCIM provisioning on SAML 2.0 and SWA integrations,
   so pick one of those. Then on the **General** tab, under **App Settings**, set **Provisioning** to **SCIM**.
2. On the **Provisioning** tab, **Integration** > **Edit**:
   - **SCIM connector base URL**: the endpoint.
   - **Unique identifier field for users**: `userName`.
   - **Supported provisioning actions**: Push New Users, Push Profile Updates, and Push Groups if you use them.
   - **Authentication Mode**: **HTTP Header**, with the token as the **Authorization** bearer token.
   **Test Connector Configuration**, then save.
3. Under **Provisioning** > **To App**, enable **Create Users**, **Update User Attributes** and **Deactivate
   Users**.
4. Assign people or groups on **Assignments**. For groups and roles, add the groups on **Push Groups**; the
   names pushed are the names `map-group` matches.

Okta deactivates rather than deletes: unassigning a person sets `active` to false.

### Authentik

1. **Applications** > **Providers** > **Create**, choose **SCIM Provider**.
2. Set **URL** to the endpoint and **Token** to the token. Keep the default user and group property mappings.
   Use the user and group filtering options to leave out service accounts or limit the sync to some groups.
3. **Applications** > **Applications**, open or create the tix application and add the SCIM provider under
   **Backchannel Providers**.
4. Authentik syncs on a schedule and on every change; **Run sync** on the provider pushes everything now.

### JumpCloud

1. **SSO Applications** > **Add New Application** > **Custom Application**, and choose **Export users to this
   app (Identity Management)**.
2. On the **Identity Management** tab, pick **SCIM 2.0** as the API type and version, set **Base URL** to the
   endpoint and **Token Key** to the token, sent as an `Authorization: Bearer` header. Turn on **Group
   Management** to push groups. **Test Connection**, then **Activate**.
3. Bind user groups to the application on its **User Groups** tab. The people in them are provisioned, and with
   Group Management on the groups are pushed under their JumpCloud names.

### Keycloak

Keycloak has no SCIM client of its own. A SCIM plugin adds one; configure it with the endpoint and the token as
a bearer token, as above.

## Related

- [tenancy.md](tenancy.md) for members, roles and accounts in several tenants
- [agents.md](agents.md#tokens-and-scopes) for the other scopes
- [audit-log.md](audit-log.md) for the access channel
