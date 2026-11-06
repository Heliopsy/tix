## 1. Decoding

- [x] 1.1 `UpdateTaskInput` keeps `"due_at": null` as a clear, with unit tests for absent, null and value

## 2. Surfaces

- [x] 2.1 HTTP test that sets, keeps and clears a due date, and a client round trip through the router
- [x] 2.2 `tix task edit --due ""` against `--server` clears it
- [x] 2.3 MCP forwards a null argument into the request body
- [x] 2.4 Doc comment, `just openapi`, docs/api.md
- [x] 2.5 skills/tix notes that `--due ""` clears a deadline
