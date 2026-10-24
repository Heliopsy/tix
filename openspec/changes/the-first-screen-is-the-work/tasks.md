# Tasks

- [x] 1.1 `internal/core/due.go`: a task with `CompletedAt` set has no due state
- [x] 1.2 `internal/web/tasks.go`: default the listing to the workflows' non-terminal states
- [x] 1.3 `internal/web/templates/tasks.html`: say the list is narrowed, and offer the way off
- [x] 1.4 `internal/web/templates/tasks.html`: an empty narrowed list reads as nothing open
- [x] 1.5 `internal/web/render.go`: the advanced navigation follows the scope, not the path
- [x] 1.6 `internal/web/admin.go`: a monogram is made of letters and digits
- [x] 1.7 Guards for each, each mutated and watched to fail
- [x] 2.1 `internal/web/tasks.go`: the pager carries the request for finished work
- [x] 2.2 `internal/web/tasks.go`: the open-work default is the list's, not the board's
- [x] 2.3 `internal/web/templates/tasks.html`: the way off is built from the address the reader is on
- [x] 2.4 `internal/web/render.go`: workflow:read shows the navigation that leads to /workflows
- [x] 2.5 `internal/capability/authority_test.go`: the scope list is read against the registry
- [x] 2.6 Guards for each, each mutated and watched to fail
