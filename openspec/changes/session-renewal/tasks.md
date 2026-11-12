# Tasks

## Service

- [x] `SessionPolicy.MaxLifetime`, `RememberMaxLifetime`, `Renewal`; defaults 30d and 90d
- [x] `StoredSession.Renewed`, written with the last-use touch; deleted session refused
- [x] Store `TouchSession` takes the renewed expiry; `CreateSession` honours a carried sign-in time; both engines
- [x] Tenant switch carries the sign-in time (code and recovery code paths) and the remembered flag
- [x] Remembered cookie re-sent on renewal; `session_expires_at` on whoami

## Configuration

- [x] `session.max_lifetime`, `session.remember_max_lifetime`, validation, `tix serve` flags

## Docs and tests

- [x] OpenAPI regenerated, configuration, deployment, tenancy, web-ui, api, SECURITY.md, skill
- [x] Bulgarian strings for the changed sign-in and tenant texts
- [x] Tests: renewal math, throttle, cap, removed session, cookie, both stores, credential lookup, whoami
