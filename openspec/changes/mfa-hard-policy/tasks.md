# Tasks

## Service

- [ ] `core.Tenant` `mfa_policy`, `mfa_grace`, `mfa_required_at`; `UpdateTenantInput` fields and validation
- [ ] Migration and both stores
- [ ] The gate in `authorize`, `authorizeGlobal`, `GetActor`, `ListActors`, `RotateToken`
- [ ] Sign-in under the policy: browser mints the session, CLI enrols, grace signs in
- [ ] Switch and own removal follow the policy
- [ ] Lockout guard and the sealing key check
- [ ] WhoAmI and TOTPStatus report the policy and the deadline
- [ ] `UserFilter.WithoutTOTP`
- [ ] Audit: policy metadata, rate-limited `user.totp.gate`
- [ ] Service tests: gate, agent exempt, person token, grace, lockout guard, clear

## Surfaces

- [ ] HTTP `without_totp`, `just openapi`
- [ ] `tix tenant edit --mfa-policy --mfa-grace`, `tix user ls --without-2fa`, login refusal without a terminal
- [ ] Web: enrolment screen, gate redirect and 403, grace banner, Tenant section, Users filter, Bulgarian strings
- [ ] SSH gate and in-process test
- [ ] TUI tenant editor attributes and people filter
- [ ] Docs: second-factor, tenancy, web-ui, tui; skills
