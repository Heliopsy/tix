# Tasks

## 1. Data

- [x] 1.1 Migration `project_key_aliases`, scoped table, PostgreSQL policy
- [x] 1.2 Store: follow aliases in `GetProject` and `GetTask`; add, read and drop aliases (SQLite and PostgreSQL)

## 2. Service

- [x] 2.1 `UpdateProjectInput.Key`; rename in one transaction with alias, audit and event
- [x] 2.2 Refuse a key held by another project, as key or alias; allow taking back an own alias
- [x] 2.3 Snapshot import keeps the current key of a project matched through an alias

## 3. Surfaces

- [x] 3.1 CLI `tix project edit --key`
- [x] 3.2 HTTP `PATCH /api/v1/projects/{ref}` accepts `key`
- [x] 3.3 Web key field, tip, confirmation step, redirects from old project and task URLs
- [x] 3.4 TUI key attribute on the project edit form
- [x] 3.5 i18n entries with Bulgarian
- [x] 3.6 Docs, README, skill reference
