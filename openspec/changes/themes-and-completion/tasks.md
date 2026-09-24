# Tasks

## Theming

- [x] `internal/core/theme.go`: `Theme`, colour validation, built-in registry, `ResolveTheme`
- [x] `internal/core/theme_test.go`: validation table, registry precedence, unknown names
- [x] `Tenant.Theme` field, migration `0012_tenant_theme.sql`, store read/write both engines
- [x] `internal/config`: `themes:` block, validated on load, merged over built-ins
- [x] `service`: refuse an unresolvable theme on write; expose the resolved theme for a tenant
- [x] `internal/web`: accent from the resolved theme, hash only when unthemed
- [x] `internal/tui`: `NewTheme` takes the resolved theme; accent on header, ref, selection
- [x] `cmd`: `tix theme ls`, `tix tenant set --theme`
- [ ] Leak test: a theme name never crosses a tenant

## Completion install

- [x] `cmd/completion_install.go`: paths per shell, `--shell`, `--dry-run`, `--uninstall`
- [x] Detect from `$SHELL`, refuse unsupported, exit 2
- [x] Idempotent write; report path and whether sourcing is still needed
- [x] Golden tests over a temp HOME for all three shells

## Statistics

- [x] `core.Stats`, `core.StatsInput`, `Service.Stats`
- [x] Store queries, keyset-free aggregates, tenant-scoped through the builder, both engines
- [x] `service/stats.go` with a `FakeClock`-driven window
- [x] `cmd/stats.go` with table and JSON output
- [x] `internal/httpapi`: `GET /api/v1/stats`
- [x] `internal/web`: `/stats` screen, bars drawn in CSS
- [x] `internal/tui`: stats view
- [x] Capability registry entries with CLI, HTTP and Web bindings
- [x] Tenant-isolation test over every new route

## Documentation

- [x] `docs/theming.md`, `docs/statistics.md`, both linked from `docs/README.md`
- [x] `ROADMAP.md`: move completion and theming into what shipped
- [ ] Regenerate the README command table

## Release

- [ ] `just ci` green, coverage above the gate
- [ ] `openspec validate themes-and-completion --strict`
- [ ] Archive the change, tag v0.3.0
