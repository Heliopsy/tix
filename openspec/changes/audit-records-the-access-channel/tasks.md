# Tasks

## 1. Contract and storage

- [x] 1.1 `internal/core/via.go`: `ViaKind`, `Via`, `Refine`, `Derive`, `Public`, `WithVia`, `ViaFrom`
- [x] 1.2 `AuditEntry.Via`, `Event.Via`, `AuditFilter.Vias` and `Tokens`, `Actor.TokenName` and `SessionID`
- [x] 1.3 Migration 0033 and both engines: write, scan, filter, text search

## 2. Entry points

- [x] 2.1 HTTP: base channel from the path, credential override, `X-Tix-Client` refinement
- [x] 2.2 CLI: `cli-local` with token and optional host details; the client sends `X-Tix-Client`
- [x] 2.3 `tix mcp` sends `mcp`
- [x] 2.4 SSH sessions, watch and second factor; sandbox seed
- [x] 2.5 Background jobs name their job; sync and import derive their channel
- [x] 2.6 The mutation stamps the channel on every audit entry and event

## 3. Surfaces

- [x] 3.1 `via` and `token` on the audit routes, `just openapi`
- [x] 3.2 `tix audit ls --via --token`, the VIA column, `via:` and `token:` filter terms
- [x] 3.3 Activity: channel chip, details, channel dropdown, token box; History: channel column
- [x] 3.4 Terminal history line names the channel
- [x] 3.5 Icons, Bulgarian strings, demo seed with a mix of channels

## 4. Tests and docs

- [x] 4.1 One test per channel through its real entry point; filters; unknown on old rows; spoofing
- [x] 4.2 docs/audit-log.md, api.md, web-ui.md, configuration.md, skill
