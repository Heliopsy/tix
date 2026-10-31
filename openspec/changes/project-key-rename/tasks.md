# Tasks

## 1. Data

- [ ] 1.1 Migration `project_key_aliases`, scoped table, PostgreSQL policy
- [ ] 1.2 Store: follow aliases in `GetProject` and `GetTask`; add, read and drop aliases (SQLite and PostgreSQL)

## 2. Service

- [ ] 2.1 `UpdateProjectInput.Key`; rename in one transaction with alias, audit and event
- [ ] 2.2 Refuse a key held by another project, as key or alias; allow taking back an own alias
- [ ] 2.3 Snapshot import keeps the current key of a project matched through an alias

## 3. Surfaces

- [ ] 3.1 CLI `tix project edit --key`
- [ ] 3.2 HTTP `PATCH /api/v1/projects/{ref}` accepts `key`
- [ ] 3.3 Web key field, tip, confirmation step, redirects from old project and task URLs
- [ ] 3.4 TUI key attribute on the project edit form
- [ ] 3.5 i18n entries with Bulgarian
- [ ] 3.6 Docs, README, skill reference
