# Tasks

## 1. Storage and service

- [x] 1.1 Migration 0032: `time_format`, `timezone` on `tenants` and `users`, both engines
- [x] 1.2 `core.TimePrefs`, `TimeSettings`, `TimePrefsInput`, `ResolveTimePrefs`, `ValidateTimePrefs`
- [x] 1.3 `GetTimeSettings`, `SetTimePrefs`, tenant fields on `UpdateTenant`, `WithTimeDefaults`
- [x] 1.4 Registry entries, `GET`/`PATCH /api/v1/me/time`, client, OpenAPI

## 2. Surfaces

- [x] 2.1 Web: reader style resolved per request; settings and tenant screens show each level
- [x] 2.2 Web: legacy cookies moved onto the account and expired
- [x] 2.3 Web: `when`/`whenRel` helpers and `tt`; every template timestamp goes through them
- [x] 2.4 Web: stats day labels in the reader's order; tokens Last used column
- [x] 2.5 CLI: `tix user time`, `tix tenant edit --time-format --timezone`, tables follow the account
- [x] 2.6 CLI: reflected tables format instants through the style instead of Go's default
- [x] 2.7 TUI and SSH: style resolved per person; activity clock and snooze note use it
- [x] 2.8 Bulgarian catalog entries

## 3. Guards

- [x] 3.1 Resolution order, inherit, agent refused (service)
- [x] 3.2 Settings and tenant forms, cookie migration, concurrent readers (web)
- [x] 3.3 Every screen under dotted and Asia/Tokyo; every `<time>` carries the full tooltip

## 4. Docs

- [x] 4.1 `docs/configuration.md`, `docs/web-ui.md`, `docs/tui.md`, `docs/api.md`, `skills/tix`
