package sql

import "github.com/thereisnotime/tix/internal/core"

// dependencyPath walks task_deps transitively inside one tenant. A recursive CTE
// cannot be composed by Builder, so the tenant predicate is written in by hand
// and DependencyPathQuery refuses to render it without a scope.
const dependencyPath = `WITH RECURSIVE reachable(id) AS (
  SELECT depends_on FROM task_deps WHERE tenant_id = ? AND task_id = ?
  UNION
  SELECT d.depends_on FROM task_deps d
    JOIN reachable r ON d.task_id = r.id
   WHERE d.tenant_id = ?
)
SELECT EXISTS (SELECT 1 FROM reachable WHERE id = ?)`

// DependencyPathQuery renders the reachability probe that rejects dependency cycles.
func DependencyPathQuery(d Dialect, scope core.TenantScope, from, to string) (string, []any, error) {
	if !scope.Valid() {
		return "", nil, core.Invalid("refusing to walk dependencies without a tenant scope")
	}
	b := &Builder{dialect: d}
	return b.rebind(dependencyPath), []any{scope.TenantID, from, scope.TenantID, to}, nil
}
