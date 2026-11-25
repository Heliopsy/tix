# Design

## The external identity model

Two tables, both tenant-scoped, so the tenant-scoped builder and PostgreSQL row-level security cover them like
every other tenant row.

```text
sso_providers
  id             TEXT PK
  tenant_id      TEXT  -> tenants
  key            TEXT  short name, unique per tenant: URLs, CLI, session method
  kind           TEXT  oidc | oauth_github | saml (reserved)
  display_name   TEXT  "Sign in with <display_name>"
  enabled        bool
  auto_create    bool  just-in-time creation of unknown people
  default_role   TEXT  role for a created person when no group maps
  trust_mfa      bool  skip the tix second factor when the provider asserts one
  allowed_domains TEXT JSON array of email domains, empty is any
  role_mapping   TEXT JSON object group -> role
  config         TEXT JSON, the kind's own settings (issuer, client id, claims, orgs ...)
  secret_sealed, secret_nonce, secret_hint   the sealed client secret
  created_at, updated_at
  UNIQUE (tenant_id, key)

user_identities
  id             TEXT PK
  tenant_id      TEXT -> tenants
  provider_id    TEXT -> sso_providers  ON DELETE CASCADE
  subject        TEXT  the provider's stable id: OIDC sub, GitHub numeric id, SAML NameID
  user_id        TEXT -> users
  email          TEXT  as the provider last reported it
  created_at, last_login_at
  UNIQUE (tenant_id, provider_id, subject)
  UNIQUE (tenant_id, provider_id, user_id)

tenants.password_login TEXT 'allowed' | 'disabled'
```

Kind-independent policy (enabled, auto_create, default_role, trust_mfa, allowed domains, role mapping) is in
columns or typed fields, so the sign-in logic reads it the same way for every kind. Only the protocol settings
are in `config`. SAML adds `kind = saml` and its own `config` shape (IdP metadata, entity id, attribute names)
and needs no migration.

`user_identities` is per tenant because providers are per tenant: the same Keycloak realm configured in two
tenants is two providers, and a person linked in one is not linked in the other. `users` stays global.

## Replacing users.sso_subject and users.sso_provider

They are dropped rather than reused. One provider and one subject per global account cannot hold a person who
signs in to one tenant through Keycloak and another through GitHub, or two providers in one tenant, and a global
column cannot say which tenant's provider vouched for it. Nothing ever wrote or read them, so the drop loses no
data. `core.User` loses the two fields, which were always empty.

## Code seams SAML reuses

- `internal/sso`: `Identity` (subject, email, email verified, name, username, groups, MFA asserted) is what any
  protocol hands the service. `Adapter` (`AuthURL`, `Exchange`) is the redirect-and-code shape OIDC and GitHub
  share; SAML implements its own request and response handling and produces the same `Identity`.
- `service.(*Local).signInIdentity(ctx, provider, identity, flow)` does everything after the protocol: domain
  allowlist, linking, just-in-time creation, role mapping, the global admin refusal, the second factor and the
  session. SAML calls it unchanged.
- `auth.SSOFlow`, sealed with a key derived from `auth.totp_key`, carries tenant, provider, state, nonce, PKCE
  verifier, redirect URI, surface, keep me signed in and the screen to return to. SAML uses it as RelayState.

## Flow state is sealed, not stored

The browser carries the sealed flow in an HttpOnly, SameSite=Lax cookie scoped to `/login/sso`, and the CLI keeps
it in memory. Nothing is written until the sign-in completes, so an abandoned sign-in leaves no row, and a
replica without sticky routing completes a flow another began, as long as both have the same key. The state in
the callback is compared with the state in the flow, which binds the callback to the browser that began it.
A flow lasts ten minutes.

## One callback

`GET /login/sso/callback` serves every provider; the flow names the provider. The redirect URL to register is
`<public address>/login/sso/callback`. The CLI registers no URL of its own: it asks for a loopback redirect,
`http://127.0.0.1:PORT/callback`, which RFC 8252 identity providers accept on any port. The service accepts only
those two shapes of redirect URI.

## Linking rules and the confinement of an SSO sign-in

1. An identity row for (provider, subject) signs in its user.
2. Otherwise a verified email (`email_verified` in the token, or a GitHub email that is primary and verified)
   links to the account with that email when it is a member of this tenant.
3. Otherwise, with `auto_create`, a new account is created with the mapped or default role, when no account has
   that email anywhere. An account that exists only in other tenants is not adopted: a tenant reaches an
   existing account only by invitation, and a provider the tenant's admin controls is not the person's consent.
4. Otherwise the sign-in is refused, saying whether to ask for an invitation or why the email was not used.

Whoever administers a tenant's provider can assert any email, so an SSO session is confined to its tenant: it
cannot switch tenant, and an account that is a global admin is refused SSO outright and signs in with a password
or an SSH key. Allowed email domains apply on every sign-in, linked identities included.

## Roles from groups

`role_mapping` maps a group (an OIDC groups claim value, a GitHub organisation) to a role. When a person's groups
match, the highest matched role is applied on every sign-in, so removing someone from a group demotes them at
their next sign-in. When none match, a new person gets `default_role` and an existing member keeps theirs.

## Second factor

The tenant's TOTP requirement applies after SSO exactly as after a password: the same ticket, method `sso:KEY`.
`trust_mfa` is opt-in and per provider: the token's `amr` must contain one of the provider's `mfa_values`
(default `mfa`) or its `acr` must equal one. GitHub reports neither, so `trust_mfa` is refused there.

## Password policy

`tenants.password_login` is read in the password step of `Login`. Disabled refuses every account but a global
admin, with the same error everywhere. Disabling is refused while the tenant has no enabled provider, and
disabling or deleting the last enabled provider is refused while password sign-in is disabled. API tokens and SSH
keys keep working, and so does `tix login --ssh-key`.

## auth.mode

Authentication modes describe what the server accepts as a credential. With per-tenant providers, there is no
server-wide OIDC mode to choose: `auth.mode` stays `token`, and `oidc` is refused with a message pointing to
`tix tenant sso add`. Accepting it would make an operator think the server was configured for SSO.

## Dependencies

`github.com/coreos/go-oidc/v3` for discovery, JWKS caching and ID token verification, and `golang.org/x/oauth2`
(already in the module graph) for the code exchange with PKCE. Writing JOSE verification by hand is the kind of
code that should not be original. Both are Apache-2.0 / BSD and pure Go.

## Outbound requests

Discovery, JWKS, token and GitHub API requests use the server's egress policy, like webhooks and sync. An identity
provider on an internal address needs `server.egress_allow`.
