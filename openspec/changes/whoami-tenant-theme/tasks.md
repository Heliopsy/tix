# Tasks

## Service

- [ ] `core.Actor.Theme`, filled by describeAccount for every credential with the tenant key and name
- [ ] `internal/config` reads the `themes` block with its dark pair, overlays a discovered file, keeps it on save
- [ ] `internal/connect` passes the configured registry with `service.WithThemes`
- [ ] Service tests on SQLite: agent token, system actor, configured theme, custom accent

## Surfaces

- [ ] `tix whoami` prints the theme
- [ ] Web profile reads the tenant from whoami alone
- [ ] `just openapi`
- [ ] docs/tenancy.md, docs/theming.md, docs/api.md, docs/web-ui.md, docs/configuration.md, tix skill reference
