# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [ ] 2.1 Migration 0016: `projects.icon_name`, `projects.icon_image`, `project_icons` (both engines)
- [ ] 2.2 Store methods `PutProjectIcon`, `GetProjectIcon`, `DeleteProjectIcon` on SQLite and PostgreSQL

## 3. Contract and service

- [ ] 3.1 `core.Project.IconName` / `IconImage`, input fields, built-in names, image sniffing and SVG checks
- [ ] 3.2 Create and update set exactly one icon kind and write the image in the same transaction
- [ ] 3.3 `Service.GetProjectIcon`, authorized as a project read
- [ ] 3.4 Snapshot and bundle import carry `icon_name` and keep or drop an existing image

## 4. Surfaces

- [ ] 4.1 Capability registry entry `project.icon`
- [ ] 4.2 HTTP `GET /api/v1/projects/{ref}/icon`, client method
- [ ] 4.3 CLI `--icon-name`, `--icon-file`, `tix project icon`, table fallback
- [ ] 4.4 Web picker on the create and edit forms, sprite symbols, icon route, rendering everywhere a project icon shows
- [ ] 4.5 TUI fallback glyphs
- [ ] 4.6 i18n keys in English and Bulgarian

## 5. Docs and tests

- [ ] 5.1 Docs for the API, CLI and web UI
- [ ] 5.2 Tests on every layer, each watched failing
