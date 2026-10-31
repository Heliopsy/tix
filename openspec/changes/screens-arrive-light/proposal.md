# Screens arrive light

## Why

Measured with a browser on a seeded scratch store (6 projects, 215 tasks, 25 rows a page), the task
list was 185KB of HTML on every load, every boosted swap and every live refresh, and the stylesheet
and htmx another 250KB on a cold load, none of it compressed. A `tag:` filter took 40ms where the
unfiltered list took 12ms: the query planner reached `task_tags` and `task_deps` through the
`(tenant_id, tag_id)` and `(tenant_id, depends_on)` indexes, walking every tag or dependency row in
the tenant once per task considered, so the cost grew with the tenant's tags times its tasks.

## What Changes

- Two indexes lead with the tenant and then the task, `task_tags(tenant_id, task_id, tag_id)` and
  `task_deps(tenant_id, task_id, depends_on)`. The page's dependency query orders by task first, so
  the new index also gives it its order.
- Screens and fragments are gzip-compressed for a browser that accepts it; assets are compressed
  once at startup and carry a validator of their own per encoding.
- Because a compressed page that reflects input must not carry a constant secret, the CSRF value a
  form carries is masked with a fresh one-time pad on every response. The cookie is unchanged, and
  the check accepts the masked value or the cookie's own value.

## Impact

- `internal/store/migrations/0015_task_relation_indexes.sql`, `internal/store/{sqlite,postgres}/task.go`.
- `internal/web`: `compress.go`, `assets.go`, `csrf.go`, `web.go`.
- No route, service method or capability changes.
