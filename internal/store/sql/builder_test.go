package sql

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

var scope = core.TenantScope{TenantID: "t1"}

func TestNewRefusesWithoutTenant(t *testing.T) {
	for _, table := range ScopedTables() {
		if _, err := New(SQLite, core.TenantScope{}, table); err == nil {
			t.Errorf("New(%q) with no tenant should be refused", table)
		}
		if _, err := NewInsert(SQLite, core.TenantScope{}, table); err == nil {
			t.Errorf("NewInsert(%q) with no tenant should be refused", table)
		}
	}
}

func TestEveryScopedTableGetsATenantPredicate(t *testing.T) {
	for _, table := range ScopedTables() {
		b, err := New(SQLite, scope, table)
		if err != nil {
			t.Fatalf("New(%q) error = %v", table, err)
		}
		q, args := b.SelectQuery()
		if !strings.Contains(q, TenantColumn+" = ?") {
			t.Errorf("SELECT on %q has no tenant predicate: %s", table, q)
		}
		if len(args) == 0 || args[0] != "t1" {
			t.Errorf("SELECT on %q does not bind the tenant: %v", table, args)
		}
	}
}

func TestEveryScopedInsertSetsTenant(t *testing.T) {
	for _, table := range ScopedTables() {
		in, err := NewInsert(SQLite, scope, table)
		if err != nil {
			t.Fatalf("NewInsert(%q) error = %v", table, err)
		}
		in.Set("id", "x")
		q, args, err := in.Query()
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if !strings.Contains(q, TenantColumn) {
			t.Errorf("INSERT into %q does not set the tenant: %s", table, q)
		}
		if len(args) == 0 || args[0] != "t1" {
			t.Errorf("INSERT into %q does not bind the tenant: %v", table, args)
		}
	}
}

func TestUnscopedTablesAreAllowedWithoutTenant(t *testing.T) {
	for _, table := range []string{"tenants", "tenant_domains", "users", "schema_migrations"} {
		if IsScoped(table) {
			t.Errorf("%q should not be tenant scoped", table)
		}
		b, err := New(SQLite, core.TenantScope{}, table)
		if err != nil {
			t.Fatalf("New(%q) error = %v", table, err)
		}
		q, _ := b.SelectQuery()
		if strings.Contains(q, TenantColumn+" = ?") {
			t.Errorf("SELECT on %q should not carry a tenant predicate: %s", table, q)
		}
	}
}

func TestScopedTablesListIsComplete(t *testing.T) {
	for _, table := range ScopedTables() {
		if !IsScoped(table) {
			t.Errorf("%q is listed as scoped but IsScoped says otherwise", table)
		}
	}
}

func TestDeleteAndUpdateCarryTenant(t *testing.T) {
	b := MustNew(SQLite, scope, "tasks").Where("id = ?", "x")
	q, args := b.DeleteQuery()
	if !strings.Contains(q, TenantColumn) {
		t.Errorf("DELETE has no tenant predicate: %s", q)
	}
	if args[0] != "t1" {
		t.Errorf("DELETE does not bind the tenant first: %v", args)
	}

	u := MustNew(SQLite, scope, "tasks").Where("id = ?", "x").Set("title", "new")
	uq, uargs, err := u.UpdateQuery()
	if err != nil {
		t.Fatalf("UpdateQuery() error = %v", err)
	}
	if !strings.Contains(uq, TenantColumn) {
		t.Errorf("UPDATE has no tenant predicate: %s", uq)
	}
	if uargs[0] != "new" {
		t.Errorf("SET arguments must precede WHERE arguments, got %v", uargs)
	}
	if uargs[1] != "t1" {
		t.Errorf("tenant argument misplaced: %v", uargs)
	}
}

func TestUpdateWithoutAssignmentsIsRejected(t *testing.T) {
	if _, _, err := MustNew(SQLite, scope, "tasks").UpdateQuery(); err == nil {
		t.Error("an update with no assignments must be rejected")
	}
}

func TestInsertWithoutColumnsIsRejected(t *testing.T) {
	in := &Insert{dialect: SQLite, table: "tasks"}
	if _, _, err := in.Query(); err == nil {
		t.Error("an insert with no columns must be rejected")
	}
}

func TestPostgresRebind(t *testing.T) {
	b := MustNew(Postgres, scope, "tasks").Where("status = ?", "todo").Where("priority = ?", 1)
	q, args := b.SelectQuery()

	if strings.Contains(q, "?") {
		t.Errorf("postgres query still contains ?: %s", q)
	}
	for _, want := range []string{"$1", "$2", "$3"} {
		if !strings.Contains(q, want) {
			t.Errorf("postgres query missing %s: %s", want, q)
		}
	}
	if len(args) != 3 {
		t.Errorf("args = %v, want 3", args)
	}
}

func TestSQLiteKeepsQuestionMarks(t *testing.T) {
	q, _ := MustNew(SQLite, scope, "tasks").Where("status = ?", "todo").SelectQuery()
	if strings.Contains(q, "$1") {
		t.Errorf("sqlite query must not use numbered placeholders: %s", q)
	}
}

func TestWhereInEmptyIsNeverTrue(t *testing.T) {
	// An empty IN list must match nothing. Omitting the predicate instead would
	// silently widen the query to every row in the tenant.
	q, args := MustNew(SQLite, scope, "tasks").WhereIn("status", nil).SelectQuery()
	if !strings.Contains(q, "1 = 0") {
		t.Errorf("empty IN should be never-true: %s", q)
	}
	if len(args) != 1 {
		t.Errorf("empty IN should bind no extra args, got %v", args)
	}
}

