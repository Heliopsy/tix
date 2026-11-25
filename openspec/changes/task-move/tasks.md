# Tasks

## Data

- [x] 1.1 Migration `task_key_aliases`, scoped table, PostgreSQL policy through `ScopedTables`
- [x] 1.2 Store: `AddTaskAlias`; `GetTask` resolves a moved task's old reference (SQLite and PostgreSQL)

## Service

- [x] 2.1 `core.MoveTaskInput`, `core.TaskMove`, `Service.MoveTask`
- [x] 2.2 Move in one transaction: renumber, status, fields, tags, alias, audit and both events
- [x] 2.3 Refuse own project, archived target, missing authority and a claim held by somebody else
- [x] 2.4 Bulk action `move_project` with `moved_ref`
- [x] 2.5 Registry entry `task.move`, HTTP route, client, OpenAPI

## Surfaces

- [x] 3.1 CLI `tix task move REF... --to PROJECT`, comma lists and `-`, `--dry-run`; `tix task bulk move-project`
- [x] 3.2 Web Move to… form on the task screen and Move to… in the bulk bar
- [x] 3.3 TUI `d` with a project picker on board and detail
- [x] 3.4 Bulgarian strings
- [x] 3.5 Transport matrix scenario
- [x] 3.6 Docs: features, API, web UI, TUI, skill
