# Tasks

## 1. Sync source settings

- [x] 1.1 `core.SyncSource` carries `config` and `mapping_path`; input doc comments say they are stored
- [x] 1.2 Service validates keys per system, refuses credentials and unknown keys, stores the settings
- [x] 1.3 Both stores read and write `config` and `mapping_path`; the run's cursor write keeps them
- [x] 1.4 Run time precedence: stored value over environment, environment fills the rest
- [x] 1.5 CLI flags on `tix sync source add`, settings column in the listing
- [x] 1.6 Web form fields and stored settings in the Sources table, Bulgarian strings
- [x] 1.7 `just openapi`, docs/migrating.md, skill

## 2. Drop the indexed flag

- [ ] 2.1 Remove `Indexed` from `core.FieldDef`, `FieldDefInput`, bundles, TUI, web, output, demo
- [ ] 2.2 Migration 0029 drops `field_defs.indexed` on both engines
- [ ] 2.3 `tix field put --indexed` hidden and deprecated
- [ ] 2.4 Tests: API body, old bundle and old snapshot carrying `indexed` still import
- [ ] 2.5 Docs and skill no longer describe the flag
