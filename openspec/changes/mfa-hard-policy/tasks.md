# Tasks

## Service

- [x] `core.Tenant` `mfa_policy`, `mfa_grace`, `mfa_required_at`; `UpdateTenantInput` fields and validation
- [x] Migration and both stores
- [x] The gate in `authorize`, `authorizeGlobal`, `GetActor`, `ListActors`, `RotateToken`
- [x] Sign-in under the policy: browser mints the session, CLI enrols, grace signs in
- [x] Switch and own removal follow the policy
- [x] Lockout guard and the sealing key check
- [x] WhoAmI and TOTPStatus report the policy and the deadline
- [x] `UserFilter.WithoutTOTP`
- [x] Audit: policy metadata, rate-limited `user.totp.gate`
- [x] Service tests: gate, agent exempt, person token, grace, lockout guard, clear

## Surfaces

- [x] HTTP `without_totp`, `just openapi`
- [x] `tix tenant edit --mfa-policy --mfa-grace`, `tix user ls --without-2fa`, login refusal without a terminal
- [x] Web: enrolment screen, gate redirect and 403, grace banner, Tenant section, Users filter, Bulgarian strings
- [x] SSH gate and in-process test
- [x] TUI tenant editor attributes and people filter
- [x] Docs: second-factor, tenancy, web-ui, tui; skills
