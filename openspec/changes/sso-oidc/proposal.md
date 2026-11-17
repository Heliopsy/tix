# Single sign-on with OIDC and GitHub

## Why

A team that already runs Keycloak, Authentik, Entra ID, Google Workspace, Okta or a GitHub organisation has to
keep a second password for tix, and an administrator has to create and remove every account by hand. The seam was
left in v1 (`users.sso_subject`, `users.sso_provider`, the `Authenticator` chain) and `auth.mode: oidc` has been
refused as unimplemented since. This is the first of three pieces: SAML and SCIM come after it and build on the
identity model it adds.

## What Changes

- Per-tenant sign-in providers, `sso_providers`, of kind `oidc`, `oauth_github`, and `saml` (reserved, refused).
  Managed by `tix tenant sso add/ls/show/edit/rm`, `/v1/sso/providers` and a Single sign-on screen under Tenant
  settings, behind a new scope `sso:admin`, which tenant admins hold through their role.
- OIDC: issuer with discovery, client id and secret, scopes, claim mapping (email, name, username, groups),
  allowed email domains, a group-to-role mapping, a default role, `auto_create` and `trust_mfa`.
- GitHub: client id and secret, optional allowed organisations, the email read from the API, optional GitHub
  Enterprise addresses.
- The client secret is sealed with the server's key (`auth.totp_key`), bound to the provider, and never returned.
- External identities live in `user_identities` (tenant, provider, subject, user, email, last sign-in), shared by
  every kind. `users.sso_subject` and `users.sso_provider`, never read, are dropped.
- Browser sign-in: one "Sign in with NAME" button per enabled provider, authorization code with PKCE, state and
  nonce, ID token checked for signature (JWKS, cached), issuer, audience, expiry and nonce.
- Linking: subject first, then a verified email of a member of this tenant, then just-in-time creation when the
  provider allows it, else a refusal that says what to do.
- Sessions come from the existing machinery (lifetime, idle timeout, keep me signed in), with the method recorded
  as `sso:KEY`. The tenant's second factor still applies unless the provider is `trust_mfa` and the token says
  the person passed one.
- `password_login: allowed | disabled` per tenant. Disabling needs an enabled provider. Global admins keep
  password sign-in as the break-glass path. API tokens and SSH keys are unaffected.
- `tix login --sso [KEY]` over a loopback redirect, with the URL printed when no browser opens.
- `auth.mode` stays `token`. `oidc` is refused with a pointer to per-tenant providers rather than as
  unimplemented. `none` is still unimplemented.
- Audit: `sso.login`, `sso.link`, `sso.user_created`, `sso.denied`, and provider create, update and delete.
- The terminal tenant screen lists the providers.
