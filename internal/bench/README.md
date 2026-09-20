# internal/bench

Load generator and latency budgets for the operations that decide whether tix
holds up at the design target of a million tasks.

The package exists to catch two regressions before a release: a listing that
quietly grows an `OFFSET`, and a query that loses its index. Both are invisible
in a unit test and obvious here.

## Running it

```sh
go test ./internal/bench/            # small profile, sqlite, ~10s
just bench                           # the Go benchmarks, small profile
```

The profile is chosen by `TIX_BENCH_SIZE`:

| value            | tenants | projects | tasks     |
| ---------------- | ------- | -------- | --------- |
| unset / `small`  | 2       | 6        | 3,000     |
| `medium`         | 3       | 20       | 100,000   |
| `full` or `1m`   | 5       | 50       | 1,000,000 |
| any integer      | 5       | 50       | that many |

```sh
TIX_BENCH_SIZE=full just bench-full                      # the design target
TIX_BENCH_SIZE=medium go test ./internal/bench/ -short   # skips the superlinear cases
```

Postgres runs whenever `TIX_TEST_POSTGRES_DSN` is set and skips cleanly when it
is not, the same gate `internal/store/postgres` uses. The fixture gets a
throwaway database of its own (`tix_bench_<pid>`), dropped when the process
exits. `TIX_BENCH_ENGINE=sqlite|postgres` restricts the run to one engine.

Postgres is `ANALYZE`d after seeding. Without statistics the planner reads a
freshly loaded table as empty; the unblocked listing measured 570 ms before
`ANALYZE` and 7.7 ms after, on the same rows.

## Measured numbers

Reference machine: 12th Gen Intel Core i7-12700H, 20 threads, Go 1.27.1,
Linux 7.0; Postgres 18-alpine in podman over loopback. p95 of 20 to 60
samples, page size 50.

### small profile, 1,500 tasks in the measured tenant

| operation              | sqlite p95 | postgres p95 | budget (sqlite) | budget (postgres) |
| ---------------------- | ---------- | ------------ | --------------- | ----------------- |
| `list_first_page`      | 2.0 ms     | 7.2 ms       | 6 ms            | 25 ms             |
| `list_deep_page`       | 3.2 ms     | 7.0 ms       | 9 ms            | 25 ms             |
| `board_grouped`        | 14.0 ms    | 22.2 ms      | 45 ms           | 70 ms             |
| `search_body`          | 1.6 ms     | 6.2 ms       | 6 ms            | 25 ms             |
| `list_unblocked`       | 98 ms      | 7.7 ms       | 300 ms          | 30 ms             |
| `claim_next_serial`    | 477 ms     | 12.8 ms      | 1.5 s           | 40 ms             |
| `claim_next_contended` | 2.97 s     | 49.6 ms      | 9 s             | 150 ms            |

Budgets are about three times the worst p95 seen across repeat runs, which
absorbs a loaded CI worker without absorbing a lost index. `claim_next_contended`
takes only eight samples, so its p95 is effectively its maximum and is noisy;
its budget is set accordingly.

### medium profile, 33,334 tasks in the measured tenant

| operation              | sqlite p95 | postgres p95 |
| ---------------------- | ---------- | ------------ |
| `list_first_page`      | 11.5 ms    | 3.4 ms       |
| `list_deep_page`       | 16.8 ms    | 16.5 ms      |
| `board_grouped`        | 196 ms     | 28.8 ms      |
| `search_body`          | 57 ms      | 26.2 ms      |
| `list_unblocked`       | not run    | 7.6 ms       |
| `claim_next_serial`    | not run    | 62.0 ms      |
| `claim_next_contended` | not run    | 112 ms       |

The superlinear SQLite cases are left out of that run; see finding 3.

`medium/sqlite` carries a budget for the four cases above. `medium/postgres`
deliberately carries none, because `TestDeepPaginationDoesNotDegrade` fails
there today: see finding 7.

### page cost against depth, medium profile, postgres, p50

| depth | latency |
| ----- | ------- |
| 0%    | 1.6 ms  |
| 10%   | 25.8 ms |
| 25%   | 23.9 ms |
| 50%   | 18.4 ms |
| 75%   | 12.7 ms |
| 90%   | 9.2 ms  |

