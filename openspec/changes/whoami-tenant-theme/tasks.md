# Tasks

## Service

- [x] `core.Actor.Theme`, filled by describeAccount for every credential with the tenant key and name
- [x] `internal/config` reads the `themes` block with its dark pair, overlays a discovered file, keeps it on save
- [x] `internal/connect` passes the configured registry with `service.WithThemes`
- [x] Service tests on SQLite: agent token, system actor, configured theme, custom accent

## Surfaces

- [x] `tix whoami` prints the theme
- [x] Web profile reads the tenant from whoami alone
- [x] `just openapi`
- [x] docs/tenancy.md, docs/theming.md, docs/api.md, docs/web-ui.md, docs/configuration.md, tix skill reference