func TestWhereIn(t *testing.T) {
	q, args := MustNew(SQLite, scope, "tasks").WhereIn("status", []string{"todo", "doing"}).SelectQuery()
	if !strings.Contains(q, "status IN (?, ?)") {
		t.Errorf("unexpected IN clause: %s", q)
	}
	if len(args) != 3 {
		t.Errorf("args = %v, want tenant plus two values", args)
	}
}

func TestKeysetPredicate(t *testing.T) {
	c := core.Cursor{SortValue: "2026-01-01", ID: "abc", Sort: "created_at", Direction: core.Ascending}

	q, args := MustNew(SQLite, scope, "tasks").
		Keyset("created_at", "id", c, core.Ascending).SelectQuery()
	if !strings.Contains(q, "(created_at, id) > (?, ?)") {
		t.Errorf("ascending keyset predicate wrong: %s", q)
	}
	if len(args) != 3 {
		t.Errorf("args = %v, want tenant plus cursor pair", args)
	}

	q, _ = MustNew(SQLite, scope, "tasks").
		Keyset("created_at", "id", c, core.Descending).SelectQuery()
	if !strings.Contains(q, "(created_at, id) < (?, ?)") {
		t.Errorf("descending keyset predicate wrong: %s", q)
	}
}

func TestKeyset2Predicate(t *testing.T) {
	c := core.Cursor{SortValue: "3", SortValue2: "2026-01-01", ID: "abc", Sort: "urgency", Direction: core.Ascending}

	q, args := MustNew(SQLite, scope, "tasks").
		Keyset2("priority", "due_at", "id", c, core.Ascending).SelectQuery()
	if !strings.Contains(q, "(priority, due_at, id) > (?, ?, ?)") {
		t.Errorf("ascending compound keyset predicate wrong: %s", q)
	}
	if len(args) != 4 {
		t.Errorf("args = %v, want tenant plus the three-part cursor", args)
	}

	q, _ = MustNew(SQLite, scope, "tasks").
		Keyset2("priority", "due_at", "id", c, core.Descending).SelectQuery()
	if !strings.Contains(q, "(priority, due_at, id) < (?, ?, ?)") {
		t.Errorf("descending compound keyset predicate wrong: %s", q)
	}
}

func TestZeroCursorAddsNoKeyset2Predicate(t *testing.T) {
	q, args := MustNew(SQLite, scope, "tasks").
		Keyset2("priority", "due_at", "id", core.Cursor{}, core.Ascending).SelectQuery()
	if strings.Contains(q, "priority, due_at, id") {
		t.Errorf("the zero cursor should add no predicate: %s", q)
	}
	if len(args) != 1 {
		t.Errorf("args = %v, want only the tenant", args)
	}
}

func TestZeroCursorAddsNoPredicate(t *testing.T) {
	q, args := MustNew(SQLite, scope, "tasks").
		Keyset("created_at", "id", core.Cursor{}, core.Ascending).SelectQuery()
	if strings.Contains(q, "created_at, id") {
		t.Errorf("the zero cursor should add no predicate: %s", q)
	}
	if len(args) != 1 {
		t.Errorf("args = %v, want only the tenant", args)
	}
}

func TestNoQueryUsesOffset(t *testing.T) {
	// OFFSET at depth scans every skipped row. Listings page by cursor instead,
	// and the builder offers no way to express OFFSET at all.
	b := MustNew(SQLite, scope, "tasks").Limit(50).OrderBy("created_at", core.Ascending)
	q, _ := b.SelectQuery()
	if strings.Contains(strings.ToUpper(q), "OFFSET") {
		t.Errorf("builder produced an OFFSET: %s", q)
	}
}

func TestOrderByDirection(t *testing.T) {
	q, _ := MustNew(SQLite, scope, "tasks").OrderBy("priority", core.Descending).SelectQuery()
	if !strings.Contains(q, "priority DESC") {
		t.Errorf("descending order missing: %s", q)
	}
	q, _ = MustNew(SQLite, scope, "tasks").OrderBy("priority", core.Ascending).SelectQuery()
	if !strings.Contains(q, "priority ASC") {
		t.Errorf("ascending order missing: %s", q)
	}
}

func TestSelectColumnsAndLimit(t *testing.T) {
	q, _ := MustNew(SQLite, scope, "tasks").Select("id", "title").Limit(10).SelectQuery()
	if !strings.Contains(q, "SELECT id, title FROM tasks") {
		t.Errorf("column list wrong: %s", q)
	}
	if !strings.Contains(q, "LIMIT 10") {
		t.Errorf("limit missing: %s", q)
	}
}

func TestCountQueryKeepsTenant(t *testing.T) {
	q, args := MustNew(SQLite, scope, "tasks").Where("status = ?", "todo").CountQuery()
	if !strings.Contains(q, "COUNT(*)") || !strings.Contains(q, TenantColumn) {
		t.Errorf("count query wrong: %s", q)
	}
	if args[0] != "t1" {
		t.Errorf("count query does not bind the tenant: %v", args)
	}
}

func TestJoinAndSetExpr(t *testing.T) {
	q, _ := MustNew(SQLite, scope, "tasks").
		Join("JOIN task_tags ON task_tags.task_id = tasks.id").SelectQuery()
	if !strings.Contains(q, "JOIN task_tags") {
		t.Errorf("join missing: %s", q)
	}

	uq, _, err := MustNew(SQLite, scope, "tasks").
		SetExpr("version", "version + 1").Where("id = ?", "x").UpdateQuery()
	if err != nil {
		t.Fatalf("UpdateQuery() error = %v", err)
	}
	if !strings.Contains(uq, "version = version + 1") {
		t.Errorf("expression assignment missing: %s", uq)
	}
}

func TestMustNewPanicsWithoutTenant(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustNew should panic without a tenant scope")
		}
	}()
	MustNew(SQLite, core.TenantScope{}, "tasks")
}