`TestPaginationDepthCurve` reports this shape on every run.

### how each operation scales, sqlite, single tenant, p50

| tasks  | list    | board   | unblocked | claim next |
| ------ | ------- | ------- | --------- | ---------- |
| 1,000  | 0.80 ms | 7.1 ms  | 37 ms     | 193 ms     |
| 2,000  | 1.10 ms | 8.8 ms  | 82 ms     | 777 ms     |
| 4,000  | 1.37 ms | 13.1 ms | 149 ms    | 3.36 s     |
| 8,000  | 1.90 ms | 27.8 ms | 497 ms    | 24.7 s     |

## Findings

These are measurements, not opinions. Nothing here was worked around by
loosening a budget.

### 1. Keyset pagination is intact

`EXPLAIN QUERY PLAN` on the listing with a cursor:

```text
SEARCH tasks USING INDEX idx_tasks_created (tenant_id=? AND (created_at,id)>(?,?))
```

The cursor predicate is resolved inside the index seek. No `OFFSET`, no temp
B-tree, and the deep page costs what the first page costs at every size
measured. `TestDeepPaginationDoesNotDegrade` asserts that ratio, which makes it
scale-free: it fails on a reintroduced `OFFSET` whatever the fixture size.

### 2. `fillTaskRelations` scans the whole tenant on every listing

Every task listing loads tags and dependencies for the page it just read. On
SQLite both lookups ignore the `task_id IN (...)` restriction:

```text
SEARCH task_tags USING INDEX idx_task_tags_tag (tenant_id=?)
USE TEMP B-TREE FOR ORDER BY
SEARCH task_deps USING INDEX idx_deps_depends_on (tenant_id=?)
```

`task_tags` is keyed `(task_id, tag_id)` and `task_deps` `(task_id, depends_on)`,
but the tenant-scoped builder emits `tenant_id = ?` first and the only
tenant-leading indexes are `(tenant_id, tag_id)` and `(tenant_id, depends_on)`.
The planner takes the tenant-leading index and scans every row the tenant owns.

This is why a keyset page, whose own seek is O(1), still grew from 2.0 ms at
1,500 tasks to 11.5 ms at 33,334. At a million tasks the same listing would scan
roughly 200,000 tag rows and 50,000 dependency rows per page.

Indexes on `task_tags(tenant_id, task_id)` and `task_deps(tenant_id, task_id)`
would make both lookups seeks. Neither exists.

### 3. `ClaimNext` is quadratic on SQLite and cannot reach the design target

```text
SEARCH tasks USING INDEX idx_tasks_parent (tenant_id=?)
CORRELATED SCALAR SUBQUERY 1
  SEARCH dep USING INDEX idx_tasks_priority (tenant_id=?)
  SEARCH d USING INDEX sqlite_autoindex_task_deps_1 (task_id=? AND depends_on=?)
USE TEMP B-TREE FOR ORDER BY
```

Two compounding problems:

- The `ORDER BY priority, created_at, id` of the `LIMIT 1` subquery matches no
  index, so the temp B-tree forces every candidate in the tenant to be
  evaluated before the limit applies. The ordered `LIMIT 1` cannot short-circuit.
- The dependency anti-join is driven from `tasks dep` rather than from
  `task_deps d`, so each candidate costs a scan of the tenant's tasks.

Together that is O(n²): 193 ms at 1,000 tasks, 24.7 s at 8,000. Extrapolated,
a single `ClaimNext` at the 1M target would take hours. The full profile on
SQLite is therefore not runnable for this operation; `ClaimTimeout` fails the
case at 90 s rather than hanging the suite.

Postgres plans the same statement properly (12.8 ms at 1,500 tasks, 49.6 ms
under eight-way contention) because it turns the correlated `NOT EXISTS` into a
hash anti-join and can use the priority index for the ordering.

`list_unblocked` shares the dependency predicate and is superlinear for the same
reason, but escapes the worst of it: its `ORDER BY priority, id` is served by
`idx_tasks_priority`, so it stops after the page fills.

### 4. `ClaimNext` reads back the claimed task by an unindexed column

After the compare-and-swap, both engines find the row they just claimed with
`WHERE lease_token = ?`. There is no index on `lease_token`:

