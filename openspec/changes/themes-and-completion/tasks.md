# Tasks

## Theming

- [ ] `internal/core/theme.go`: `Theme`, colour validation, built-in registry, `ResolveTheme`
- [ ] `internal/core/theme_test.go`: validation table, registry precedence, unknown names
- [ ] `Tenant.Theme` field, migration `0012_tenant_theme.sql`, store read/write both engines
- [ ] `internal/config`: `themes:` block, validated on load, merged over built-ins
- [ ] `service`: refuse an unresolvable theme on write; expose the resolved theme for a tenant
- [ ] `internal/web`: accent from the resolved theme, hash only when unthemed
- [ ] `internal/tui`: `NewTheme` takes the resolved theme; accent on header, ref, selection
- [ ] `cmd`: `tix theme ls`, `tix tenant set --theme`
- [ ] Leak test: a theme name never crosses a tenant

## Completion install

- [ ] `cmd/completion_install.go`: paths per shell, `--shell`, `--dry-run`, `--uninstall`
- [ ] Detect from `$SHELL`, refuse unsupported, exit 2
- [ ] Idempotent write; report path and whether sourcing is still needed
- [ ] Golden tests over a temp HOME for all three shells

## Statistics

- [ ] `core.Stats`, `core.StatsInput`, `Service.Stats`
- [ ] Store queries, keyset-free aggregates, tenant-scoped through the builder, both engines
- [ ] `service/stats.go` with a `FakeClock`-driven window
- [ ] `cmd/stats.go` with table and JSON output
- [ ] `internal/httpapi`: `GET /api/v1/stats`
- [ ] `internal/web`: `/stats` screen, bars drawn in CSS
- [ ] `internal/tui`: stats view
- [ ] Capability registry entries with CLI, HTTP and Web bindings
- [ ] Tenant-isolation test over every new route

## Documentation

- [ ] `docs/theming.md`, `docs/statistics.md`, both linked from `docs/README.md`
- [ ] `ROADMAP.md`: move completion and theming into what shipped
- [ ] Regenerate the README command table

## Release

- [ ] `just ci` green, coverage above the gate
- [ ] `openspec validate themes-and-completion --strict`
- [ ] Archive the change, tag v0.3.0
