// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"strings"
	"testing"
)

// TestTaskRelationsAreSeekedByTask asks SQLite how it would answer the
// queries a page of tasks makes for its tags and dependencies, and refuses a
// plan that reaches task_tags or task_deps by the tenant alone. That plan
// reads every tag or dependency row in the tenant once per task asked about,
// which is what made a tag: filter twenty times slower than the listing it
// narrowed.
func TestTaskRelationsAreSeekedByTask(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	tagged, tagArgs := taggedWithAny([]string{"ops"})

	cases := []struct {
		name  string
		table string
		query string
		args  []any
	}{
		{
			name:  "tag filter",
			table: "tl",
			query: "SELECT tasks.id FROM tasks WHERE tasks.tenant_id = ? AND EXISTS " + tagged,
			args:  append([]any{f.tenant.ID}, tagArgs...),
		},
		{
			name:  "blocked filter",
			table: "d",
			query: "SELECT tasks.id FROM tasks WHERE tasks.tenant_id = ? AND EXISTS " + blockingSubquery,
			args:  []any{f.tenant.ID},
		},
		{
			name:  "tags of a page",
			table: "task_tags",
			query: "SELECT task_tags.task_id, tags.name FROM task_tags JOIN tags ON tags.id = task_tags.tag_id" +
				" AND tags.tenant_id = task_tags.tenant_id WHERE task_tags.tenant_id = ? AND task_tags.task_id IN (?, ?)" +
				" ORDER BY tags.name ASC",
			args: []any{f.tenant.ID, "a", "b"},
		},
		{
			name:  "dependencies of a page",
			table: "task_deps",
			query: "SELECT task_deps.task_id, task_deps.depends_on FROM task_deps" +
				" WHERE task_deps.tenant_id = ? AND task_deps.task_id IN (?, ?)" +
				" ORDER BY task_deps.task_id ASC, task_deps.depends_on ASC",
			args: []any{f.tenant.ID, "a", "b"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := planFor(t, s, tc.table, tc.query, tc.args...)
			if !strings.Contains(line, "task_id=?") {
				t.Fatalf("%s is reached by %q, want a seek on the task", tc.table, line)
			}
		})
	}
}

// planFor returns the query plan's line for one table alias.
func planFor(t *testing.T, s *Store, table, query string, args ...any) string {
	t.Helper()
	rows, err := s.reader.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explaining %q: %v", query, err)
	}
	defer func() { _ = rows.Close() }()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatalf("reading the plan: %v", err)
		}
		plan = append(plan, detail)
		if strings.HasPrefix(detail, "SEARCH "+table+" ") || strings.HasPrefix(detail, "SCAN "+table+" ") ||
			detail == "SCAN "+table {
			return detail
		}
	}
	t.Fatalf("no step reads %s in the plan:\n%s", table, strings.Join(plan, "\n"))
	return ""
}
