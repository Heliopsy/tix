# Tasks

## Groundwork

- [ ] Migration: `sso_providers`, `user_identities`, `tenants.password_login`; drop `users.sso_*`
- [ ] `core` types: `SSOProvider`, inputs, `SignInOptions`, `SSOBegin`, scope `sso:admin`, `PasswordLogin`
- [ ] Store methods on both engines
- [ ] `internal/sso`: `Identity`, OIDC adapter on go-oidc, GitHub adapter, fake providers in `ssotest`
- [ ] `auth.SSOFlow` sealed flow state

## Service

- [ ] Provider CRUD with sealed secret, audit, and the password policy checks
- [ ] `SignInOptions`, `BeginSSOLogin`, `CompleteSSOLogin`
- [ ] Linking, just-in-time creation, domain allowlist, role mapping, global admin refusal
- [ ] Second factor after SSO and `trust_mfa`
- [ ] `password_login` in `UpdateTenant` and the password step; switch refused for SSO sessions

## Surfaces

- [ ] Registry entries, HTTP routes, client, `just openapi`
- [ ] `tix tenant sso add/ls/show/edit/rm`, `tix tenant edit --password-login`, `tix login --sso`
- [ ] Web: provider buttons, `/login/sso/KEY`, `/login/sso/callback`, Single sign-on settings screen
- [ ] TUI: providers on the tenant screen
- [ ] i18n: bg, de, fr
- [ ] `auth.mode` oidc refusal

## Docs

- [ ] docs/sso.md, configuration.md, api.md, deployment.md, README, skill, ROADMAP
