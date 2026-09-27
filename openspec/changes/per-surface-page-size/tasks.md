# Tasks

## 1. The keys

- [x] 1.1 `internal/core/filter.go`: `DefaultDisplayLimit`, the shipped row count both keys default to
- [x] 1.2 `internal/config/config.go`: `CLI.PageSize` and `Web.PageSize`, both defaulting to 25
- [x] 1.3 `internal/config/validate.go`: refuse a page size outside 1..`core.MaxPageLimit`, naming the key

## 2. The command line

- [x] 2.1 `cmd/root.go`: `globals.pageSize` returns the typed `--limit` or the configured value
- [x] 2.2 `cmd/task.go`: the configured size joins the filter expression, so `limit:` and `--limit` still win
- [x] 2.3 `cmd/project.go`, `cmd/actor.go`, `cmd/auth.go`, `cmd/tenant.go`, `cmd/admin.go`: same fallback
- [x] 2.4 Every `--limit` declares `config.DefaultPageSize` rather than the contract's fallback

## 3. The browser

- [x] 3.1 `internal/web/web.go`: `WithPageSize`, ignoring a value the contract would refuse
- [x] 3.2 `internal/web/pager.go`: `SizeParam` and `rowsPerPage`, bounded, falling back to the configured size
- [x] 3.3 `internal/web/tasks.go`, `projects.go`, `actors.go`, `admin.go`, `webhooks.go`: apply it
- [x] 3.4 The pager carries `limit` into its own links, so a walk keeps the size it was asked for
- [x] 3.5 `cmd/serve.go`: pass `web.page_size` to the handler

## 4. The guards

- [x] 4.1 `cmd/config_shadow_test.go`: `cli.page_size` classified as shadowed, `web.page_size` as read by
      `runServe`; `commandNamed` walks a path so a subcommand's flag can be named
- [x] 4.2 `cmd/pagesize_test.go`: each listing command prints the configured number of rows, and reports a
      further page, so a short listing cannot pass as a page size
- [x] 4.3 `cmd/pagesize_test.go`: the environment layer, a typed `--limit` and a `limit:` term each outrank it
- [x] 4.4 `cmd/pagesize_test.go`: an unusable page size fails the command and prints no rows
- [x] 4.5 `internal/web/pagesize_test.go`: rows counted inside `ul.tasklist`, for the configured size, the
      shipped 25, a requested size, an unusable requested size and an unusable configured one
- [x] 4.6 `internal/config/pagesize_test.go`: the default is 25 and the accepted range's edges hold

## 5. Documentation

- [x] 5.1 `docs/configuration.md`: both keys, their range, why there are two, and the query parameter
- [x] 5.2 `docs/scripting.md`: one page is `cli.page_size` rows, not fifty
