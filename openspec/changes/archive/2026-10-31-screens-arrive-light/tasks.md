# Tasks

## 1. Store

- [x] 1.1 `0015_task_relation_indexes.sql`: task-led indexes on `task_tags` and `task_deps`
- [x] 1.2 `sqlite/task.go`, `postgres/task.go`: dependency fill ordered by task, then dependency
- [x] 1.3 `sqlite/relindex_test.go`: plans for the tag filter, the blocked filter and both page fills
      seek by task; schema rewinds in the upgrade tests drop the new indexes

## 2. Compression

- [x] 2.1 `internal/web/compress.go`: gzip wrapper for HTML, Accept-Encoding with weights
- [x] 2.2 `internal/web/assets.go`: precompressed assets with an ETag per encoding
- [x] 2.3 `internal/web/csrf.go`: masked form token per response; the check accepts masked or plain
- [x] 2.4 `internal/web/compress_test.go`, `assets_test.go`: wire bytes are gzip, refusal is honoured,
      redirects survive, the token differs per render and a tampered one is refused
