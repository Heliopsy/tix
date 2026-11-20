# Single sign-on for native apps

## Why

tix mobile signs in with a password only. The command line reaches a tenant's providers through a loopback
redirect, which a phone cannot listen on and which SAML cannot use at all. Tenants that turned passwords off, or
that sign everyone in through Entra ID or Okta, have no way into the app.

## What Changes

- The app opens `/login/sso/KEY?app_redirect=URI&code_challenge=C&code_challenge_method=S256&app_state=S` in
  the system browser. The browser runs the ordinary web flow, for OIDC, GitHub and SAML, with the provider's
  redirect URI unchanged, so no IdP configuration changes.
- `app_redirect` must match an entry of the new `auth.sso_app_redirects` exactly (default
  `["red.tix.mobile:/sso"]`, empty turns the feature off). The challenge is required, S256 only; `app_state` is
  at most 256 characters. The values are sealed into the flow cookie.
- Where the web flow would set its session cookie, an app flow stores a one-time code (32 random bytes, hashed,
  bound to the challenge, the account, the tenant and the provider, two minutes, single use across replicas in
  a new `sso_handoffs` table) and sends the browser to `URI?code=CODE&state=S`; a refusal goes to
  `URI?error=REASON&state=S` once the redirect is validated.
- `POST /api/v1/auth/sso/exchange` trades the code and the PKCE verifier for the same answer a completed
  `POST /api/v1/auth/login` gives. Refusals are `401 invalid credentials`, logged as `sso_code_invalid`.
- `GET /api/v1/auth/sso/options` gains `app_sign_in`.
- Docs: docs/sso.md, docs/configuration.md, docs/api.md, docs/deployment.md.

Not in this change: the app's own screens (tix-mobile), dynamic client registration, and refresh tokens.