```text
SEARCH tasks USING INDEX idx_tasks_priority (tenant_id=?)
```

That is a full tenant scan per claim. It is dwarfed by finding 3 today, but it
is the next wall once that is fixed.

### 5. A board column does not use its own index

```text
SEARCH tasks USING INDEX idx_tasks_priority (tenant_id=?)
```

`idx_tasks_project_status` is `(tenant_id, project_id, status, deleted_at)` and
cannot serve `ORDER BY priority`, so the planner scans the tenant in priority
order and discards what does not match the project and status. With 6 projects
and 5 states that reads roughly thirty rows per row kept, which is the 196 ms
board at 33,334 tasks. Extending the index to
`(tenant_id, project_id, status, priority, id)` would make the column a seek.

### 6. Search is a substring scan on SQLite only

SQLite matches `title LIKE '%q%' OR body LIKE '%q%'`, which no index can serve:
57 ms at 33,334 tasks, growing linearly. Postgres has the generated `search_tsv`
column with a GIN index and stays flat. This is a known dialect difference
rather than a defect, but the SQLite path should not be offered as a search
feature at scale.

### 7. On Postgres, a filtered listing with a cursor abandons its index

This is the one finding that touches the invariant the package was written to
protect, and it is a live failure, not a warning.

`internal/store/postgres` renders the priority filter as
`CAST(tasks.priority AS TEXT) IN (...)`. The cast is invisible to the planner's
statistics, so the row estimate collapses, and once a cursor is added the
planner abandons `idx_tasks_created` entirely:

```text
Limit  (actual time=23.681..23.688 rows=51)
  ->  Sort  (actual time=23.679..23.683 rows=51)
        Sort Key: tasks.created_at, tasks.id
        Sort Method: top-N heapsort
        ->  Nested Loop
              Rows Removed by Join Filter: 54003
              ->  Bitmap Heap Scan on tasks  (estimated 268, actual 18000 rows)
                    Filter: ((ROW(created_at, id) > ROW($2, $3)) AND ((priority)::text = ANY (...)))
                    ->  Bitmap Index Scan on idx_tasks_project_status
```

The keyset predicate has been demoted from an index condition to a row filter.
The query reads every task after the cursor, joins it against projects, and
top-N sorts the lot to return fifty rows. That is why cost falls as the cursor
moves deeper: 25.8 ms at 10% and 9.2 ms at 90%, against 1.6 ms with no cursor
at all. It is not an `OFFSET`, but it is not O(1) either, and at a million tasks
the first pages of a filtered listing would read most of the table.

The same statement with `tasks.priority IN (1,2,3)` and no cast plans correctly:

```text
Index Scan using idx_tasks_created
  Index Cond: ((tenant_id = $1) AND (ROW(created_at, id) > ROW($2, $3)))
Execution Time: 0.388 ms
```

Sixty times faster, with the cursor back inside the index seek. Comparing
`priority` as the integer it is declared as, rather than casting it to text,
looks like the whole fix. SQLite is unaffected: it compares the same text
values under integer column affinity and keeps `idx_tasks_created`.

The nested loop's `Rows Removed by Join Filter: 54003` is a second-order effect
of the same bad estimate; the join to `projects` exists only to read the project
key for a task reference.

## What the tests assert

- `TestPercentileBudgets` measures each operation and fails when p95 exceeds the
  budget for that `<profile>/<engine>` pair. A profile with no committed budget
  is reported and not asserted.
- `TestDeepPaginationDoesNotDegrade` fails when a deep page costs more than four
  times the first page.
- `TestDeepPageReturnsAFullPage` and `TestSearchFindsTheSeededToken` keep the
  timings honest by proving the measured queries return rows.
- `TestPaginationDepthCurve` reports page cost at six depths and asserts
  nothing. It is the diagnostic that tells a rising curve (an `OFFSET`) apart
  from a falling one (a plan that reads everything past the cursor).

`TIX_BENCH_SIZE=medium TIX_BENCH_ENGINE=postgres` fails today on
`TestDeepPaginationDoesNotDegrade`. That failure is finding 7 and is real; it
was not silenced with a budget.

`-short` leaves out the operations whose cost is superlinear in the fixture, so
`just test-short` stays quick.
