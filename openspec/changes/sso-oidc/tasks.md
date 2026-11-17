# Tasks

## Groundwork

- [x] Migration: `sso_providers`, `user_identities`, `tenants.password_login`; drop `users.sso_*`
- [x] `core` types: `SSOProvider`, inputs, `SignInOptions`, `SSOBegin`, scope `sso:admin`, `PasswordLogin`
- [x] Store methods on both engines
- [x] `internal/sso`: `Identity`, OIDC adapter on go-oidc, GitHub adapter, fake providers in `ssotest`
- [x] `auth.SSOFlow` sealed flow state

## Service

- [x] Provider CRUD with sealed secret, audit, and the password policy checks
- [x] `SignInOptions`, `BeginSSOLogin`, `CompleteSSOLogin`
- [x] Linking, just-in-time creation, domain allowlist, role mapping, global admin refusal
- [x] Second factor after SSO and `trust_mfa`
- [x] `password_login` in `UpdateTenant` and the password step; switch refused for SSO sessions

## Surfaces

- [x] Registry entries, HTTP routes, client, `just openapi`
- [x] `tix tenant sso add/ls/show/edit/rm`, `tix tenant edit --password-login`, `tix login --sso`
- [x] Web: provider buttons, `/login/sso/KEY`, `/login/sso/callback`, Single sign-on settings screen
- [x] TUI: providers on the tenant screen
- [x] i18n: bg, de, fr
- [x] `auth.mode` oidc refusal

## Docs

- [x] docs/sso.md, configuration.md, api.md, deployment.md, README, skill, ROADMAP
