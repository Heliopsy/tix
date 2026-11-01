# Tasks

## 1. Model and storage

- [ ] 1.1 `internal/core/calendar.go`: feed types, `CalendarService`
- [ ] 1.2 `internal/store/migrations/0022_calendar_feeds.sql` and the store methods on both engines
- [ ] 1.3 `internal/auth`: mint and parse `tix_cal_` secrets
- [ ] 1.4 `internal/service/calendar.go`: the six methods; `internal/authz`: `calendar.feed`
- [ ] 1.5 `internal/ical`: the RFC 5545 renderer and its round-trip test

## 2. Surfaces

- [ ] 2.1 `internal/wire`, `internal/httpapi`, `internal/client`: the routes, the public feed with ETag
- [ ] 2.2 `internal/capability/registry.go`: six entries
- [ ] 2.3 `cmd/calendar.go`: `tix calendar show|create|rotate|filter|revoke`
- [ ] 2.4 `internal/web`: the Settings section, the one-time URL screen, i18n with Bulgarian

## 3. Documentation

- [ ] 3.1 `docs/calendar.md`, `docs/api.md`, `docs/web-ui.md`, `docs/features.md`, `docs/commands.md`,
      README and `skills/tix`
