# Tasks

## Service

- [x] `core.SessionPolicy`, `EffectiveSessionPolicy`, Keep me signed in parsing; tenant and input fields
- [x] Migration: tenant session columns, `sessions.last_seen_at` and `sessions.remembered`; both stores
- [x] `service.WithSessionPolicy`, tighten-only check in `UpdateTenant`, lifetime chosen at mint
- [x] `LoginInput.Remember`, carried through every sign-in path and a tenant switch
- [x] `SessionPolicy` service method, registry entry, public route, client, OpenAPI
- [x] Idle timeout enforced at credential lookup, use recorded at most once a minute

## Configuration and serving

- [x] `session.ttl`, `session.idle_timeout`, `session.remember_ttl` with validation and classification
- [x] `tix serve --session-ttl --session-idle-timeout --session-remember-ttl` join the flag layer when typed

## Surfaces

- [x] `tix tenant edit --session-ttl --session-idle-timeout --session-remember-ttl`, `tix tenant session-policy`,
      `tix login --remember`
- [x] Web: Keep me signed in on sign-in, persistent cookie only when remembered, Tenant settings fields, sessions
      list marks remembered
- [x] TUI: tenant screen shows and edits all three; account sessions mark remembered
- [x] Transport matrix scenarios
- [x] Bulgarian strings
- [x] Docs: configuration, tenancy, web UI, TUI, API, deployment, SECURITY, skill
