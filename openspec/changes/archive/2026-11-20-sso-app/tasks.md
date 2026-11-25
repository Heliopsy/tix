# Tasks

- [x] `auth.AppSignIn` in the sealed flow and the signed ticket, `PeekSSOFlow`, PKCE S256 helpers
- [x] `sso_handoffs` migration, `CreateSSOHandoff` and `TakeSSOHandoff` on both engines
- [x] `auth.sso_app_redirects` with its default and validation, wired into the service
- [x] Begin validates the app values; the hand off at `mintSessionIf`; refusals carry the app's redirect
- [x] Second factor judged on web and cli for an app sign-in, asked in the browser
- [x] `ExchangeSSOCode`, `POST /api/v1/auth/sso/exchange`, `sso_code_invalid`, registry entry, MCP hidden
- [x] `app_sign_in` on sign-in options; `just openapi`
- [x] Web: app values at `/login/sso/KEY`, 302 or the hand-back page, recovery codes before going on, strings
- [x] Tests: OIDC end to end, SAML through the ACS, second factor, wrong verifier, reuse, expiry, race,
  allowlist, feature off, options field
- [x] Docs: sso.md, configuration.md, api.md, deployment.md
